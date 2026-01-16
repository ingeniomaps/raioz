# Guía de Desarrollo - Raioz

Esta guía explica cómo desarrollar y contribuir al proyecto Raioz siguiendo los estándares establecidos.

## 📋 Requisitos Previos

- Go 1.22 o superior
- `golangci-lint` (ver instalación abajo)
- `make` (opcional, pero recomendado)

## 🛠️ Instalación de Herramientas

### golangci-lint

```bash
# Linux/macOS
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin latest

# O con Homebrew (macOS)
brew install golangci-lint

# Verificar instalación
golangci-lint --version
```

### goimports

```bash
go install golang.org/x/tools/cmd/goimports@latest
```

### Mockery (Opcional - Para Generación de Mocks)

```bash
go install github.com/vektra/mockery/v2@latest
```

**Nota**: Mockery es necesario solo si vas a generar mocks automáticamente. La configuración está lista en `.mockery.yaml` para cuando se creen interfaces en el código.

## 📏 Estándares de Código

### Reglas Principales

1. **Máximo 400 líneas por archivo**
   - Si excedes, divide el archivo por responsabilidad
   - Ver `.ia/REGLA_400_LINEAS.md` para estrategias

2. **Máximo 120 caracteres por línea**
   - Divide líneas largas de manera legible
   - Funciones con muchos parámetros: dividir en múltiples líneas

3. **Un archivo, un propósito**
   - Cada archivo debe tener una responsabilidad clara
   - Nombre del archivo debe reflejar su propósito

4. **Estándares de Go**
   - Seguir convenciones de naming de Go
   - Manejo apropiado de errores
   - Documentación para funciones exportadas
   - Ver `.ia/CODIGO_STANDARDS.md` para detalles

## 🔍 Verificación de Código

### Verificar Todo

```bash
make check
```

Esto ejecuta:
- Formateo de código
- Verificación de líneas (400 max)
- Verificación de longitud de línea (120 max)
- Linter
- Tests

### Verificación Individual

```bash
# Solo formateo
make format

# Solo linter
make lint

# Solo tests
make test

# Verificar líneas
make check-lines

# Verificar longitud
make check-length

# Script manual
./scripts/check-code-standards.sh
```

## 📝 Workflow de Desarrollo

### 1. Antes de Empezar

Lee la documentación en `.ia/`:
- `DEFINICIONES_PROYECTO.md` - Contexto del proyecto
- `CODIGO_STANDARDS.md` - Estándares de código

### 2. Crear Feature/Bugfix

```bash
# Crear rama
git checkout -b feature/nueva-funcionalidad

# Hacer cambios
# ...

# Verificar antes de commit
make check

# Commit
git commit -m "feat: descripción del cambio"
```

### 3. Antes de Push

```bash
# Ejecutar todas las verificaciones
make ci

# Si todo pasa, push
git push origin feature/nueva-funcionalidad
```

## 🧪 Testing

### Ejecutar Tests

```bash
# Todos los tests
make test

# Tests con cobertura
make test-coverage

# Tests de un paquete específico
go test ./internal/env -v
```

### Escribir Tests

- Un archivo de test por archivo de código
- Nombre: `{archivo}_test.go`
- Funciones de test: `TestXxx`
- Usar tabla de tests cuando sea apropiado

Ejemplo:
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

## 🔧 Comandos Útiles

### Build

```bash
# Build local
make build

# Instalar
make install
```

### Limpieza

```bash
# Limpiar artefactos
make clean
```

### Desarrollo

```bash
# Formatear código
make format

# Solo verificar sin modificar
golangci-lint run

# Generar código (mocks, etc.) - requiere //go:generate directives
make generate

# Generar mocks usando mockery - requiere mockery instalado
make mock
```

**Nota sobre generación de mocks**: Actualmente no hay interfaces en el código, por lo que la generación de mocks no es necesaria. Los mocks manuales en `internal/mocks/` seguirán funcionando hasta que se implementen interfaces y se integre Mockery completamente. La configuración está lista en `.mockery.yaml` para cuando se creen interfaces.

## 🚨 Problemas Comunes

### Archivo excede 400 líneas

1. Identificar responsabilidades múltiples
2. Ver `.ia/REGLA_400_LINEAS.md` para estrategias
3. Dividir en múltiples archivos
4. Mantener coherencia en nombres

### Línea excede 120 caracteres

1. Dividir función con muchos parámetros:
```go
// ❌ Mal
func longFunction(param1, param2, param3, param4 string) error

// ✅ Bien
func longFunction(
    param1, param2 string,
    param3, param4 string,
) error
```

2. Dividir strings largos:
```go
// ✅ Bien
msg := "This is a very long " +
    "string that needs to be " +
    "split across lines"
```

### Errores del Linter

1. Leer el mensaje de error
2. Verificar documentación del linter
3. Ajustar código según la recomendación
4. Si es necesario, agregar excepción en `.golangci.yml`

## 📚 Documentación

### Para Agentes IA

Toda la documentación para agentes IA está en `.ia/`:
- `README.md` - Índice de documentación
- `DEFINICIONES_PROYECTO.md` - Contexto y arquitectura
- `CODIGO_STANDARDS.md` - Estándares detallados
- `REGLA_400_LINEAS.md` - Guía de división de archivos

### Para Desarrolladores

- Este archivo: guía de desarrollo
- `TODO.md`: tareas pendientes y plan
- `project.md`: visión del proyecto
- `como-funciona.md`: funcionamiento desde perspectiva usuario

## 🤝 Contribuir

1. Leer `DEFINICIONES_PROYECTO.md` para entender contexto
2. Revisar `TODO.md` para ver qué está pendiente
3. Crear rama y hacer cambios
4. Seguir estándares de código
5. Escribir tests
6. Verificar con `make check`
7. Crear PR con descripción clara

## 🔗 Enlaces Útiles

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [golangci-lint](https://golangci-lint.run/)
