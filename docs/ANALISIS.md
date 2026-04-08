# Análisis: Estado Actual vs Documentos de Referencia

Análisis comparativo entre el estado actual del proyecto y los documentos de referencia (caso-real.md, como-funciona.md, escenario.md, project.md).

## ✅ Lo que está BIEN implementado

### 1. Arquitectura Core
- ✅ **Binario único**: Se instala una vez, no requiere repo del orquestador
- ✅ **Configuración declarativa**: Un solo archivo `deps.json` por proyecto
- ✅ **No invasión**: Los microservicios no saben que Raioz existe
- ✅ **Workspace centralizado**: `/opt/raioz-proyecto/` con fallback a `~/.raioz/`
- ✅ **Estructura de workspace**: Separación `local/` y `readonly/` implementada

### 2. Funcionalidades Principales
- ✅ **Clonado selectivo**: Solo clona repos listados en deps.json
- ✅ **Soporte git e image**: Servicios pueden ser repos Git o imágenes Docker
- ✅ **Modo readonly**: Implementado con `access: "readonly"` para servicios dependientes
- ✅ **Variables de entorno centralizadas**: Estructura `/env/` con global, services, projects
- ✅ **Modo dev/prod**: Diferentes configuraciones según modo
- ✅ **Idempotencia**: Seguro ejecutar múltiples veces
- ✅ **Detección de conflictos**: Puertos, volúmenes, redes

### 3. Validación y Robustez
- ✅ **JSON Schema**: Validación estricta de deps.json
- ✅ **Validaciones de negocio**: Dependencias, ciclos, compatibilidad
- ✅ **Manejo de errores**: Mensajes claros con sugerencias
- ✅ **Locks**: Previene ejecuciones concurrentes
- ✅ **Estado local**: Guarda estado por proyecto en `.state.json`

### 4. Comandos Implementados
- ✅ `raioz up`: Levanta proyecto completo
- ✅ `raioz down`: Detiene proyecto
- ✅ `raioz status`: Muestra estado detallado
- ✅ `raioz logs`: Logs de servicios
- ✅ `raioz clean`: Limpia recursos no usados
- ✅ `raioz check`: Verifica alineación
- ✅ `raioz version`: Versión del binario
- ✅ `raioz ci`: Comando para CI/CD

## ⚠️ Lo que está MAL o INCOMPLETO

### 1. Convención de Nombres de Contenedores
**Problema**: No se aplica convención estricta `raioz-{project}-{service}`

**Estado actual**:
- Docker Compose genera nombres automáticos (formato `{project}-{service}-1`)
- No hay `container_name` explícito en compose generado
- No hay normalización de nombres
- Dificulta debugging manual

**Referencia**: escenario.md línea 148-149 menciona `raioz-billing-users`, `raioz-billing-payments`

**Impacto**: ALTO - Ahorra horas de debugging según usuario

### 2. Estado Global
**Problema**: No existe estado global `~/.raioz/state.json`

**Estado actual**:
- Solo existe estado por proyecto (`workspaces/{project}/.state.json`)
- No hay forma de listar proyectos activos
- No hay comando `raioz list`
- No se guarda información global (última ejecución, proyectos activos)

**Referencia**: Usuario solicitó explícitamente estado local mínimo

**Impacto**: MEDIO - Importante para consistencia y debugging

### 3. Documento de Límites
**Problema**: No existe `/docs/limits.md`

**Estado actual**:
- No hay documentación clara de qué NO hace Raioz
- No hay casos no soportados documentados
- No hay decisiones conscientes explicadas
- Puede generar frustración futura

**Referencia**: Usuario solicitó explícitamente documento de límites

**Impacto**: ALTO - Evita frustración futura

### 4. Inconsistencias Menores
- **Workspace path**: Código usa `/opt/raioz-proyecto/` pero escenario.md menciona `/opt/raioz/` (inconsistencia menor, no crítica)
- **Estructura de workspace**: Ya implementada correctamente con `local/` y `readonly/`

## ❌ Lo que FALTA según documentos

### 1. Comando `raioz list`
**Referencia**: caso-real.md, como-funciona.md (implícito en necesidad de listar proyectos)

**Funcionalidad esperada**:
- Listar proyectos activos
- Mostrar estado de cada proyecto
- Última ejecución
- Servicios activos

### 2. Normalización de Nombres
**Referencia**: escenario.md línea 148-149

**Funcionalidad esperada**:
- Función `NormalizeName(project, service)`
- Validación de nombres en configuración
- Aplicar `container_name` en docker-compose
- Formato: `raioz-{project}-{service}`

### 3. Documentación de Límites
**Referencia**: Usuario solicitó explícitamente

**Funcionalidad esperada**:
- `/docs/limits.md` con:
  - Qué Raioz NO hace
  - Casos no soportados
  - Decisiones conscientes

### 4. Estado Global
**Referencia**: Usuario solicitó explícitamente

**Funcionalidad esperada**:
- `~/.raioz/state.json` con:
  - Proyectos activos
  - Última ejecución
  - Servicios activos por proyecto
  - Modo, versión, estado

## 📊 Resumen de Gaps

| Funcionalidad | Estado | Prioridad | Referencia |
|--------------|--------|-----------|------------|
| Convención nombres contenedores | ❌ Falta | ALTA | escenario.md, usuario |
| Estado global | ❌ Falta | MEDIA | Usuario |
| Comando `raioz list` | ❌ Falta | MEDIA | Implícito en docs |
| Documento límites | ❌ Falta | ALTA | Usuario |
| Workspace path | ⚠️ Menor | BAJA | escenario.md (inconsistencia menor) |

## 🎯 Conclusión

El proyecto está **muy bien implementado** en las funcionalidades core. Las funcionalidades faltantes son principalmente:

1. **Convención de nombres** (ALTA prioridad) - Ya está en TODO pero no implementado
2. **Documento de límites** (ALTA prioridad) - Ya está en TODO pero no implementado
3. **Estado global** (MEDIA prioridad) - Ya está en TODO pero no implementado
4. **Comando list** (MEDIA prioridad) - No está en TODO, debería agregarse

Las tareas 29, 30, 31 del TODO ya cubren la mayoría de estos gaps. Solo falta agregar el comando `raioz list` como subtarea de la tarea 30.
