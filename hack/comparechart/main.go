package main

// Compares ClusterRole rule triples (apiGroup/resource/verb) between
// kubebuilder config/rbac/role.yaml and a rendered Helm ClusterRole.
// Ignores metadata/name and rule grouping / YAML formatting.
// Helm multi-doc: skips *-kubevirt-mutate (tenant RoleBinding slice, #28/#49).

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type policyRule struct {
	APIGroups []string `yaml:"apiGroups"`
	Resources []string `yaml:"resources"`
	Verbs     []string `yaml:"verbs"`
}

type objectMeta struct {
	Name string `yaml:"name"`
}

type clusterRole struct {
	Kind     string       `yaml:"kind"`
	Metadata objectMeta   `yaml:"metadata"`
	Rules    []policyRule `yaml:"rules"`
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: %s <kubebuilder-role.yaml> <helm-rendered.yaml>\n", os.Args[0])
		os.Exit(2)
	}
	want, err := loadTriples(os.Args[1], true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubebuilder role: %v\n", err)
		os.Exit(1)
	}
	got, err := loadTriples(os.Args[2], false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "helm chart role: %v\n", err)
		os.Exit(1)
	}
	onlyWant := diff(want, got)
	onlyGot := diff(got, want)
	if len(onlyWant) == 0 && len(onlyGot) == 0 {
		fmt.Println("OK: Helm ClusterRole rules match config/rbac/role.yaml (apiGroup/resource/verb set)")
		return
	}
	fmt.Fprintln(os.Stderr, "FAIL: Helm ClusterRole drifted from kubebuilder config/rbac/role.yaml")
	if len(onlyWant) > 0 {
		fmt.Fprintln(os.Stderr, "  missing from chart (present in markers/role.yaml):")
		for _, t := range onlyWant {
			fmt.Fprintf(os.Stderr, "    - %s\n", t)
		}
	}
	if len(onlyGot) > 0 {
		fmt.Fprintln(os.Stderr, "  extra in chart (not in markers/role.yaml):")
		for _, t := range onlyGot {
			fmt.Fprintf(os.Stderr, "    - %s\n", t)
		}
	}
	fmt.Fprintln(os.Stderr, "Update charts/virtfoundry-operator/templates/rbac.yaml in virtfoundry/helm-charts")
	fmt.Fprintln(os.Stderr, "after `make manifests`, or adjust +kubebuilder:rbac markers.")
	os.Exit(1)
}

func loadTriples(path string, singleDoc bool) (map[string]struct{}, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	out := map[string]struct{}{}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	for {
		var doc clusterRole
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if doc.Kind != "" && doc.Kind != "ClusterRole" {
			if singleDoc {
				return nil, fmt.Errorf("%s: expected ClusterRole, got %s", path, doc.Kind)
			}
			continue
		}
		// Per-tenant mutate slice is not in config/rbac/role.yaml.
		if strings.Contains(doc.Metadata.Name, "kubevirt-mutate") {
			continue
		}
		if len(doc.Rules) == 0 {
			continue
		}
		for _, rule := range doc.Rules {
			groups := rule.APIGroups
			if len(groups) == 0 {
				groups = []string{""}
			}
			for _, g := range groups {
				for _, res := range rule.Resources {
					for _, v := range rule.Verbs {
						out[g+"/"+res+"/"+v] = struct{}{}
					}
				}
			}
		}
		if singleDoc {
			break
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s: no ClusterRole rules found", path)
	}
	return out, nil
}

func diff(a, b map[string]struct{}) []string {
	var out []string
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	slices.Sort(out)
	return out
}
