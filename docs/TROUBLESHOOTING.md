# Troubleshooting

Common issues and how to fix them.

## Docker

### Docker is not running

```
[error] Docker is not running
```

**Fix:** Start Docker Desktop or the daemon:
```bash
# Linux
sudo systemctl start docker

# macOS
open -a Docker
```

Verify: `docker info`

### Port already in use

```
[error] Port 5432 is already in use by project 'other-project'
```

**Fix:** Stop the conflicting project or change the port:
```bash
# Stop the other project
raioz down --project other-project

# Or check what's using the port
lsof -i :5432
```

### Network already exists with different config

```
[error] Network 'raioz-myproject-net' already exists
```

**Fix:** Remove the stale network:
```bash
docker network rm raioz-myproject-net
raioz up
```

### Containers not stopping

```bash
# Force cleanup
raioz clean

# Nuclear option: remove every raioz-managed container for this project
docker ps -a --filter "label=com.raioz.managed=true" \
             --filter "label=com.raioz.project=myproject" \
             -q | xargs -r docker rm -f

# Nuclear option across ALL projects (careful — hits every workspace on this host)
docker ps -a --filter "label=com.raioz.managed=true" -q | xargs -r docker rm -f
```

Raioz stamps every container with `com.raioz.managed=true` and
`com.raioz.project=<project>`. Filtering by label (not name prefix) is
how `raioz down` itself works; the commands above mirror that.

---

## Proxy / HTTPS

### mkcert not installed

```
[error] mkcert is required for local HTTPS
```

**Fix:** Install mkcert:
```bash
# macOS
brew install mkcert
mkcert -install

# Linux
# See https://github.com/FiloSottile/mkcert#installation
```

### Certificate not trusted

Browser shows "Your connection is not private" for `https://api.localhost`.

**Fix:** Install the CA certificate:
```bash
mkcert -install
# Restart your browser
```

### Proxy not routing to service

Service is running but `https://api.localhost` returns 502.

**Fix:** Check that the service port is correct:
```bash
raioz status    # verify service is running and on which port
raioz ports     # check port mappings
```

The proxy routes to the port Raioz detected. If auto-detection
got it wrong, set `ports` explicitly in `raioz.yaml`.

### Dep has an HTTP UI but no proxy route is created

Images like `dpage/pgadmin4`, `redis/redisinsight`, `mongo-express`,
and `rabbitmq:management` serve web UIs — but raioz treats certain
images as non-HTTP by default (postgres, redis, mysql, mongo,
rabbitmq, kafka, etc.) and skips creating an HTTPS route.

**Fix:** Opt in explicitly with `routing:`:
```yaml
dependencies:
  adminer:
    image: adminer
    routing: {}             # opt-in; empty object is enough
```

An empty `routing: {}` is sufficient — it just signals "yes, proxy
this". Add `ws: true`, `stream: true`, or `grpc: true` inside if you
need those.

### Two workspaces fighting for ports 80/443

Second `raioz up` fails with "host port 80 already in use" because
another workspace's Caddy is bound there.

**Fix:** Run each workspace on its own subnet without host port
binding. Proxy becomes reachable only via its container IP:

```yaml
workspace: acme-corp
project: e-commerce
network:
  subnet: 172.28.0.0/16        # pinned subnet = deterministic IPs
proxy:
  domain: acme.localhost
  publish: false               # don't bind host 80/443
```

Then map hostnames via `/etc/hosts`:

```bash
raioz hosts | sudo tee -a /etc/hosts
# 172.28.1.1 api.acme.localhost frontend.acme.localhost   # raioz:acme-corp
```

**Linux-only** — macOS and Windows route Docker through a VM whose
bridge IPs aren't reachable from the host.

### `command:` launches its own compose, proxy can't find the service

When a service uses `command: make start` (or any script that spawns
its own `docker compose`), raioz can't introspect the resulting
containers. It classifies the service as "host" and the proxy ends
up pointing at `host.docker.internal` with no port — 502 everywhere.

**Fix:** Tell the proxy explicitly where to forward:

```yaml
services:
  keycloak:
    path: ./keycloak
    command: make start
    stop: make stop
    proxy:
      target: hypixo-keycloak   # container name on the shared network
      port: 8080
```

### Dependency container name collides with external tooling

Raioz names shared deps `{workspace}-{dep}` (e.g. `acme-postgres`),
but your IDE plugin / backup script / external client expects a
specific literal name like `my-app-postgres`.

**Fix:** Override the container name:

```yaml
dependencies:
  postgres:
    image: postgres:16
    name: my-app-postgres       # literal override, raioz uses this verbatim
```

When `name:` is set, the dep is also treated as workspace-shared
regardless of whether `workspace:` is declared, because the override
signals "this is a named, durable container other tools depend on".

---

## Services

### Service detected as wrong runtime

```
api → make (make dev)     # expected: go (go run .)
```

Raioz uses priority-based detection. A `Makefile` in the same
directory as `go.mod` wins because Make has higher priority.

**Fix:** Remove the ambiguity, or set the service path to a
subdirectory that only has the intended project file.

### Host service won't start

```
[error] Failed to start service 'api': exec: "go": executable not found
```

**Fix:** The runtime tool must be in your PATH. Verify:
```bash
which go    # or npm, python, cargo, etc.
```

### Stale PID after crash

```
[error] Service 'api' already running (pid: 12345)
```

If the process actually died but the PID wasn't cleaned up:
```bash
raioz down          # cleans stale PIDs
raioz up            # fresh start
```

---

## Config

### "No config file found"

```
No config file found — auto-detecting project structure...
```

This is normal — Raioz works without config (zero-config mode).
To create one: `raioz init`

### Project name validation error

```
[error] name contains invalid characters
```

Project names must be valid Docker resource names: lowercase
letters, numbers, and hyphens only. Max 63 characters.

**Fix:** Rename in `raioz.yaml`:
```yaml
project: my-project    # not "My Project" or "my_project"
```

### Migrating from .raioz.json

```
[!!] .raioz.json is deprecated. Run 'raioz migrate yaml' to convert.
```

**Fix:**
```bash
raioz migrate yaml    # generates raioz.yaml from .raioz.json
```

Review the generated file, then delete `.raioz.json`.

---

## Watch mode

### Changes not detected

File watcher uses fsnotify. Some editors (e.g., Vim with swap files)
write to temporary files then rename, which may not trigger events.

**Fix:** Verify the file watcher is active:
```bash
raioz up    # with watch: true in config
# Look for "Watching N service(s)" in output
```

### Too many restarts

Rapid file saves cause multiple restarts.

Raioz debounces at 500ms. If that's not enough, consider using
`watch: native` and letting your framework handle hot-reload
(Next.js, Vite, Air for Go, etc.).

---

## General

### Run diagnostics

```bash
raioz doctor
```

Checks: Docker, Docker Compose, Git, disk space, config directory.

### Reset everything

```bash
raioz down            # stop services
raioz clean           # remove stopped containers
docker system prune   # clean Docker (careful: affects all projects)
```

### Debug mode

Set `RAIOZ_DEBUG=1` for verbose logging:
```bash
RAIOZ_DEBUG=1 raioz up
```
