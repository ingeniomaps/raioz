# Límites y Decisiones de Diseño - Raioz

Este documento establece claramente qué **NO** hace Raioz, qué casos no soporta, y las decisiones conscientes de diseño que se tomaron. Esto ayuda a establecer expectativas claras y evitar frustración futura.

## 🚫 Qué Raioz NO hace

### Orquestación y Contenedores

- **No gestiona Kubernetes**: Raioz está diseñado exclusivamente para Docker Compose. No genera manifiestos de Kubernetes, no gestiona pods, services, o deployments. Si necesitas Kubernetes, deberás convertir manualmente la configuración generada.

- **No soporta Docker Swarm**: Solo Docker Compose está soportado. Docker Swarm requiere una configuración diferente y no está en el alcance del proyecto.

- **No gestiona múltiples entornos simultáneos**: Raioz está diseñado para un entorno local a la vez. No puedes tener dev/staging/prod corriendo simultáneamente en la misma máquina con diferentes configuraciones. Cada proyecto tiene un único estado.

### Gestión de Datos y Seguridad

- **No gestiona secrets de forma nativa**: Raioz no tiene un sistema de gestión de secrets. Usa archivos `.env` estándar para variables de entorno. Los secrets deben gestionarse manualmente o con herramientas externas (como `pass`, `sops`, o servicios de gestión de secrets).

- **No hace backup automático de datos**: Raioz no realiza backups automáticos de volúmenes Docker, bases de datos, o datos persistentes. Es responsabilidad del desarrollador hacer backups si es necesario.

- **No gestiona permisos de archivos avanzados**: Los permisos de archivos se establecen con valores por defecto (0755 para directorios, 0644 para archivos). No hay gestión granular de permisos.

### CI/CD y Automatización

- **No gestiona orquestación de CI/CD**: Raioz puede validar configuraciones (`raioz ci`) pero no ejecuta pipelines completos de CI/CD. No reemplaza herramientas como GitHub Actions, GitLab CI, o Jenkins.

- **No despliega a producción**: Raioz es exclusivamente para desarrollo local. No tiene capacidades de despliegue a servidores remotos, cloud, o producción.

### Networking y Infraestructura

- **No gestiona DNS local**: Raioz no configura DNS local ni resolución de nombres personalizada. Usa las redes Docker estándar y la resolución de nombres de Docker Compose.

- **No gestiona balanceadores de carga**: No hay soporte para balanceadores de carga, reverse proxies complejos, o configuración avanzada de networking.

- **No gestiona certificados SSL/TLS**: Los certificados SSL/TLS deben gestionarse manualmente si es necesario. Raioz no genera ni renueva certificados automáticamente.

## ❌ Casos No Soportados

### Contenedores y Privilegios

- **Servicios que requieren privilegios especiales (`--privileged`)**: Raioz no soporta servicios que requieren el flag `--privileged` de Docker. Esto incluye servicios que necesitan acceso directo al hardware o capacidades del kernel que requieren privilegios elevados.

- **Servicios que requieren dispositivos específicos**: No se soportan servicios que requieren acceso a dispositivos específicos del sistema (como `/dev/ttyUSB0`, dispositivos de audio, etc.). Estos casos requieren configuración manual de Docker.

- **Servicios Windows containers**: Raioz está diseñado exclusivamente para contenedores Linux. No soporta Windows containers ni contenedores multi-plataforma que requieran Windows.

### Configuración Avanzada de Docker

- **Servicios con múltiples redes personalizadas**: Raioz crea una red por proyecto. No soporta servicios que necesiten estar en múltiples redes personalizadas simultáneamente.

- **Servicios con healthchecks complejos personalizados**: Aunque Raioz genera healthchecks básicos para modo `prod`, no soporta healthchecks complejos personalizados que requieran scripts, comandos complejos, o lógica avanzada.

- **Servicios con configuraciones de recursos personalizadas**: No se pueden especificar límites de CPU, memoria, o I/O personalizados en `.raioz.json`. Estos deben configurarse manualmente en el `docker-compose.generated.yml` si es necesario.

- **Servicios con volúmenes con drivers personalizados**: Solo se soportan volúmenes con el driver por defecto de Docker. No se soportan drivers personalizados (como NFS, CIFS, etc.).

### Arquitectura y Escalabilidad

- **Escalado horizontal de servicios**: Raioz no gestiona múltiples instancias del mismo servicio. Cada servicio se ejecuta como una sola instancia.

- **Servicios con dependencias circulares complejas**: Aunque Raioz detecta dependencias circulares básicas, no maneja casos complejos donde múltiples servicios dependen entre sí de forma circular con condiciones.

- **Servicios con actualizaciones en caliente (hot-swap)**: No hay soporte para actualizar servicios sin downtime. Para actualizar un servicio, debe detenerse y reiniciarse.

## 🎯 Decisiones Conscientes de Diseño

### Por qué no se usa docker-compose override files

**Decisión**: Raioz genera `docker-compose.generated.yml` directamente en lugar de usar archivos `docker-compose.override.yml`.

**Razones**:
1. **Simplicidad**: Un solo archivo generado es más fácil de entender y depurar que múltiples archivos que se combinan.
2. **Transparencia**: El desarrollador puede ver exactamente qué configuración se está usando sin tener que entender cómo Docker Compose combina archivos.
3. **Control**: Evita conflictos y comportamientos inesperados cuando se combinan múltiples archivos.
4. **Idempotencia**: Cada ejecución genera el mismo archivo desde la misma configuración, sin depender de archivos externos que puedan cambiar.

**Alternativa**: Si necesitas personalizar la configuración, puedes editar `docker-compose.generated.yml` manualmente, pero los cambios se perderán en la próxima ejecución de `raioz up`. Para cambios permanentes, modifica `.raioz.json` o considera extender Raioz.

### Por qué se genera docker-compose.generated.yml (no se edita directamente)

**Decisión**: El archivo `docker-compose.generated.yml` es generado automáticamente y no debe editarse manualmente de forma permanente.

**Razones**:
1. **Fuente única de verdad**: `.raioz.json` es la única fuente de verdad. Si se permite editar el compose directamente, se crean dos fuentes de verdad que pueden divergir.
2. **Consistencia**: Todos los desarrolladores obtienen la misma configuración desde `.raioz.json`, evitando diferencias entre entornos.
3. **Versionado**: `.raioz.json` se versiona en Git, permitiendo revisar cambios de configuración en PRs.
4. **Regeneración**: Cada `raioz up` regenera el archivo, asegurando que refleja la configuración actual.

**Alternativa**: Para cambios temporales, puedes editar `docker-compose.generated.yml` manualmente, pero recuerda que se regenerará en la próxima ejecución. Para cambios permanentes, actualiza `.raioz.json`.

### Por qué se usa estructura de workspace específica

**Decisión**: Raioz usa una estructura de directorios específica: `{base}/workspaces/{project}/local/` y `{base}/workspaces/{project}/readonly/`.

**Razones**:
1. **Separación de concerns**: Los servicios editables (que desarrollas) están separados de los readonly (que solo consumes), evitando modificaciones accidentales.
2. **Aislamiento por proyecto**: Cada proyecto tiene su propio workspace, evitando conflictos entre proyectos.
3. **Migración automática**: La estructura permite migrar servicios legacy automáticamente sin romper configuraciones existentes.
4. **Claridad**: La estructura hace explícito qué servicios son editables y cuáles son de solo lectura.

**Alternativa**: Podrías usar una estructura plana, pero perderías la separación entre servicios editables y readonly, y la capacidad de migración automática.

### Por qué no se gestionan secrets de forma nativa

**Decisión**: Raioz no tiene un sistema de gestión de secrets integrado. Usa archivos `.env` estándar.

**Razones**:
1. **Simplicidad**: Los archivos `.env` son estándar, familiares para todos los desarrolladores, y funcionan con cualquier herramienta.
2. **Flexibilidad**: Permite usar cualquier herramienta de gestión de secrets (pass, sops, vault, etc.) sin acoplar Raioz a una solución específica.
3. **Portabilidad**: Los archivos `.env` funcionan igual en todos los sistemas operativos y entornos.
4. **Menos dependencias**: No requiere integraciones con servicios externos o herramientas adicionales.

**Alternativa**: Podrías integrar un sistema de secrets, pero agregaría complejidad y dependencias. Los archivos `.env` con herramientas externas proporcionan la misma funcionalidad con mayor flexibilidad.

### Por qué se prioriza simplicidad sobre flexibilidad

**Decisión**: Raioz prioriza la simplicidad y la facilidad de uso sobre la flexibilidad máxima.

**Razones**:
1. **Onboarding rápido**: Un desarrollador nuevo puede empezar en minutos, no horas.
2. **Menos errores**: Menos opciones significa menos formas de configurar algo incorrectamente.
3. **Mantenibilidad**: Código más simple es más fácil de mantener y extender.
4. **Casos de uso claros**: Al enfocarse en casos de uso comunes, se evita la complejidad de casos edge.

**Alternativa**: Podrías hacer Raioz más flexible, pero aumentaría la complejidad, el tiempo de onboarding, y la probabilidad de errores. La simplicidad es una característica, no una limitación.

### Por qué no se soporta Kubernetes

**Decisión**: Raioz está diseñado exclusivamente para Docker Compose, no para Kubernetes.

**Razones**:
1. **Alcance diferente**: Kubernetes es para producción y orquestación a escala. Raioz es para desarrollo local.
2. **Complejidad**: Kubernetes agrega complejidad significativa (pods, services, deployments, ingress, etc.) que no es necesaria para desarrollo local.
3. **Recursos**: Kubernetes requiere más recursos (CPU, memoria) que Docker Compose, lo cual es problemático para desarrollo local.
4. **Tiempo de desarrollo**: Agregar soporte para Kubernetes duplicaría o triplicaría el tiempo de desarrollo y mantenimiento.

**Alternativa**: Para producción con Kubernetes, puedes convertir manualmente la configuración de `.raioz.json` a manifiestos de Kubernetes, o usar herramientas de conversión.

### Por qué se usa un solo archivo de configuración (.raioz.json)

**Decisión**: Raioz usa un solo archivo `.raioz.json` para toda la configuración del proyecto.

**Razones**:
1. **Simplicidad**: Un solo archivo es más fácil de entender, versionar y compartir.
2. **Portabilidad**: Puedes copiar un solo archivo para reproducir un entorno completo.
3. **Versionado**: Un solo archivo es más fácil de revisar en PRs y trackear cambios.
4. **Consistencia**: Todos los servicios e infraestructura están definidos en un solo lugar.

**Alternativa**: Podrías usar múltiples archivos (uno por servicio, uno para infra, etc.), pero aumentaría la complejidad y haría más difícil mantener la consistencia.

## 🔄 Alternativas y Workarounds

### Si necesitas Kubernetes

1. **Conversión manual**: Convierte `.raioz.json` a manifiestos de Kubernetes manualmente.
2. **Herramientas de conversión**: Usa herramientas como `kompose` para convertir Docker Compose a Kubernetes (aunque puede requerir ajustes manuales).
3. **Doble configuración**: Mantén `.raioz.json` para desarrollo local y manifiestos de Kubernetes separados para producción.

### Si necesitas gestión de secrets avanzada

1. **Herramientas externas**: Usa `pass`, `sops`, `vault`, o similar para gestionar secrets y generar archivos `.env` automáticamente.
2. **Scripts de pre-procesamiento**: Crea scripts que generen archivos `.env` desde tu sistema de secrets antes de ejecutar `raioz up`.
3. **Integración con CI/CD**: En CI/CD, genera archivos `.env` desde tu sistema de secrets antes de ejecutar validaciones.

### Si necesitas configuración avanzada de Docker

1. **Edición manual temporal**: Edita `docker-compose.generated.yml` manualmente para cambios temporales.
2. **Extensión de Raioz**: Considera extender Raioz para agregar soporte para tu caso de uso específico.
3. **Docker Compose directo**: Para casos muy específicos, puedes usar `docker compose` directamente en lugar de Raioz.

### Si necesitas múltiples entornos simultáneos

1. **Múltiples proyectos**: Crea proyectos separados en Raioz para cada entorno (dev, staging, prod).
2. **Máquinas virtuales**: Usa VMs o contenedores para aislar entornos completamente.
3. **Docker Compose directo**: Para casos complejos, usa Docker Compose directamente con diferentes archivos de configuración.

## 📝 Notas Finales

Este documento refleja las decisiones de diseño actuales de Raioz. Si encuentras un caso de uso que no está cubierto y crees que debería estarlo, considera:

1. **¿Es un caso de uso común?**: Si es un caso edge, puede no valer la pena agregar complejidad.
2. **¿Puede resolverse con workarounds?**: A veces, una solución simple externa es mejor que agregar complejidad.
3. **¿Contribuye a la simplicidad?**: Si agregar la funcionalidad hace Raioz más complejo, puede no ser el camino correcto.

Raioz está diseñado para ser **simple, predecible y fácil de usar**. Estas limitaciones son características, no bugs.
