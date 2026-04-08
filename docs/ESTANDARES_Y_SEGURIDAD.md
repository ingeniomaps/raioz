# Auditorias de Cumplimiento y Seguridad - Raioz

Resultados de auditorias de estandares Go 2025-2026 y seguridad.
Solo hallazgos accionables; los estandares basicos de Go estan en `.context/CODIGO_STANDARDS.md`.

## Estado de Cumplimiento

| Categoria | Score | Estado |
|-----------|-------|--------|
| Version de Go (1.22) | 100% | Cumplido |
| Estructura de Proyecto | 100% | Cumplido |
| Linting (golangci-lint) | 100% | Cumplido |
| Manejo de Errores | 100% | Cumplido |
| Testing | 90% | Bueno |
| Logging (`log/slog`) | 20% | Pendiente |
| Context/Timeouts | 0% | Pendiente |
| Security | 60% | Mejorable |
| CI/CD | 0% | Pendiente |
| **TOTAL** | **57%** | **Mejorable** |

---

## Problemas de Seguridad (por prioridad)

### 1. Command Injection - CRITICO

Uso de `exec.Command` sin validacion de inputs.

**Archivos afectados**:
- `internal/git/readonly.go:27,82` - git clone con `src.Branch`, `src.Repo`
- `internal/docker/runner.go:10,22` - docker compose con `composePath`

**Solucion**:
```go
// Validar inputs antes de exec.Command
func validateGitInput(branch, repo string) error {
    // Branch: solo alfanumericos, guiones, barras, guiones bajos
    if !regexp.MustCompile(`^[a-zA-Z0-9/_.-]+$`).MatchString(branch) {
        return fmt.Errorf("invalid branch name: %s", branch)
    }
    // Repo: formato valido de URL git
    if !regexp.MustCompile(`^(https?://|git@|ssh://)`).MatchString(repo) {
        return fmt.Errorf("invalid repo URL: %s", repo)
    }
    // Rechazar caracteres peligrosos: ; | & $ ` \n
    return nil
}
```

**Tareas**:
- [ ] Crear `validateGitInput(branch, repo)` con regex estricta
- [ ] Crear `validatePath(path)` para paths de archivos
- [ ] Validar todos los inputs antes de `exec.Command`
- [ ] Tests de command injection

### 2. Path Traversal - CRITICO

Paths del usuario sin validar permiten acceso fuera del workspace.

**Archivos afectados**:
- `internal/workspace/migrate.go:36` - `filepath.Join(ws.ServicesDir, svc.Source.Path)`
- `internal/env/resolver.go:38` - `filepath.Join(ws.EnvDir, envFile+".env")`
- `internal/docker/dockerfile.go:15` - `filepath.Join(servicePath, dockerfile)`

**Solucion**:
```go
func validatePathInBase(path, baseDir string) error {
    absBase, _ := filepath.Abs(baseDir)
    absTarget, _ := filepath.Abs(filepath.Join(baseDir, path))
    if !strings.HasPrefix(absTarget, absBase+string(os.PathSeparator)) {
        return fmt.Errorf("path traversal detected: %s", path)
    }
    return nil
}
```

**Tareas**:
- [ ] Crear `validatePathInBase(path, baseDir)` con `filepath.Abs`
- [ ] Aplicar en todos los `filepath.Join` con inputs del usuario
- [ ] Rechazar paths con `..` y symlinks peligrosos
- [ ] Tests de path traversal

### 3. Secrets en Logs - ALTO

Variables de entorno con secrets pueden filtrarse en logs o errores.

**Archivos afectados**:
- `internal/env/resolver.go:133` - secrets en memoria
- Cualquier `fmt.Printf` o error que incluya valores de env

**Solucion**:
```go
var sensitiveKeys = map[string]bool{
    "PASSWORD": true, "SECRET": true, "TOKEN": true,
    "KEY": true, "API_KEY": true,
}

func sanitizeEnvValue(key, value string) string {
    for sk := range sensitiveKeys {
        if strings.Contains(strings.ToUpper(key), sk) {
            return "***REDACTED***"
        }
    }
    return value
}
```

**Tareas**:
- [ ] Crear `sanitizeEnvValue(key, value)` con lista de keys sensibles
- [ ] Aplicar en todos los puntos donde se imprimen env vars
- [ ] Sanitizar mensajes de error que puedan contener secrets

### 4. File Permissions - MEDIO

Permisos demasiado permisivos en archivos sensibles.

**Archivos afectados**:
- `internal/lock/lock.go:24` - lock file con `0644`
- `internal/state/state.go:19` - state file con `0644`
- `internal/workspace/workspace.go:112-134` - directorios con `0755`
- `internal/docker/dockerfile.go:53` - Dockerfile wrapper con `0644`

**Correccion directa**:
```go
// Archivos sensibles (state, lock, env): 0600
os.WriteFile(statePath, data, 0600)
os.WriteFile(lockPath, data, 0600)
// Directorios de workspace: 0700
os.MkdirAll(dir, 0700)
```

**Tareas**:
- [ ] Cambiar permisos de lock, state y env files a `0600`
- [ ] Cambiar permisos de directorios de workspace a `0700`

### 5. Validacion de Inputs - MEDIO

Configuracion del usuario (`raioz.json`) sin validacion de contenido peligroso.

**Tareas**:
- [ ] Validar longitud maxima de strings (project name: 63, path: 255)
- [ ] Validar formato de nombres con regex (`^[a-z0-9-]+$`)
- [ ] Validar URLs de repositorios
- [ ] Validar puertos (rango 1-65535)
- [ ] Limitar tamano de archivo de config (max 1MB)

---

## Estandares Pendientes de Implementar

### Context y Timeouts

No se usa `context.Context` para cancelacion ni timeouts en comandos externos.

**Archivos afectados**:
- `internal/docker/runner.go:10-13` - `docker compose up` sin timeout
- `internal/git/readonly.go:27-31` - `git clone` sin timeout
- Todos los `exec.Command().Run()` sin context

**Solucion**:
```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d")
if err := cmd.Run(); err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        return fmt.Errorf("command timed out after 5 minutes")
    }
    return err
}
```

**Timeouts recomendados**:
- Git clone: 10 min
- Docker compose up: 5 min
- Docker compose down: 2 min
- Docker pull: 15 min

**Tareas**:
- [ ] Agregar `context.Context` como primer parametro a funciones con comandos externos
- [ ] Usar `exec.CommandContext()` en lugar de `exec.Command()`
- [ ] Manejar `context.DeadlineExceeded` con mensajes claros
- [ ] Agregar flag `--timeout` para override

### Logging Estructurado (`log/slog`)

Se usa `fmt.Printf` en lugar de logging estructurado.

**Archivos afectados**:
- `internal/workspace/workspace.go:103`
- `internal/git/branch.go:112`
- `internal/workspace/migrate.go:66`
- `internal/validate/validate.go:50`

**Solucion**:
```go
// Reemplazar:
fmt.Printf("Using workspace directory: %s\n", base)
// Por:
slog.Info("Using workspace directory", "path", base)
```

**Tareas**:
- [ ] Reemplazar `fmt.Printf` con `slog` en codigo interno
- [ ] Definir niveles de log (DEBUG, INFO, WARN, ERROR)
- [ ] Agregar flag `--log-level`
- [ ] Sanitizar secrets en logs

### CI/CD Pipeline

No hay CI/CD configurado.

**Tareas**:
- [ ] Crear `.github/workflows/ci.yml` (tests, lint, build)
- [ ] Integrar `govulncheck` en CI
- [ ] Integrar code coverage (Codecov/Coveralls, objetivo 80%)
- [ ] Agregar badge de coverage en README

### govulncheck

`gosec` esta habilitado, pero falta `govulncheck` (herramienta oficial de Go).

**Tareas**:
- [ ] Ya existe `make security` con gosec + govulncheck
- [ ] Integrar en CI/CD pipeline
- [ ] Revisar dependencias regularmente

---

## Buenas Practicas Pendientes

### Testing de Seguridad

- [ ] Tests de command injection con inputs maliciosos
- [ ] Tests de path traversal con `../` y symlinks
- [ ] Tests de race conditions (`go test -race`)
- [ ] Fuzzing para parsers de JSON y env files

### Generacion de Mocks

Mocks manuales en `internal/mocks/`. Considerar mockery para generacion automatica.

- [ ] Configurar `.mockery.yaml`
- [ ] Agregar `//go:generate` directives
- [ ] Generar mocks desde interfaces existentes

---

## Plan de Accion Priorizado

### Semana 1-2: Seguridad Critica
1. Validacion de inputs para `exec.Command` (command injection)
2. Validacion de paths (path traversal)
3. Permisos de archivos a `0600`/`0700`
4. Sanitizacion de secrets en logs/errores

### Semana 3-4: Robustez
5. `context.Context` y timeouts en comandos externos
6. Logging estructurado con `log/slog`

### Semana 5-6: Infraestructura
7. CI/CD pipeline con GitHub Actions
8. Code coverage en CI
9. `govulncheck` en pipeline

### Semana 7-8: Mejoras
10. Mockery para mocks automaticos
11. Tests de seguridad y fuzzing
