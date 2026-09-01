# Contributing to VirtFoundry Operator

Thank you for helping grow VirtFoundry. This repository lives under the [virtfoundry](https://github.com/virtfoundry) organization.

## Before you start

- Read the design: [CRD operator design](https://github.com/virtfoundry/core/blob/main/docs/superpowers/specs/2026-09-01-crd-operator-design.md) (path may move with docs PRs)
- Search [existing issues](https://github.com/virtfoundry/operator/issues) before opening a duplicate

## Language

- **Commits**: English only, [Conventional Commits](https://www.conventionalcommits.org/)
- **Documentation** and PR descriptions: English

### Commit examples

```
feat(api): add Tenant CRD with vf-tenant shortName
feat(controller): reconcile Tenant to namespace
fix(controller): requeue while Namespace terminates
docs(readme): document kind smoke path
```

## Development setup

```bash
make generate manifests
make test
make build
```

## Branch workflow

**Do not commit directly to `main`.** Use:

1. Branch from `main`: `feat/<name>`, `fix/<name>`, `docs/<name>`, or `chore/<name>`
2. Open a PR against `main`
3. Wait for CI (`make test`) before merge

## Code of conduct

See [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
