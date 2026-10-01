# AGENTS — virtfoundry/operator

Controllers Kubebuilder / CRDs `virtfoundry.io` (Tenant, Instance, VPC, …).

## Cursor Team Kit

Usar o plugin **cursor-team-kit**:

| Situação | Skill |
|----------|--------|
| Branch + PR | `new-branch-and-pr` / `review-and-ship` |
| CI | `fix-ci` + `loop-on-ci` |
| PR legível | `make-pr-easy-to-review` |
| Typecheck | `check-compiler-errors` |
| Limpar noise de AI | `deslop` |

Rules: `typescript-exhaustive-switch`, `no-inline-imports` (UI/TS); em Go seguir `make lint` / `golangci`.

## VirtFoundry

- SemVer produto **0.8.x** (alinhar chart/operator no release).
- Testes no **homelab Linux** (cluster real ou Kind/Linux com KubeVirt). Gate de produto = homelab; **não** Kind no macOS (KubeVirt não funciona).
- Preview sem commit só com pedido explícito.
- Não taguear / mergear release sem OK do maintainer.

## Scaffold (Kubebuilder)

Não editar gerados: `config/crd/bases/*`, `config/rbac/role.yaml`, `**/zz_generated.*`, `PROJECT`.  
Após mudar `*_types.go` / markers: `make manifests && make generate`.

## Docs locais

- [README.md](README.md)
- [CONTRIBUTING.md](CONTRIBUTING.md)
- [docs/INSTANCE-CR-SMOKE.md](docs/INSTANCE-CR-SMOKE.md) — smoke do Instance CR
