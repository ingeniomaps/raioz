# Raioz Examples

Real-world use cases showing how Raioz orchestrates different project types.

| # | Example | What it shows |
|---|---------|---------------|
| [01](01-minimal.yaml) | **Minimal** | One service, one dependency. The simplest config. |
| [02](02-fullstack.yaml) | **Fullstack** | Frontend (npm) + backend (Go) + infra. Proxy enabled. |
| [03](03-microservices.yaml) | **Microservices** | 4+ services with cross-dependencies and ordering. |
| [04](04-multi-project/) | **Multi-project** | Two projects sharing workspace, network, and infra. |
| [05](05-existing-compose.yaml) | **Existing Compose** | Migrate a docker-compose.yml project without rewriting. |
| [06](06-no-docker.yaml) | **No Docker** | All services on host. Docker only for infra. |
| [07](07-mixed-runtimes.yaml) | **Mixed runtimes** | Go + Node + Python + PHP + Rust in one project. |
| [08](08-kubernetes-local.yaml) | **Kubernetes + Docker** | Kind, Docker containers, and host processes together. |
| [09](09-secrets-and-hooks.yaml) | **Secrets & hooks** | Infisical, Vault, Doppler integration via pre/post hooks. |
| [10](10-proxy-https.yaml) | **Proxy & HTTPS** | Local mkcert + shared dev server with Let's Encrypt. |
| [11](11-advanced-routing.yaml) | **Advanced routing** | WebSocket, SSE, gRPC, and tunnel configuration. |
| [12](12-monorepo.yaml) | **Monorepo** | Turborepo/pnpm workspace with multiple packages. |
| [13](13-port-conflicts.yaml) | **Port conflicts** | Multiple services on same port, solved by proxy. |
| [14](14-dev-swap.yaml) | **Dev swap** | Promote dependency from image to local with `raioz dev`. |
| [15](15-zero-config.md) | **Zero-config** | `raioz up` without any config file. |

## Quick start

```bash
# Copy any example to your project
cp examples/02-fullstack.yaml my-project/raioz.yaml

# Edit paths to match your project structure
vim my-project/raioz.yaml

# Run
cd my-project/
raioz up
```

## Or skip examples entirely

```bash
cd my-project/
raioz init     # auto-scan and generate raioz.yaml
raioz up       # start everything
```
