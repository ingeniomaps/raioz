Raioz Local Orchestrator
¿Qué es?

Raioz Local Orchestrator es una herramienta interna que permite levantar, coordinar y mantener entornos de desarrollo local para proyectos basados en microservicios, a partir de una configuración declarativa.

Su propósito es que cualquier desarrollador pueda ejecutar un proyecto completo con un solo comando, sin necesidad de clonar todos los repositorios, sin configuraciones manuales repetitivas y sin conflictos entre servicios o proyectos.

¿Qué hace?

A partir de un archivo de dependencias (deps.json), el orquestador:

Crea y gestiona un workspace local compartido

Clona únicamente los microservicios necesarios, en la rama correcta

Usa imágenes Docker versionadas para servicios estables

Provisiona infraestructura local (DB, colas, cache) de forma consistente

Centraliza y valida variables de entorno

Genera automáticamente la configuración de Docker Compose

Levanta el entorno de forma idempotente (segura de ejecutar múltiples veces)

Evita conflictos de puertos, volúmenes y redes entre proyectos

Mantiene estado local para detectar desalineaciones o errores

Todo esto sin que el desarrollador tenga que entender o manipular directamente Docker, redes o dependencias cruzadas.

¿Por qué existe?

El orquestador existe porque los enfoques tradicionales no escalan cuando:

Hay múltiples microservicios compartidos entre proyectos

No todos los desarrolladores trabajan en los mismos servicios

Algunos servicios se desarrollan localmente y otros se consumen como imágenes

Las configuraciones de entorno empiezan a duplicarse y divergir

Aparecen errores tipo “funciona en mi máquina”

El onboarding se vuelve lento y frágil

Usar solo docker-compose por repositorio o pedir que todos clonen todo genera:

Entornos inconsistentes

Conflictos de puertos y volúmenes

Dificultad para cambiar de proyecto

Alto costo cognitivo para nuevos integrantes

El orquestador no añade complejidad: la expone, la ordena y la controla.

¿Por qué este enfoque y no scripts simples?

Aunque el problema puede resolverse inicialmente con scripts, el dominio real incluye:

Validaciones complejas

Manejo de estado

Diferentes fuentes de servicios (git vs imágenes)

Ramas, modos (dev/prod), dependencias explícitas

Re-ejecución segura

Evolución sin romper setups existentes

Esto deja de ser un “script” y se convierte en un sistema de orquestación local.

El orquestador proporciona:

Convenciones claras

Comportamiento predecible

Extensibilidad

Observabilidad del entorno

Objetivo

El objetivo principal es:

Eliminar la fricción entre desarrollo y arquitectura,
haciendo que trabajar con microservicios localmente sea tan simple como trabajar con un monolito.

¿A dónde se quiere llegar?
Corto plazo

Onboarding en un solo comando

Entornos reproducibles entre desarrolladores

Reducción drástica de errores de configuración

Claridad sobre qué servicios están activos y por qué

Mediano plazo

Soporte para perfiles de proyecto (backend / frontend / full)

Mocks y feature flags por servicio

Integración con CI para entornos efímeros

Validación de compatibilidad entre servicios

Largo plazo

Paridad real entre desarrollo y producción

Entornos locales como contratos declarativos

Base para previews locales, QA y testing distribuido

Plataforma interna de desarrollo, no solo una herramienta

En una frase

Raioz Local Orchestrator transforma la complejidad inevitable de los microservicios en una experiencia de desarrollo simple, consistente y controlada.
