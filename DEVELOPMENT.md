# Development Guide - Raioz

How to set up your environment and contribute to the project.

## Prerequisites

- Go 1.22+
- `golangci-lint`
- `goimports`
- `make`
- Docker (for integration tests)

## Tool Installation

### golangci-lint

```bash
# Linux/macOS
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest

# macOS via Homebrew
brew install golangci-lint
```

### goimports

```bash
go install golang.org/x/tools/cmd/goimports@latest
```

### Security tools (optional)

```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
# gosec is already included in golangci-lint
```

## Development Workflow

### 1. Create a branch

```bash
git checkout -b feature/my-change
```

### 2. Make changes and verify

```bash
make check    # format + lint + check-i18n + tests
```

### 3. Before pushing

```bash
make ci       # full CI: check + build
```

See `CLAUDE.md` for the complete list of `make` targets.

## Code Standards

| Rule | Limit | Check |
|------|-------|-------|
| Max lines per file (non-test) | 400 | `make check-lines` |
| Max line length | 120 chars | `make check-length` |
| Test coverage | >= 80% | `make check-coverage` |
| i18n catalog sync | all keys present | `make check-i18n` |

### Splitting long lines

```go
// Multi-param functions
func longFunction(
    param1, param2 string,
    param3, param4 string,
) error

// Long strings
msg := "This is a very long " +
    "string that needs to be " +
    "split across lines"
```

### Linter issues

1. Read the error message
2. Check the linter documentation
3. Fix according to the recommendation
4. If necessary, add an exception in `.golangci.yml`

## Testing

- One test file per source file: `{file}_test.go`
- Table-driven tests with `t.Run`
- Mocks live in `internal/mocks/` (manual implementations for 9 domain interfaces)

```go
func TestLoadFiles(t *testing.T) {
    tests := []struct {
        name string
        // ...
    }{
        // ...
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // test
        })
    }
}
```

## Tools Status

Summary of tooling that is in place vs planned improvements.

### Implemented

| Category | Tool | Notes |
|----------|------|-------|
| Linting | golangci-lint, gofmt, goimports | Strict rules in `.golangci.yml` |
| Testing | go test, manual mocks | `internal/mocks/` covers all domain interfaces |
| Build/CI | Makefile, validation scripts | `scripts/check-code-standards.sh` |
| Git hooks | pre-commit | `scripts/setup-hooks.sh` |
| Security | gosec (via lint), govulncheck | `make security` |
| i18n | embedded JSON catalogs | 503 keys, en + es |

### Not Yet Implemented

| Tool | Priority | Purpose |
|------|----------|---------|
| GitHub Actions CI | High | Automated lint, test, build, coverage on push/PR |
| Codecov / Coveralls | High | Coverage reporting and badges |
| Dependabot | High | Automated dependency updates |
| goreleaser | Medium | Cross-platform release binaries |
| Go fuzzing | Medium | Fuzz JSON parsing, env files, path validation |
| Benchmarks | Medium | Performance regression detection |
| godoc | Medium | Package documentation generation |
| Mockery | Low | Auto-generated mocks (manual mocks work fine for now) |
| Dev Containers | Low | Reproducible dev environment |
| git-chglog | Low | Automated CHANGELOG from conventional commits |

### Recommended implementation order

1. GitHub Actions CI (`.github/workflows/ci.yml`)
2. Code coverage reporting (Codecov integration)
3. Dependabot (`.github/dependabot.yml`)
4. goreleaser for releases
5. Fuzzing and benchmarks

## Contributing

1. Read `CLAUDE.md` for architecture and conventions
2. Create a branch and make changes
3. Follow code standards (400 lines, 120 chars, i18n)
4. Write tests (table-driven, >= 80% coverage)
5. Run `make check`
6. Create a PR with a clear description

## Useful Links

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [golangci-lint docs](https://golangci-lint.run/)
