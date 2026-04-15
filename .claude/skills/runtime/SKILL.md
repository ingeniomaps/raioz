# Skill: runtime

## Description

Add a new runtime to Raioz's auto-detection system.
Follow this checklist to ensure full integration.

## When to use

Run `/runtime` when:
- Adding support for a new programming language/tool
- Modifying detection logic for an existing runtime
- Need to understand how runtimes are wired end-to-end

## How detection works

```
service directory
  → detect.Detect(path)        # scans for trigger files
  → DetectResult{Runtime, StartCommand, Port, ...}
  → orchestrate.Dispatcher     # routes to correct runner
  → HostRunner / ComposeRunner / DockerfileRunner
```

Most new runtimes are **host runtimes** (run directly on
the machine, not in Docker).

## Steps to add a new runtime

### Step 1: Add the Runtime constant

File: `internal/detect/result.go`

```go
const (
    // ... existing runtimes ...
    RuntimeMyLang Runtime = "mylang"
)
```

Use lowercase, short name. This value appears in CLI
output (`raioz status`, `raioz init`).

### Step 2: Add detection logic

File: `internal/detect/detect.go`

Add a new block in the `Detect()` function following the
priority order. Place it after existing runtimes of
similar specificity.

```go
// Check for mylang.toml (MyLang)
mylangConfig := filepath.Join(path, "mylang.toml")
if fileExists(mylangConfig) {
    result.Runtime = RuntimeMyLang
    result.Files = append(result.Files, "mylang.toml")
    result.StartCommand = "mylang run"
    result.Port = 8080  // default port, 0 if unknown
    return result
}
```

Detection priority (most specific first):
compose > Dockerfile > package.json > go.mod >
Makefile > pyproject.toml > Cargo.toml > ... >
justfile > Taskfile.yml

Guidelines:
- Detect by **trigger file** (the project manifest)
- Set `StartCommand` to the idiomatic dev command
- Set `Port` if the ecosystem has a conventional default
- Set `HasHotReload = true` if the runtime/framework
  has built-in watch mode
- Set `DevCommand` if different from `StartCommand`

### Step 3: Add to the orchestrate dispatcher

File: `internal/orchestrate/orchestrate.go`

Add the new runtime to the `selectRunner` host case:

```go
case detect.RuntimeNPM, detect.RuntimeGo, ...,
    detect.RuntimeMyLang:
    return d.host, nil
```

### Step 4: Add tests

File: `internal/detect/detect_all_runtimes_test.go`

Add a test case to the table-driven test:

```go
{
    name:     "MyLang",
    files:    map[string]string{"mylang.toml": "name = \"myapp\""},
    expected: RuntimeMyLang,
    command:  "mylang run",
},
```

Test multiple variants if the runtime has them (e.g.,
Maven vs Gradle for Java, Rails vs plain Ruby).

### Step 5: Update README.md

Add a row to the runtimes table:

```markdown
| **MyLang** | `mylang.toml` | `mylang run` |
```

Keep the table alphabetically ordered by runtime name
(after the primary runtimes: Compose, Dockerfile, Node,
Go, Python, Rust).

### Step 6: Verify

```bash
go test -v -run TestDetect_AllRuntimes ./internal/detect/...
make test
make check
```

## Advanced: port inference

If the runtime reads port from a config file, add
inference logic in `internal/detect/infer.go`:

```go
func inferFromMyLang(result *DetectResult, path string) {
    // Read config file and extract port
    data, err := os.ReadFile(filepath.Join(path, "mylang.toml"))
    if err != nil {
        return
    }
    // Parse and set result.Port
}
```

Call it from the detection block:

```go
inferFromMyLang(&result, path)
```

## Advanced: dependency inference

If the runtime's config or env files hint at
infrastructure needs (e.g., a DATABASE_URL), add
inference in `internal/detect/infer_deps.go`.

## Existing runtimes reference

| Runtime | Trigger file | Start command |
|---------|-------------|---------------|
| compose | compose.yml | docker compose up |
| dockerfile | Dockerfile | docker build + run |
| npm | package.json | npm run dev |
| go | go.mod | go run . |
| python | pyproject.toml | python -m flask run |
| rust | Cargo.toml | cargo run |
| php | composer.json | php artisan serve |
| java | pom.xml / build.gradle | ./mvnw spring-boot:run |
| dotnet | *.csproj | dotnet watch run |
| ruby | Gemfile | bundle exec rails server |
| elixir | mix.exs | mix phx.server |
| scala | build.sbt | sbt run |
| swift | Package.swift | swift run |
| dart | pubspec.yaml | dart run |
| clojure | deps.edn | clj -M:dev |
| haskell | stack.yaml / *.cabal | stack run |
| zig | build.zig | zig build run |
| gleam | gleam.toml | gleam run |
| deno | deno.json | deno task dev |
| bun | bunfig.toml | bun run dev |
| make | Makefile | make dev |
| just | justfile | just dev |
| task | Taskfile.yml | task dev |

## Rules

- One trigger file per runtime (use the project manifest)
- Start command must be the idiomatic dev/run command
- Port 0 means "unknown" — Raioz won't configure proxy
- Detection is first-match — order matters
- Always add tests for detection AND command inference
- Update README.md runtimes table
