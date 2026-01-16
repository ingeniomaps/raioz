# Evaluación de Arquitectura del Directorio `cmd/`

**Fecha:** 2025-01-XX

## 📊 Análisis de la Estructura Actual

### Estructura Actual de `cmd/`

```
cmd/
├── root.go                        # 65 líneas - ✅ Correcto
├── up.go                          # 837 líneas - ❌ MUY GRANDE (excede límite 400)
├── down.go                        # 61 líneas - ✅ Bueno (usa DI)
├── status.go                      # 57 líneas - ✅ Bueno (usa DI)
├── list.go                        # 185 líneas - ✅ Aceptable
├── ports.go                       # 67 líneas - ✅ Bueno
├── logs.go                        # 109 líneas - ✅ Aceptable
├── clean.go                       # 178 líneas - ✅ Aceptable
├── check.go                       # 88 líneas - ✅ Bueno
├── compare.go                     # 125 líneas - ✅ Aceptable
├── migrate.go                     # 143 líneas - ✅ Aceptable
├── version.go                     # 44 líneas - ✅ Bueno
├── workspace.go                   # 158 líneas - ✅ Aceptable
├── ignore.go                      # 167 líneas - ✅ Aceptable
├── override.go                    # 211 líneas - ✅ Aceptable
├── link.go                        # 244 líneas - ✅ Aceptable
├── dependency_assist.go           # 270 líneas - ✅ Aceptable
├── ci.go                          # 43 líneas - ✅ Bueno
├── ci_execute.go                  # 314 líneas - ✅ Aceptable
├── ci_helpers.go                  # 79 líneas - ✅ Bueno
├── ci_types.go                    # 25 líneas - ✅ Bueno
├── ci_validations.go              # 177 líneas - ✅ Aceptable
├── edge_cases_errors_test.go      # 709 líneas - ⚠️ Test grande
├── integration_flows_test.go      # 665 líneas - ⚠️ Test grande
├── integration_test.go            # 216 líneas - ✅ Test aceptable
└── version_test.go                # 35 líneas - ✅ Test pequeño
```

**Total:** 26 archivos (22 archivos de producción, 4 archivos de test)

## ✅ Aspectos Positivos

1. **Uso de Cobra:** Estructura correcta para CLI con Cobra
2. **Separación de comandos:** Cada comando tiene su propio archivo
3. **DI parcial:** `down.go` y `status.go` ya usan Dependency Injection
4. **Tests:** Hay tests de integración y edge cases
5. **Organización CI:** Los archivos relacionados con `ci` están agrupados

## ❌ Problemas Identificados

### 1. Archivo `up.go` Excesivamente Grande (837 líneas)

**Problema:** `up.go` tiene **837 líneas**, más del doble del límite de 400 líneas.

**Impacto:**

- Difícil de mantener y entender
- Violación de la regla del proyecto (máximo 400 líneas)
- Demasiada lógica de negocio en la capa de comando
- No usa Dependency Injection como `down.go` y `status.go`

**Análisis de `up.go`:**

- Líneas 35-810: Lógica de ejecución del comando (775 líneas)
- Líneas 814-830: Función helper `formatPortConflicts` (16 líneas)
- Muchas responsabilidades mezcladas:
  - Carga de configuración
  - Validaciones
  - Resolución de workspace
  - Gestión de locks
  - Clonado de repos
  - Generación de compose
  - Manejo de estado
  - Gestión de root config
  - Detección de drift

### 2. Tests Mezclados con Código de Producción

**Problema:** Los archivos `*_test.go` están en `cmd/` junto con el código de producción.

**Análisis:**

- Según las mejores prácticas de Go, esto es **correcto** ✅
- Los tests deben estar en el mismo paquete que el código que prueban
- Sin embargo, los tests grandes (709 y 665 líneas) podrían organizarse mejor

### 3. Archivos Helper de CI Separados

**Problema:** `ci.go` tiene helpers separados (`ci_execute.go`, `ci_helpers.go`, `ci_types.go`, `ci_validations.go`).

**Análisis:**

- Esto es **aceptable** pero podría mejorarse
- Alternativa: Mover lógica a `internal/app/ci.go` (caso de uso)

### 4. Inconsistencia en Uso de Dependency Injection

**Problema:**

- `down.go` y `status.go` usan DI y casos de uso
- `up.go` no usa DI (llama directamente a funciones de paquetes internos)

**Impacto:**

- Inconsistencia arquitectónica
- `up.go` no sigue el mismo patrón que otros comandos
- Más difícil de testear

## 📋 Mejores Prácticas de Go 2026

### 1. Estructura de `cmd/` en Go

Según las mejores prácticas:

```
cmd/
└── raioz/
    └── main.go          # Punto de entrada único
```

**O para múltiples ejecutables:**

```
cmd/
├── app1/
│   └── main.go
└── app2/
    └── main.go
```

**En este proyecto:**

- ✅ Estructura actual con package `cmd` es aceptable para Cobra
- ⚠️ Pero la lógica debería estar en `internal/app/`

### 2. Responsabilidades de `cmd/`

**DEBE hacer:**

- Definir comandos Cobra
- Parsear flags y argumentos
- Validar inputs de usuario
- Llamar a casos de uso en `internal/app/`
- Manejar errores y formatear output

**NO DEBE hacer:**

- Contener lógica de negocio
- Llamar directamente a múltiples paquetes internos
- Contener más de 100-200 líneas por comando

### 3. Organización de Tests

**Correcto:**

- Tests en el mismo paquete (`*_test.go` junto al código)
- Tests de integración pueden estar en `cmd/` o en `internal/`

**Para tests grandes:**

- Dividir en múltiples archivos (`*_test.go` con sufijos)
- Ejemplo: `up_test.go`, `up_integration_test.go`, `up_edge_cases_test.go`

## 🎯 Recomendaciones

### Prioridad ALTA: Migrar `up.go` a Caso de Uso

**Acción:** Crear `internal/app/up.go` con `UpUseCase`

**Beneficios:**

1. ✅ Reduce `cmd/up.go` a ~50-100 líneas
2. ✅ Consistencia con `down.go` y `status.go`
3. ✅ Facilita testing
4. ✅ Sigue Clean Architecture
5. ✅ Cumple con el límite de 400 líneas

**Plan:**

1. Crear `internal/app/up.go` con `UpUseCase`
2. Mover toda la lógica de `cmd/up.go` a `UpUseCase.Execute()`
3. Actualizar `cmd/up.go` para solo inicializar y llamar al caso de uso
4. Verificar que todo funcione correctamente

### Prioridad MEDIA: Organizar Tests

**Acción:** Dividir tests grandes en archivos más pequeños

**Plan:**

1. Dividir `edge_cases_errors_test.go` (709 líneas) en:
   - `up_edge_cases_test.go`
   - `down_edge_cases_test.go`
   - `status_edge_cases_test.go`
   - `common_edge_cases_test.go`
2. Dividir `integration_flows_test.go` (665 líneas) en:
   - `up_integration_test.go`
   - `down_integration_test.go`
   - `workspace_integration_test.go`
   - `override_integration_test.go`

### Prioridad BAJA: Consolidar Archivos CI

**Acción:** Considerar mover lógica de CI a `internal/app/ci.go`

**Alternativa:** Mantener estructura actual (es aceptable si se mantiene consistente)

## 📊 Evaluación por Archivo

| Archivo                | Líneas | Estado       | Acción                        |
| ---------------------- | ------ | ------------ | ----------------------------- |
| `up.go`                | 837    | ❌ CRÍTICO   | Migrar a `internal/app/up.go` |
| `dependency_assist.go` | 270    | ✅ Aceptable | Ninguna                       |
| `link.go`              | 244    | ✅ Aceptable | Ninguna                       |
| `override.go`          | 211    | ✅ Aceptable | Ninguna                       |
| `list.go`              | 185    | ✅ Aceptable | Ninguna                       |
| `clean.go`             | 178    | ✅ Aceptable | Ninguna                       |
| `ci_validations.go`    | 177    | ✅ Aceptable | Ninguna                       |
| `ignore.go`            | 167    | ✅ Aceptable | Ninguna                       |
| `workspace.go`         | 158    | ✅ Aceptable | Ninguna                       |
| `migrate.go`           | 143    | ✅ Aceptable | Ninguna                       |
| `compare.go`           | 125    | ✅ Aceptable | Ninguna                       |
| `logs.go`              | 109    | ✅ Aceptable | Ninguna                       |
| `check.go`             | 88     | ✅ Bueno     | Ninguna                       |
| `ci_helpers.go`        | 79     | ✅ Bueno     | Ninguna                       |
| `ports.go`             | 67     | ✅ Bueno     | Ninguna                       |
| `root.go`              | 65     | ✅ Bueno     | Ninguna                       |
| `down.go`              | 61     | ✅ Bueno     | Ninguna (ya usa DI)           |
| `status.go`            | 57     | ✅ Bueno     | Ninguna (ya usa DI)           |
| `version.go`           | 44     | ✅ Bueno     | Ninguna                       |
| `ci.go`                | 43     | ✅ Bueno     | Ninguna                       |

## ✅ Conclusión

### Estado General: **BUENO con 1 problema crítico**

**Problemas:**

1. ❌ `up.go` es demasiado grande (837 líneas vs 400 límite)
2. ⚠️ Inconsistencia en uso de DI (solo `down` y `status` la usan)

**Fortalezas:**

- ✅ Estructura general es correcta
- ✅ Otros comandos están bien dimensionados
- ✅ Tests están en el lugar correcto
- ✅ Organización de archivos CI es aceptable
- ✅ **Estructura `cmd/raioz/main.go` implementada** (2025-01-XX)

**Acción Requerida:**

1. **URGENTE:** Migrar `up.go` a caso de uso en `internal/app/` (PENDIENTE - requiere refactoring completo)
2. **RECOMENDADO:** Organizar tests grandes en archivos más pequeños
3. **OPCIONAL:** Considerar mover lógica de CI a caso de uso

### Cambios Implementados (2025-01-XX)

1. ✅ **Estructura `cmd/raioz/main.go` creada**

   - Movido `main.go` de raíz a `cmd/raioz/main.go`
   - Eliminado `main.go` de la raíz
   - Proyecto compila correctamente

2. ✅ **Errores de compilación corregidos en `cmd/up.go`**
   - Agregados imports faltantes (`validate`, `lock`)
   - Corregidas referencias a variables (`depsVar` → `deps`)
   - Corregidas llamadas a funciones que no existen en `config.Deps`

### Criterio de Éxito

Cuando `cmd/up.go` tenga menos de 150 líneas y use Dependency Injection como `down.go` y `status.go`, la arquitectura será consistente y seguirá las mejores prácticas.

**Nota:** La migración completa de `up.go` a un caso de uso requiere un refactoring extenso debido a su tamaño (837 líneas) y complejidad. Se recomienda hacerlo en fases.
