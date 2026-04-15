# 15 — Zero-config: raioz up without any file

Raioz works without creating a `raioz.yaml` file.
Just run `raioz up` in any project directory.

## How it works

```bash
cd my-project/
raioz up
```

Raioz auto-detects:

1. **Services** — scans subdirectories for known project files:
   - `docker-compose.yml` → Docker Compose
   - `Dockerfile` → Docker build + run
   - `package.json` → npm/pnpm/yarn
   - `go.mod` → Go
   - `Makefile` → Make
   - `pyproject.toml` / `requirements.txt` → Python
   - `Cargo.toml` → Rust
   - `composer.json` → PHP

2. **Dependencies** — reads `.env` files to infer infrastructure:
   - `DATABASE_URL=postgres://...` → adds postgres:16
   - `REDIS_URL=redis://...` → adds redis:7
   - `MONGO_URL=mongodb://...` → adds mongo:7
   - `RABBITMQ_URL=amqp://...` → adds rabbitmq:3
   - `MYSQL_HOST=...` → adds mysql:8
   - And more (kafka, nats, elasticsearch)

3. **Dependencies between services** — wires `dependsOn` automatically
   based on which service's `.env` references which infrastructure.

## Example

```
my-project/
├── api/
│   ├── go.mod
│   └── .env          ← DATABASE_URL=postgres://localhost:5432/api
│                         REDIS_URL=redis://localhost:6379
├── frontend/
│   └── package.json  ← scripts: { "dev": "next dev" }
└── worker/
    └── Makefile      ← dev: go run ./cmd/worker
```

```bash
$ raioz up

  No config file found — auto-detecting project structure...

    api → go (go run .)
    frontend → npm (npm run dev)
    worker → make (make dev)
    postgres → postgres:16 (from api/.env:DATABASE_URL)
    redis → redis:7 (from api/.env:REDIS_URL)

  Auto-detected 3 services, 2 dependencies

  Starting dependencies...
    ✓ postgres (image: postgres:16) :5432
    ✓ redis (image: redis:7) :6379

  Starting services...
    ✓ api (go) :8080
    ✓ frontend (npm) :3000
    ✓ worker (make)
```

## When to create raioz.yaml

Zero-config is great for getting started. Create a `raioz.yaml` when you need:

- **Proxy**: `proxy: true` for HTTPS
- **Custom dependencies**: specific image versions or env files
- **Explicit ordering**: control `dependsOn` precisely
- **Watch mode**: configure which services auto-restart
- **Team sharing**: commit the config so everyone has the same setup

Generate one from your current auto-detected setup:

```bash
raioz init    # scans and writes raioz.yaml
```

Then edit as needed.
