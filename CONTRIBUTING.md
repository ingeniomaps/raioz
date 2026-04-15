# Contributing to Raioz

## Prerequisites

- Go 1.24+
- Docker (for integration tests)
- `golangci-lint` — [install](https://golangci-lint.run/welcome/install/)
- `goimports` — `go install golang.org/x/tools/cmd/goimports@latest`

Optional:
- `govulncheck` — `go install golang.org/x/vuln/cmd/govulncheck@latest`
- `mkcert` — for local HTTPS testing

## Development workflow

Raioz uses git-flow: feature work lands on `develop`, `main` always
reflects the latest released version. Open pull requests against
`develop`; releases are cut by merging `develop` → `main` and tagging
on `main`.

```bash
# 1. Create a branch off develop
git fetch origin
git checkout -b feat/my-change origin/develop

# 2. Make changes and verify
make check       # format + lint + i18n + tests

# 3. Before pushing
make ci           # full CI: check + build

# 4. Run integration tests (requires Docker)
make integration-test

# 5. Open a pull request targeting `develop` (NOT main)
```

## Code standards

| Rule | Limit | Check |
|------|-------|-------|
| Max lines per file | 400 (tests + `internal/config/schema.go` exempt) | `make check-lines` |
| Max line length | 120 chars | `make check-length` |
| Test coverage | >= 70% (target: 80% post-v0.2.0) | `make check-coverage` |
| i18n catalog sync | all keys present | `make check-i18n` |

Lint is currently scoped to a reduced baseline for v0.1.0. See
[ROADMAP.md](ROADMAP.md) for the plan to re-tighten it.

## Architecture

Clean Architecture with inward dependency flow:

```
cmd/raioz/     →  internal/cli/  →  internal/app/  →  internal/domain/
                                                    →  internal/infra/
```

- **cli/**: Cobra commands. Thin — create deps, call use case, return error.
- **app/**: Use cases with `Options` + `Execute()`. Only uses domain interfaces.
- **domain/interfaces/**: Port interfaces (27 total).
- **infra/**: Adapters implementing domain interfaces.

See `CLAUDE.md` for the full architecture reference.

## Key patterns

- **i18n**: All user-facing strings through `i18n.T()`. Add keys to both `en.json` and `es.json`.
- **Errors**: `errors.New(code, i18n.T("error.xxx")).WithSuggestion(...)`.
- **DI**: Dependencies injected via struct, never created inline in app/.
- **Tests**: Table-driven with `t.Run`. Mocks in `internal/mocks/`.
- **Commits**: Conventional Commits, English, imperative, max 50 char subject. Full convention and examples in `.claude/skills/commit/SKILL.md` (also usable via the `/commit` Claude Code skill).

## Make targets

```bash
make help             # list all targets
make build            # build binary
make test             # run all tests
make lint             # golangci-lint
make format           # gofmt + goimports
make check            # all checks
make ci               # check + build
make security         # gosec + govulncheck
make integration-test # e2e tests (requires Docker)
make clean            # remove build artifacts
```

## Adding a new runtime

See `.claude/skills/runtime/SKILL.md` for the full checklist:

1. Add constant in `internal/detect/result.go`
2. Add detection in `internal/detect/detect.go`
3. Register in `internal/orchestrate/orchestrate.go`
4. Add tests in `internal/detect/detect_all_runtimes_test.go`
5. Update README.md runtimes table

## Adding a new CLI command

See `.claude/skills/architecture/SKILL.md`:

1. Create `internal/cli/{command}.go`
2. Create use case in `internal/app/`
3. Add i18n keys to both catalogs
4. Register in `internal/cli/root.go`

## License

MIT
