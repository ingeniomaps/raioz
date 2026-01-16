# Seguridad, Buenas Prácticas y Calidad de Código

Análisis de seguridad, buenas prácticas de Go y mejoras de calidad de código para el proyecto Raioz.

## 🔒 Seguridad

### 1. Command Injection (CRÍTICO)

**Problema**: Uso de `exec.Command` sin validación adecuada de inputs.

**Ubicaciones afectadas**:
- `internal/git/readonly.go:27` - `exec.Command("git", "clone", "-b", src.Branch, src.Repo, target)`
- `internal/git/readonly.go:82` - `exec.Command("git", "clone", "-b", src.Branch, src.Repo, target)`
- `internal/docker/runner.go:10` - `exec.Command("docker", "compose", "-f", composePath, "up", "-d")`
- `internal/docker/runner.go:22` - `exec.Command("docker", "compose", "-f", composePath, "down")`
- Todos los usos de `exec.Command` en el proyecto

**Riesgo**: Si `src.Branch`, `src.Repo`, `composePath` o cualquier input contienen caracteres especiales o comandos maliciosos, pueden ejecutarse comandos arbitrarios.

**Solución**:
```go
// ❌ MAL
cmd := exec.Command("git", "clone", "-b", src.Branch, src.Repo, target)

// ✅ BIEN
cmd := exec.Command("git", "clone", "-b", src.Branch, src.Repo, target)
cmd.Args = append(cmd.Args, target) // Siempre usar argumentos separados
// Validar inputs antes:
if err := validateGitInput(src.Branch, src.Repo); err != nil {
    return err
}
```

**Tareas**:
- [ ] Crear función `validateGitInput(branch, repo string) error` que valide:
  - Branch: solo caracteres alfanuméricos, guiones, barras, guiones bajos
  - Repo: formato válido de URL git (ssh://, https://, git@)
  - Rechazar caracteres especiales peligrosos (`;`, `|`, `&`, `$`, `` ` ``, `\n`)
- [ ] Crear función `validatePath(path string) error` para validar paths de archivos
- [ ] Validar todos los inputs antes de pasarlos a `exec.Command`
- [ ] Usar `filepath.Clean()` y `filepath.Join()` para normalizar paths
- [ ] Agregar tests para command injection

### 2. Path Traversal (ALTO)

**Problema**: Paths de archivos no validados pueden permitir acceso fuera del workspace.

**Ubicaciones afectadas**:
- `internal/workspace/migrate.go:36` - `filepath.Join(ws.ServicesDir, svc.Source.Path)`
- `internal/env/resolver.go:38` - `filepath.Join(ws.EnvDir, envFile+".env")`
- `internal/docker/dockerfile.go:15` - `filepath.Join(servicePath, dockerfile)`
- Todos los usos de `filepath.Join` con inputs del usuario

**Riesgo**: Si `svc.Source.Path` contiene `../`, puede acceder a archivos fuera del workspace.

**Solución**:
```go
// ❌ MAL
targetPath := filepath.Join(baseDir, userPath)

// ✅ BIEN
targetPath := filepath.Join(baseDir, userPath)
// Validar que el path resultante está dentro del baseDir
absBase, _ := filepath.Abs(baseDir)
absTarget, _ := filepath.Abs(targetPath)
if !strings.HasPrefix(absTarget, absBase) {
    return fmt.Errorf("path traversal detected: %s", userPath)
}
```

**Tareas**:
- [ ] Crear función `validatePathInBase(path, baseDir string) error` que:
  - Normalice paths con `filepath.Abs()`
  - Verifique que el path resultante esté dentro de `baseDir`
  - Rechace paths con `..` o symlinks peligrosos
- [ ] Aplicar validación en todos los lugares donde se construyen paths desde inputs
- [ ] Agregar tests para path traversal

### 3. File Permissions (MEDIO)

**Problema**: Permisos de archivos demasiado permisivos.

**Ubicaciones afectadas**:
- `internal/lock/lock.go:24` - `os.OpenFile(..., 0644)` - Lock file legible por todos
- `internal/workspace/workspace.go:112-134` - `os.MkdirAll(..., 0755)` - Directorios ejecutables por todos
- `internal/docker/dockerfile.go:53` - `os.WriteFile(..., 0644)` - Dockerfile wrapper legible por todos
- `internal/state/state.go:19` - `os.WriteFile(..., 0644)` - State file legible por todos

**Riesgo**: Archivos sensibles (state, lock, env files) pueden ser leídos por otros usuarios.

**Solución**:
```go
// ❌ MAL
os.WriteFile(path, data, 0644) // Legible por todos

// ✅ BIEN
// Lock files: 0600 (solo owner)
os.WriteFile(lockPath, data, 0600)
// State files: 0600 (solo owner)
os.WriteFile(statePath, data, 0600)
// Env files: 0600 (solo owner, contienen secrets)
os.WriteFile(envPath, data, 0600)
// Directorios: 0700 (solo owner puede acceder)
os.MkdirAll(dir, 0700)
```

**Tareas**:
- [ ] Cambiar permisos de lock files a `0600`
- [ ] Cambiar permisos de state files a `0600`
- [ ] Cambiar permisos de env files a `0600` (contienen secrets)
- [ ] Cambiar permisos de directorios de workspace a `0700`
- [ ] Documentar política de permisos en README

### 4. Secrets en Logs (ALTO)

**Problema**: Variables de entorno con secrets pueden aparecer en logs o errores.

**Ubicaciones afectadas**:
- `internal/env/resolver.go:133` - `env[key] = value` - Secrets se cargan en memoria
- `cmd/up.go:65-69` - `os.Environ()` - Todas las env vars se cargan
- Cualquier lugar donde se imprimen errores con context

**Riesgo**: Secrets pueden aparecer en logs, errores, o stdout/stderr.

**Solución**:
```go
// ❌ MAL
fmt.Printf("Error: %v", err) // Puede contener secrets

// ✅ BIEN
// Lista de keys sensibles
sensitiveKeys := map[string]bool{
    "PASSWORD": true,
    "SECRET": true,
    "TOKEN": true,
    "KEY": true,
    "API_KEY": true,
}

func sanitizeEnvValue(key, value string) string {
    if sensitiveKeys[strings.ToUpper(key)] {
        return "***REDACTED***"
    }
    return value
}

// Al imprimir errores, sanitizar
func formatError(err error) string {
    // No incluir env vars completas en errores
}
```

**Tareas**:
- [ ] Crear función `sanitizeEnvValue(key, value string) string` que redacte valores sensibles
- [ ] Lista de keys sensibles (PASSWORD, SECRET, TOKEN, KEY, API_KEY, etc.)
- [ ] Aplicar sanitización en todos los lugares donde se imprimen env vars
- [ ] Aplicar sanitización en mensajes de error que puedan contener secrets
- [ ] Agregar flag `--verbose` para debugging (solo mostrar secrets con flag explícito)

### 5. Timeouts y Context (MEDIO)

**Problema**: Comandos externos sin timeouts pueden colgarse indefinidamente.

**Ubicaciones afectadas**:
- `internal/docker/runner.go:10-13` - `docker compose up` sin timeout
- `internal/git/readonly.go:27-31` - `git clone` sin timeout
- Todos los `exec.Command().Run()` sin context

**Riesgo**: Comandos pueden colgarse, bloqueando la ejecución.

**Solución**:
```go
// ❌ MAL
cmd := exec.Command("docker", "compose", "up", "-d")
return cmd.Run()

// ✅ BIEN
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()

cmd := exec.CommandContext(ctx, "docker", "compose", "up", "-d")
cmd.Stdout = os.Stdout
cmd.Stderr = os.Stderr
if err := cmd.Run(); err != nil {
    if ctx.Err() == context.DeadlineExceeded {
        return fmt.Errorf("command timed out after 5 minutes")
    }
    return err
}
```

**Tareas**:
- [ ] Agregar `context.Context` a todas las funciones que ejecutan comandos externos
- [ ] Definir timeouts apropiados:
  - Git clone: 10 minutos
  - Docker compose up: 5 minutos
  - Docker compose down: 2 minutos
  - Docker pull: 15 minutos
- [ ] Usar `exec.CommandContext()` en lugar de `exec.Command()`
- [ ] Manejar `context.DeadlineExceeded` con mensajes claros
- [ ] Agregar flag `--timeout` para permitir override

### 6. Validación de Inputs (MEDIO)

**Problema**: Inputs de usuario no validados suficientemente.

**Ubicaciones afectadas**:
- `internal/config/deps.go` - Carga de `deps.json` sin validación de contenido malicioso
- `internal/validate/validate.go` - Validación de schema pero no de contenido peligroso
- Todos los campos de `deps.json` que se usan directamente

**Riesgo**: Configuración maliciosa puede causar comportamiento inesperado.

**Solución**:
```go
// Validar longitud máxima de strings
const (
    MaxProjectNameLength = 63
    MaxServiceNameLength = 63
    MaxPathLength = 255
)

func validateProjectName(name string) error {
    if len(name) > MaxProjectNameLength {
        return fmt.Errorf("project name too long (max %d)", MaxProjectNameLength)
    }
    // Validar formato
    if !regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(name) {
        return fmt.Errorf("invalid project name format")
    }
    return nil
}
```

**Tareas**:
- [ ] Agregar validación de longitud máxima para todos los campos de string
- [ ] Validar formato de nombres (project, service, network) con regex
- [ ] Validar URLs de repositorios (formato válido)
- [ ] Validar paths (sin `..`, sin caracteres especiales)
- [ ] Validar puertos (rango válido, formato correcto)
- [ ] Agregar límites de tamaño para `deps.json` (ej: max 1MB)

## 📐 Buenas Prácticas de Go

### 1. Manejo de Errores

**Estado actual**: ✅ Bueno - Se usa sistema de errores estructurado (`internal/errors`)

**Mejoras**:
- [ ] Agregar `errors.Is()` y `errors.As()` donde sea apropiado
- [ ] Asegurar que todos los errores se envuelven con `fmt.Errorf("...: %w", err)`
- [ ] Documentar códigos de error en `internal/errors/types.go`
- [ ] Agregar tests para verificar que errores se envuelven correctamente

### 2. Context Propagation

**Problema**: No se usa `context.Context` para cancelación y timeouts.

**Tareas**:
- [ ] Agregar `context.Context` como primer parámetro a todas las funciones que:
  - Ejecutan comandos externos
  - Hacen I/O (archivos, red)
  - Realizan operaciones que pueden tardar
- [ ] Usar `context.WithTimeout()` para operaciones con límite de tiempo
- [ ] Usar `context.WithCancel()` para operaciones cancelables
- [ ] Propagar context a través de todas las llamadas
- [ ] Agregar tests para verificar cancelación

### 3. Logging

**Problema**: Uso inconsistente de `fmt.Printf` en lugar de logging estructurado.

**Ubicaciones afectadas**:
- `internal/workspace/workspace.go:103` - `fmt.Printf("ℹ️  Using workspace directory...")`
- `internal/git/branch.go:112` - `fmt.Printf("⚠️  Branch changed...")`
- `internal/workspace/migrate.go:66` - `fmt.Printf("ℹ️  Migrated...")`
- `internal/validate/validate.go:50` - `fmt.Printf("⚠️  Compatibility warnings...")`

**Solución**:
```go
// ❌ MAL
fmt.Printf("ℹ️  Using workspace directory: %s\n", base)

// ✅ BIEN
// Usar logging estructurado
import "log/slog"

logger := slog.Default()
logger.Info("Using workspace directory", "path", base)
```

**Tareas**:
- [ ] Agregar `log/slog` o `logrus` para logging estructurado
- [ ] Reemplazar todos los `fmt.Printf` con logging apropiado
- [ ] Definir niveles de log (DEBUG, INFO, WARN, ERROR)
- [ ] Agregar flag `--log-level` para controlar verbosidad
- [ ] Agregar formato JSON para logs (útil para CI/CD)
- [ ] No loggear secrets (aplicar sanitización)

### 4. Testing

**Estado actual**: ✅ Bueno - Hay tests unitarios

**Mejoras**:
- [ ] Agregar tests de integración para comandos completos
- [ ] Agregar tests de seguridad (command injection, path traversal)
- [ ] Agregar tests de race conditions (`go test -race`)
- [ ] Agregar benchmarks para operaciones críticas
- [ ] Agregar tests de fuzzing para parsers (JSON, env files)
- [ ] Aumentar cobertura de código (objetivo: >80%)

### 5. Dependencies

**Estado actual**: ✅ Bueno - Dependencias mínimas y actualizadas

**Mejoras**:
- [ ] Agregar `go.mod` checksum verification
- [ ] Usar `go mod tidy` regularmente
- [ ] Revisar dependencias con `go list -m -u all`
- [ ] Agregar dependabot o renovate para actualizaciones automáticas
- [ ] Documentar política de versiones (¿usar versiones fijas o ranges?)

### 6. Code Organization

**Estado actual**: ✅ Bueno - Estructura clara

**Mejoras**:
- [ ] Agregar documentación de paquetes (`package docker // Package docker provides Docker Compose integration`)
- [ ] Agregar ejemplos de uso en documentación de funciones públicas
- [ ] Revisar y mejorar nombres de funciones/variables
- [ ] Agregar comentarios para funciones complejas

### 7. Error Messages

**Estado actual**: ✅ Bueno - Sistema de errores estructurado

**Mejoras**:
- [ ] Asegurar que todos los errores tienen sugerencias
- [ ] Agregar códigos de error únicos para cada tipo de error
- [ ] Documentar códigos de error en README
- [ ] Agregar links a documentación en mensajes de error

## 🧹 Calidad de Código

### 1. Linting

**Estado actual**: ✅ Bueno - Hay `.golangci.yml`

**Mejoras**:
- [ ] Revisar configuración de golangci-lint
- [ ] Habilitar más linters:
  - `gosec` - Security issues
  - `govet` - Vet checks
  - `ineffassign` - Ineffective assignments
  - `staticcheck` - Static analysis
  - `unused` - Unused code
- [ ] Agregar pre-commit hook para linting
- [ ] Failing builds si hay errores de lint

### 2. Code Coverage

**Tareas**:
- [ ] Agregar `go test -cover` al CI
- [ ] Establecer objetivo de cobertura (ej: 80%)
- [ ] Agregar badge de cobertura en README
- [ ] Revisar y mejorar cobertura de código crítico

### 3. Documentation

**Tareas**:
- [ ] Agregar `godoc` comments a todas las funciones públicas
- [ ] Agregar ejemplos de uso (`ExampleXxx` functions)
- [ ] Generar documentación con `go doc`
- [ ] Publicar documentación en godoc.org o similar

### 4. Performance

**Tareas**:
- [ ] Agregar benchmarks para operaciones críticas
- [ ] Optimizar operaciones de I/O (usar buffering donde sea apropiado)
- [ ] Revisar uso de memoria (evitar allocations innecesarias)
- [ ] Agregar profiling para identificar bottlenecks

### 5. Concurrency

**Tareas**:
- [ ] Revisar uso de goroutines (si hay)
- [ ] Agregar tests de race conditions
- [ ] Documentar comportamiento concurrente
- [ ] Usar `sync` package apropiadamente (mutex, waitgroup, etc.)

## 📋 Checklist de Implementación

### Prioridad ALTA (Seguridad Crítica)
- [ ] Command injection: Validar todos los inputs a `exec.Command`
- [ ] Path traversal: Validar todos los paths construidos desde inputs
- [ ] File permissions: Cambiar permisos de archivos sensibles a `0600`
- [ ] Secrets en logs: Sanitizar valores sensibles en logs y errores

### Prioridad MEDIA (Seguridad y Robustez)
- [ ] Timeouts: Agregar context y timeouts a comandos externos
- [ ] Validación de inputs: Validar longitud, formato, y contenido
- [ ] Logging estructurado: Reemplazar `fmt.Printf` con logging apropiado
- [ ] Context propagation: Agregar context a funciones que lo necesiten

### Prioridad BAJA (Mejoras de Calidad)
- [ ] Testing: Agregar tests de seguridad, integración, y fuzzing
- [ ] Documentation: Mejorar documentación de código
- [ ] Performance: Agregar benchmarks y optimizaciones
- [ ] Code coverage: Aumentar cobertura y agregar CI checks

## 🔗 Referencias

- [Go Security Best Practices](https://go.dev/doc/security/best-practices)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://go.dev/doc/effective_go)
