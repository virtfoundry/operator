package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadPending(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rbac-pending.txt")
	if got, err := loadPending(path); err != nil || len(got) != 0 {
		t.Fatalf("missing file: got %v, %v", got, err)
	}
	if err := os.WriteFile(path, []byte("# why\n/secrets/get # trailing\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := loadPending(path)
	if _, ok := got["/secrets/get"]; err != nil || len(got) != 1 || !ok {
		t.Fatalf("got %v, %v", got, err)
	}
}
