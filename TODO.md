# TODO - Raioz Local Orchestrator

Análisis del proyecto y plan de acción para llegar al 100% de funcionalidad.

## 📊 Estado Actual

- **Completitud funcional:** ~97%
- **Core básico:** ✅ 100%
- **Funcionalidades críticas:** ✅ 100%
- **Experiencia de usuario:** ✅ 100%
- **Robustez/producción:** ~88%

---

## 🎯 Tareas Pendientes para Alcanzar 100%

### Robustez/Producción (~88% → 100%)

**Objetivo:** Hacer el sistema robusto y listo para producción

- [ ] **52.9** Aumentar Cobertura de Tests a 90%+

  - [ ] Continuar agregando tests para paquetes con baja cobertura
  - [ ] Aumentar cobertura de `internal/state` (actualmente 37.1%)
  - [ ] Aumentar cobertura de `internal/workspace` (actualmente 37.1%)
  - [ ] Aumentar cobertura de otros paquetes según necesidad

- [ ] **52.13** Completar Migración de Dependency Injection
  - [ ] Completar migración de `cmd/up.go` (reemplazar todas las referencias directas a paquetes)
  - [ ] Migrar `cmd/down.go` a usar DI
  - [ ] Migrar otros comandos principales a usar DI
  - [ ] Actualizar tests para usar mocks
  - [ ] Verificar que todas las dependencias están inyectadas

### Completitud Funcional (~97% → 100%)

**Objetivo:** Completar todas las funcionalidades planificadas

- [x] **52.15** Completar Migración Arquitectónica ✅ (Parcial - Comandos principales migrados)

  - [x] Migrar comandos principales a capa de aplicación ✅
    - [x] Crear casos de uso en `internal/app/` para `down` y `status` ✅
    - [x] Actualizar `cmd/down.go` para usar `DownUseCase` ✅
    - [x] Actualizar `cmd/status.go` para usar `StatusUseCase` ✅
  - [ ] Migrar comandos restantes (list, ports, logs, clean, etc.)
  - [x] Separar completamente dominio de infraestructura ✅
    - [x] Definido tipo alias `Workspace` en dominio para evitar dependencias directas ✅
    - [x] Expandidas interfaces para incluir todos los métodos necesarios ✅
    - [x] Actualizado `internal/app` para usar solo interfaces, no tipos concretos ✅
    - [x] Eliminadas dependencias directas a `docker`, `workspace`, `state` desde `app` ✅
    - [x] Todas las operaciones ahora pasan por interfaces del dominio ✅
  - [x] Verificar que arquitectura está completa ✅
    - [x] Verificada separación de capas (domain, app, infra, cmd) ✅
    - [x] Verificadas dependencias entre capas ✅
    - [x] Verificadas interfaces e implementaciones ✅
    - [x] Verificados principios SOLID ✅
    - [x] Creado documento de verificación (`docs/ARCHITECTURE_VERIFICATION.md`) ✅
    - [x] Identificadas áreas de mejora (migración de comandos pendientes) ✅

- [x] **52.16** Funcionalidades Opcionales Pendientes ✅
  - [x] Evaluar y completar funcionalidades opcionales de FASE 7 ✅
    - [x] Evaluación completada - 7/9 funcionalidades implementadas (78%) ✅
    - [x] Creado documento de evaluación (`docs/FASE7_EVALUATION.md`) ✅
    - [x] Identificadas mejoras opcionales de baja prioridad ✅
  - [x] Implementar mejoras menores identificadas ✅
    - [x] Mejorar comando `raioz list` con filtros y mejor visualización ✅
      - [x] Agregado filtro `--filter` para buscar por nombre de proyecto ✅
      - [x] Agregado filtro `--status` para filtrar por estado de servicios ✅
      - [x] Mejora en visualización de servicios (muestra nombres cuando hay pocos) ✅
      - [x] Mensajes mejorados para filtros ✅
      - [x] Documentación actualizada ✅
    - [ ] Implementar Stub/Missing Mode solo si hay demanda específica (muy baja prioridad - opcional)

---

## 📋 Tareas Menores Pendientes

### 20.2 Paridad con Producción (futuro)

- [ ] Convertir Kubernetes configs a formato .raioz.json (futuro)

---

## 📝 Notas

- Este documento contiene solo las tareas pendientes
- Las funcionalidades opcionales pueden implementarse según necesidades
- Las tareas de arquitectura son mejoras a largo plazo, considerar Strangler Pattern
- Priorizar funcionalidades core antes de avanzar a funcionalidades avanzadas
- Considerar retrocompatibilidad al implementar cambios

---

## ✅ Funcionalidades Completadas (referencia histórica)

Las siguientes funcionalidades ya están implementadas y funcionando:

- Variables de entorno centralizadas
- Gestión de redes Docker
- Gestión de volúmenes
- Dependencias (dependsOn)
- Detección de conflictos de puertos
- Fallback de workspace para permisos
- Validación y manejo de Dockerfile.dev
- Verificación de imágenes Docker
- Idempotencia mejorada
- Actualización de repos Git (incluye detección de drift)
- Modo dev/prod
- Comando `raioz logs`
- Status mejorado
- Comando `raioz clean`
- Detección de desalineaciones
- Validación de compatibilidad
- README completo
- Comando `raioz version`
- Mejora de output y UX
- Mensajes de error claros
- Testing completo
- Campos no usados (deprecados)
- Mocks y feature flags
- Integración con CI
- Comparación y migración de configs de producción
- Modo readonly para repositorios Git (punto 26)
- Modo disabled para servicios (punto 27)
- Estructura de workspace mejorada (punto 28)
- Documento de límites (punto 29)
- Estado local mínimo (punto 30)
- Convención estricta de nombres (punto 31)
- Sistema de Override Explícito (punto 32)
- Resolución Asistida de Dependencias parcial (punto 33)
- Archivo raioz.root.json (punto 34)
- Comando Workspace (punto 35)
- Audit Log (punto 36)
- Sistema de Ignore (punto 37)
- Comando Link (punto 38)
- Validación de Inputs para prevenir command injection (punto 39)
- Prevención de Path Traversal (punto 40)
- Permisos de Archivos Seguros (punto 41)
- Sanitización de Secrets en Logs (punto 42)
- Generación Automática de Mocks (punto 43)
- CI/CD Pipeline (punto 44)
- Code Coverage en CI (punto 45)
- Dependency Management Automatizado (punto 46)
- Security Scanning Avanzado (punto 47)
- Logging Estructurado - Infraestructura y Migración (punto 48, 52.3)
- Context para Timeouts - Funciones principales completadas (punto 49, 52.2)
- Dependency Injection - Infraestructura (punto 50, 52.13 - parcial)
- Separación de Capas Arquitectónicas - Infraestructura (punto 51)
- Detección de Drift Posterior - Visualización e Integración (punto 33.5.2, 33.5.3, 52.5, 52.14)
- Advertencia sobre Volúmenes Compartidos (punto 52.1)
- Mejora de Manejo de Errores Críticos (punto 52.4)
- Mejora de Mensajes de Error y Feedback (punto 52.6)
- Mejora de Output y Formato (punto 52.7)
- Documentación de Comandos (punto 52.8)
- Mejora de Validaciones (punto 52.10)
- Logging Estructurado Completo (punto 52.11)
- Mejora de Resiliencia con Retry Logic y Circuit Breakers (punto 52.12)
