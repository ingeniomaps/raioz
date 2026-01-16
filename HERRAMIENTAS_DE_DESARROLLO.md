# Herramientas de Desarrollo - Raioz

Lista de herramientas implementadas, faltantes y recomendadas para facilitar el desarrollo futuro.

## ✅ Herramientas Implementadas

### Linting y Formato
- ✅ **golangci-lint** - Linter configurado con reglas estrictas
- ✅ **gofmt** - Formateo de código
- ✅ **goimports** - Organización de imports

### Testing
- ✅ **go test** - Framework de testing nativo
- ✅ **Mocks manuales** - `internal/mocks/` con mocks básicos

### Build y CI
- ✅ **Makefile** - Targets para build, test, lint, format
- ✅ **Scripts de validación** - `scripts/check-code-standards.sh`
- ✅ **Pre-commit hooks** - `scripts/setup-hooks.sh`

### Documentación
- ✅ **README.md** - Documentación principal
- ✅ **DEVELOPMENT.md** - Guía de desarrollo
- ✅ **.ia/** - Documentación para agentes IA

## ❌ Herramientas Faltantes (Prioridad ALTA)

### 1. Generación Automática de Mocks

**Problema actual**: Mocks manuales en `internal/mocks/` son básicos y no cubren todas las interfaces.

**Recomendación**: **Mockery** o **gomock**

**Mockery** (Recomendado):
```bash
# Instalación
go install github.com/vektra/mockery/v2@latest

# Configuración: .mockery.yaml
with-expecter: true
packages:
  raioz/internal/docker:
    interfaces:
      Runner:
        filename: "mock_runner.go"
        dir: "internal/mocks/docker"
```

**Ventajas**:
- Generación automática desde interfaces
- Mantenimiento automático cuando cambian interfaces
- Soporte para expectativas en tests
- Integración con `go generate`

**Tareas**:
- [ ] Instalar mockery
- [ ] Crear `.mockery.yaml` con configuración
- [ ] Agregar `//go:generate` directives en interfaces
- [ ] Generar mocks para todas las interfaces
- [ ] Actualizar tests para usar mocks generados
- [ ] Agregar `make generate` target

**Alternativa: gomock**
```bash
go install github.com/golang/mock/mockgen@latest
```

### 2. CI/CD Pipeline

**Problema actual**: No hay CI/CD configurado.

**Recomendación**: **GitHub Actions**

**Tareas**:
- [ ] Crear `.github/workflows/ci.yml` con:
  - Linting (golangci-lint)
  - Testing (unit + integration)
  - Code coverage
  - Build para múltiples plataformas
  - Security scanning
- [ ] Crear `.github/workflows/release.yml` para releases automáticos
- [ ] Agregar badges en README (build status, coverage)

**Ejemplo mínimo**:
```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - run: make ci
```

### 3. Code Coverage

**Problema actual**: Coverage no se reporta en CI.

**Recomendación**: **Codecov** o **Coveralls**

**Tareas**:
- [ ] Agregar `make test-coverage` (ya existe, mejorar)
- [ ] Integrar con Codecov o Coveralls
- [ ] Agregar badge de coverage en README
- [ ] Establecer objetivo de coverage (ej: 80%)
- [ ] Failing builds si coverage baja

### 4. Dependency Management

**Problema actual**: Dependencias no se actualizan automáticamente.

**Recomendación**: **Dependabot** o **Renovate**

**Tareas**:
- [ ] Habilitar Dependabot en GitHub:
  ```yaml
  # .github/dependabot.yml
  version: 2
  updates:
    - package-ecosystem: "gomod"
      directory: "/"
      schedule:
        interval: "weekly"
  ```
- [ ] O usar Renovate (más configurable)
- [ ] Configurar auto-merge para patches menores

### 5. Security Scanning

**Problema actual**: No hay escaneo automático de vulnerabilidades.

**Recomendación**: **gosec** + **Snyk** o **GitHub Dependabot Security**

**Tareas**:
- [ ] gosec ya está en golangci-lint, verificar que funciona
- [ ] Habilitar GitHub Dependabot Security alerts
- [ ] Agregar `make security` target:
  ```makefile
  security:
    @gosec ./...
    @go list -json -m all | nancy sleuth
  ```
- [ ] Integrar en CI

## ⚠️ Herramientas Faltantes (Prioridad MEDIA)

### 6. Fuzzing

**Recomendación**: **go-fuzz** o **native fuzzing** (Go 1.18+)

**Tareas**:
- [ ] Agregar fuzzing tests para:
  - JSON parsing (`deps.json`)
  - Env file parsing
  - Path validation
- [ ] Ejemplo:
  ```go
  func FuzzParseDeps(f *testing.F) {
      f.Add(`{"schemaVersion":"1.0",...}`)
      f.Fuzz(func(t *testing.T, data string) {
          _, _, err := config.LoadDepsFromBytes([]byte(data))
          // Test no panics
      })
  }
  ```
- [ ] Agregar `make fuzz` target

### 7. Benchmarks

**Recomendación**: Benchmarks nativos de Go

**Tareas**:
- [ ] Agregar benchmarks para operaciones críticas:
  - `config.LoadDeps()`
  - `docker.GenerateCompose()`
  - `env.ResolveEnvFiles()`
  - `validate.All()`
- [ ] Ejemplo:
  ```go
  func BenchmarkLoadDeps(b *testing.B) {
      for i := 0; i < b.N; i++ {
          config.LoadDeps("testdata/deps.json")
      }
  }
  ```
- [ ] Agregar `make benchmark` target
- [ ] Comparar benchmarks en CI (detectar regresiones)

### 8. Documentation Generation

**Recomendación**: **godoc** + **pkg.go.dev**

**Tareas**:
- [ ] Mejorar documentación de paquetes (agregar `package` comments)
- [ ] Agregar ejemplos (`ExampleXxx` functions)
- [ ] Generar documentación local: `godoc -http=:6060`
- [ ] Publicar en pkg.go.dev (automático si está en GitHub)
- [ ] Agregar `make docs` target

### 9. Release Automation

**Recomendación**: **goreleaser**

**Tareas**:
- [ ] Instalar goreleaser
- [ ] Crear `.goreleaser.yml`:
  ```yaml
  builds:
    - main: ./main.go
      binary: raioz
      goos:
        - linux
        - darwin
        - windows
      goarch:
        - amd64
        - arm64
  archives:
    - format: tar.gz
  ```
- [ ] Integrar con GitHub Releases
- [ ] Agregar `make release` target

### 10. Performance Profiling

**Recomendación**: **pprof** (nativo de Go)

**Tareas**:
- [ ] Agregar profiling en comandos críticos:
  ```go
  import _ "net/http/pprof"
  go func() {
      log.Println(http.ListenAndServe("localhost:6060", nil))
  }()
  ```
- [ ] Agregar `make profile` target
- [ ] Documentar cómo usar pprof

## 💡 Herramientas Recomendadas (Prioridad BAJA)

### 11. Code Generation

**Recomendación**: **Stringer**, **go generate**

**Tareas**:
- [ ] Usar `go generate` para:
  - Generar mocks (mockery)
  - Generar código repetitivo
  - Generar constantes
- [ ] Ejemplo:
  ```go
  //go:generate stringer -type=ErrorCode
  type ErrorCode string
  ```

### 12. Testing Helpers

**Recomendación**: **testify** o **gomega**

**Tareas**:
- [ ] Evaluar si vale la pena agregar testify:
  ```go
  import "github.com/stretchr/testify/assert"
  assert.Equal(t, expected, actual)
  ```
- [ ] O mantener testing nativo (más simple)

### 13. Development Containers

**Recomendación**: **Dev Containers** (VS Code) o **Docker Compose para dev**

**Tareas**:
- [ ] Crear `.devcontainer/devcontainer.json`
- [ ] O crear `docker-compose.dev.yml` para entorno de desarrollo
- [ ] Documentar setup de desarrollo

### 14. Pre-commit Hooks Avanzados

**Recomendación**: **pre-commit** (framework)

**Tareas**:
- [ ] Evaluar migrar a `pre-commit` framework:
  ```yaml
  # .pre-commit-config.yaml
  repos:
    - repo: https://github.com/golangci/golangci-lint
      hooks:
        - id: golangci-lint
  ```
- [ ] O mantener scripts actuales (más simple)

### 15. Changelog Automation

**Recomendación**: **git-chglog** o **conventional commits**

**Tareas**:
- [ ] Adoptar conventional commits
- [ ] Usar git-chglog para generar CHANGELOG.md
- [ ] Integrar en release process

### 16. Dependency Vulnerability Scanning

**Recomendación**: **nancy** o **govulncheck**

**Tareas**:
- [ ] Instalar nancy:
  ```bash
  go install github.com/sonatypecommunity/nancy@latest
  ```
- [ ] Agregar `make vuln-check` target
- [ ] Integrar en CI

### 17. License Checking

**Recomendación**: **go-licenses**

**Tareas**:
- [ ] Instalar go-licenses:
  ```bash
  go install github.com/google/go-licenses@latest
  ```
- [ ] Verificar compatibilidad de licencias
- [ ] Generar NOTICE file

### 18. Code Complexity Analysis

**Recomendación**: Ya cubierto por **gocyclo** en golangci-lint

**Tareas**:
- [ ] Verificar que gocyclo está funcionando
- [ ] Revisar y reducir complejidad donde sea necesario

## 📋 Plan de Implementación

### Fase 1: Fundamentos (Semana 1)
1. ✅ Mockery para generación de mocks
2. ✅ GitHub Actions CI básico
3. ✅ Code coverage en CI
4. ✅ Dependabot para dependencias

### Fase 2: Calidad (Semana 2)
5. ✅ Security scanning (gosec + Dependabot)
6. ✅ Fuzzing básico
7. ✅ Benchmarks críticos
8. ✅ Documentation generation

### Fase 3: Automatización (Semana 3)
9. ✅ Release automation (goreleaser)
10. ✅ Performance profiling
11. ✅ Pre-commit hooks mejorados
12. ✅ Changelog automation

### Fase 4: Opcionales (Futuro)
13. ⚠️ Testing helpers (testify) - Solo si necesario
14. ⚠️ Dev containers - Solo si equipo lo necesita
15. ⚠️ Dependency vulnerability scanning avanzado
16. ⚠️ License checking

## 🔧 Configuración Recomendada

### Mockery (`.mockery.yaml`)
```yaml
with-expecter: true
mockname: "Mock{{.InterfaceName}}"
filename: "mock_{{.InterfaceName | snakecase }}.go"
dir: "{{.InterfaceDir}}/mocks"
packages:
  raioz/internal/docker:
    interfaces:
      Runner:
  raioz/internal/git:
    interfaces:
      Repository:
  raioz/internal/workspace:
    interfaces:
      Workspace:
```

### GitHub Actions (`.github/workflows/ci.yml`)
```yaml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      - name: Run tests
        run: make test-coverage
      - name: Upload coverage
        uses: codecov/codecov-action@v3
      - name: Lint
        run: make lint
      - name: Security scan
        run: make security
      - name: Build
        run: make build
```

### Makefile (Agregar targets)
```makefile
generate: ## Generate code (mocks, etc.)
	@go generate ./...

mock: ## Generate mocks
	@mockery

fuzz: ## Run fuzzing tests
	@go test -fuzz=./...

benchmark: ## Run benchmarks
	@go test -bench=. -benchmem ./...

profile: ## Run with profiling
	@go test -cpuprofile=cpu.prof -memprofile=mem.prof ./...

security: ## Run security checks
	@gosec ./...
	@go list -json -m all | nancy sleuth

vuln-check: ## Check for vulnerabilities
	@govulncheck ./...

docs: ## Generate documentation
	@echo "Starting godoc server on http://localhost:6060"
	@godoc -http=:6060

release: ## Create a release
	@goreleaser release --snapshot
```

## 📊 Comparación de Herramientas de Mocks

| Herramienta | Pros | Contras | Recomendación |
|------------|------|---------|---------------|
| **Mockery** | ✅ Fácil de usar<br>✅ Soporte para expectativas<br>✅ Integración con go generate | ⚠️ Requiere configuración | ⭐⭐⭐⭐⭐ Recomendado |
| **gomock** | ✅ Oficial de Google<br>✅ Type-safe | ⚠️ Más verboso<br>⚠️ Menos intuitivo | ⭐⭐⭐⭐ Alternativa |
| **Counterfeiter** | ✅ Simple<br>✅ Genera interfaces también | ⚠️ Menos features | ⭐⭐⭐ Si necesitas simplicidad |
| **Mocks manuales** | ✅ Control total<br>✅ Sin dependencias | ❌ Mantenimiento manual<br>❌ No escalable | ⭐⭐ Solo para casos simples |

## 🎯 Recomendación Final

**Para desarrollo futuro, implementar en este orden:**

1. **Mockery** - Generación automática de mocks (ALTA prioridad)
2. **GitHub Actions CI** - Automatización básica (ALTA prioridad)
3. **Code Coverage** - Visibilidad de calidad (ALTA prioridad)
4. **Dependabot** - Mantenimiento de dependencias (ALTA prioridad)
5. **goreleaser** - Releases automáticos (MEDIA prioridad)
6. **Fuzzing** - Robustez (MEDIA prioridad)
7. **Benchmarks** - Performance (MEDIA prioridad)

**No implementar ahora (evaluar después):**
- testify (testing nativo es suficiente)
- Dev containers (solo si equipo lo necesita)
- Pre-commit framework (scripts actuales funcionan)

## 📚 Referencias

- [Mockery Documentation](https://github.com/vektra/mockery)
- [GitHub Actions](https://docs.github.com/en/actions)
- [goreleaser](https://goreleaser.com/)
- [Go Fuzzing](https://go.dev/doc/fuzz/)
- [Codecov](https://about.codecov.io/)
