# Skill: test

## Description

Generate idiomatic Go tests following Raioz project
conventions.

## When to use

Run `/test` when:
- Writing tests for new or existing functions
- Need to understand testing patterns in this project
- Want to generate a test skeleton for a package

## Project test conventions

### File naming

- Tests live next to source: `foo.go` -> `foo_test.go`
- Integration tests: `integration_test.go` with build tag
- Edge case tests: `*_edge_cases_test.go` or `*_errors_test.go`

### Test structure: table-driven

Always use table-driven tests with `t.Run`:

```go
func TestFunctionName(t *testing.T) {
    tests := []struct {
        name    string
        input   InputType
        want    OutputType
        wantErr bool
    }{
        {
            name:  "descriptive case name",
            input: InputType{Field: "value"},
            want:  OutputType{Result: "expected"},
        },
        {
            name:    "error case description",
            input:   InputType{Field: ""},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := FunctionName(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("FunctionName() error = %v, wantErr %v",
                    err, tt.wantErr)
                return
            }
            if !tt.wantErr && got != tt.want {
                t.Errorf("FunctionName() = %v, want %v",
                    got, tt.want)
            }
        })
    }
}
```

### Temporary directories

Use `t.TempDir()` for file system tests — auto-cleaned:

```go
func TestSomething(t *testing.T) {
    tmpDir := t.TempDir()
    configPath := filepath.Join(tmpDir, "raioz.yaml")
    os.WriteFile(configPath, []byte(`project: test`), 0644)
    // test logic
}
```

### Mocks

The project uses **hand-crafted mocks** in
`internal/mocks/`. There are 11 mocks for domain
interfaces:

| Mock | Interface |
|------|-----------|
| `MockDockerRunner` | `DockerRunner` |
| `MockProxyManager` | `ProxyManager` |
| `MockWorkspaceManager` | `WorkspaceManager` |
| `MockStateManager` | `StateManager` |
| `MockConfigLoader` | `ConfigLoader` |
| `MockValidator` | `Validator` |
| `MockGitRepository` | `GitRepository` |
| `MockLockManager` | `LockManager` |
| `MockLock` | `Lock` (acquired lock handle) |
| `MockHostRunner` | `HostRunner` |
| `MockEnvManager` | `EnvManager` |

Each mock stores call arguments and returns configurable
values. Example usage:

```go
func TestUseCase(t *testing.T) {
    mock := &mocks.MockDockerRunner{
        ComposeUpFn: func(ctx context.Context, opts ComposeUpOptions) error {
            return nil
        },
    }
    deps := &app.Dependencies{Docker: mock}
    uc := app.NewSomeUseCase(deps)
    err := uc.Execute(context.Background(), opts)
    // assertions
}
```

### Context parameter

All use case `Execute()` methods take `context.Context`
as first parameter. Tests must pass one:

```go
ctx := context.Background()
err := useCase.Execute(ctx, options)
```

### Environment variables

Isolate env var tests:

```go
os.Setenv("RAIOZ_LANG", "es")
defer os.Unsetenv("RAIOZ_LANG")
```

### Integration tests

Require Docker and use build tags:

```go
//go:build integration

package cmd_test
```

Run with: `go test -v -tags=integration ./cmd/...`

### Error assertions

For structured errors, check the error code:

```go
var raiozErr *errors.RaiozError
if !stderrors.As(err, &raiozErr) {
    t.Fatal("expected RaiozError")
}
if raiozErr.Code != errors.ErrConfigNotFound {
    t.Errorf("got code %s, want %s",
        raiozErr.Code, errors.ErrConfigNotFound)
}
```

## Test categories to cover

For each function, consider:

1. **Happy path** — normal input, expected output
2. **Edge cases** — empty input, nil, zero values
3. **Error cases** — invalid input, missing files, permission errors
4. **Boundary cases** — max lengths, special characters
5. **Security cases** — path traversal, injection attempts

## Commands

```bash
# Run all tests
make test

# Run specific test
go test -v -run TestFunctionName ./internal/package/...

# Run with coverage
make test-coverage

# Check coverage threshold (70% for v0.1.0; target 80% post-release)
make check-coverage

# Quick tests (pre-commit)
go test ./... -short
```

## Rules

- Always use table-driven tests with `t.Run`
- Always use `t.TempDir()` for file system operations
- Always pass `context.Background()` to Execute methods
- Use project mocks from `internal/mocks/`, do not create ad-hoc mocks
- Test names: descriptive phrases, not function signatures
- No `t.Log` noise — only log on failure via `t.Errorf`
- Tests must be deterministic and isolated
- Coverage floor: 70% total (enforced by `make check-coverage`). Target 80% — see ROADMAP.md. Don't regress packages you touch.
