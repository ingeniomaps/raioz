# Caso de Uso 2: Levantar un Proyecto Completo

## 📋 Descripción

Un desarrollador necesita levantar todo el stack de un proyecto: servicios en desarrollo, servicios estables, e infraestructura compartida.

## 🎯 Objetivo

Levantar todos los servicios necesarios del proyecto con un solo comando, manejando automáticamente:
- Clonado de repositorios Git
- Pull de imágenes Docker
- Configuración de redes y volúmenes
- Variables de entorno
- Dependencias entre servicios

## 🔄 Flujo Completo

### Comando Principal

```bash
raioz up
```

### Qué Hace Internamente

#### 1. Validación Pre-vuelo

**Validaciones realizadas:**
- ✅ Docker está corriendo
- ✅ Git está instalado
- ✅ Permisos de escritura en workspace
- ✅ `.raioz.json` existe y es válido

**Si falla:**
- Muestra error claro con sugerencias
- No continúa hasta que se resuelva

#### 2. Carga y Validación de Configuración

**Lee `.raioz.json`:**
```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "billing-platform",
    "network": "billing-network"
  },
  "services": { ... },
  "infra": { ... }
}
```

**Validaciones:**
- ✅ Esquema JSON válido
- ✅ Nombres de proyecto/servicios válidos
- ✅ Puertos no conflictivos
- ✅ Dependencias válidas (sin ciclos)
- ✅ Ramas de Git existen
- ✅ Imágenes Docker accesibles

#### 3. Filtrado por Perfiles y Feature Flags

**Si se usa `--profile`:**
```bash
raioz up --profile frontend
```

**Qué hace:**
- Filtra servicios que tienen el perfil especificado
- Incluye servicios sin perfil (siempre incluidos)
- Excluye servicios con otros perfiles

**Si hay feature flags:**
- Evalúa variables de entorno
- Habilita/deshabilita servicios según configuración
- Reemplaza servicios con mocks si está configurado

#### 4. Resolución de Workspace

**Crea estructura:**
```
/opt/raioz-proyecto/
├── workspaces/
│   └── billing-platform/
│       ├── local/          # Servicios editables
│       ├── readonly/        # Servicios readonly
│       ├── .state.json      # Estado del proyecto
│       └── docker-compose.generated.yml
├── env/
│   ├── global.env
│   ├── projects/
│   └── services/
└── state.json               # Estado global
```

**Migración automática:**
- Si hay servicios en estructura antigua (`{base}/services/`), los migra automáticamente
- Mueve a `local/` o `readonly/` según `access`

#### 5. Clonado de Repositorios Git

**Para cada servicio con `source.kind: "git"`:**

**Servicios editables:**
- Clona en `{base}/workspaces/{project}/local/{path}`
- Hace checkout de la rama especificada
- Detecta drift de rama (cambios manuales)
- Hace pull si es necesario

**Servicios readonly:**
- Clona en `{base}/workspaces/{project}/readonly/{path}`
- Solo clona si no existe (no actualiza)
- No hace checkout ni pull automático
- Protegido de modificaciones

**Mensajes informativos:**
```
✔ users-service clonado (develop)
ℹ️  billing-service ya existe, verificando rama...
⚠️  Branch drift detectado en auth-service: expected 'main', found 'develop'
✔ auth-service clonado (readonly, protegido)
```

#### 6. Validación de Imágenes Docker

**Para cada servicio con `source.kind: "image"`:**
- Verifica que la imagen existe localmente o es accesible
- Hace `docker pull` si es necesario
- Valida que el tag existe

**Mensajes:**
```
✔ Verificando imagen org/payments-service:2.4.1
✔ Imagen encontrada
```

#### 7. Gestión de Recursos Docker

**Red:**
- Crea red Docker si no existe: `{project.network}`
- Verifica que no hay conflictos con otros proyectos

**Volúmenes:**
- Crea volúmenes nombrados si es necesario
- Valida que no están en uso por otros proyectos

**Puertos:**
- Valida que los puertos no están en uso
- Muestra conflictos si los hay

#### 8. Resolución de Variables de Entorno

**Lee archivos `.env` en orden de precedencia:**
1. `env/services/{service}.env` (mayor precedencia)
2. `env/projects/{project}.env`
3. `env/global.env` (menor precedencia)

**Genera archivo consolidado:**
- Crea `.env.{service-name}` en el workspace
- Combina todas las variables según precedencia
- Usado por Docker Compose

#### 9. Generación de Docker Compose

**Genera `docker-compose.generated.yml`:**

**Para cada servicio:**
- Agrega `container_name` normalizado: `raioz-{project}-{service}`
- Configura build context (para servicios Git)
- Configura imagen (para servicios image)
- Agrega puertos mapeados
- Agrega volúmenes (con `:ro` si es readonly)
- Agrega dependencias (`depends_on`)
- Agrega variables de entorno (`env_file`)
- Aplica modo dev/prod:
  - Dev: bind mounts, restart: no
  - Prod: sin bind mounts, restart: unless-stopped, healthchecks

**Para infra:**
- Similar a servicios pero sin build
- Solo imagen Docker

**Estructura generada:**
```yaml
version: "3.9"
services:
  users-service:
    container_name: raioz-billing-platform-users-service
    build:
      context: /opt/raioz-proyecto/workspaces/billing-platform/local/services/users
      dockerfile: Dockerfile.dev
    ports:
      - "3001:3000"
    volumes:
      - ./services/users:/app
    depends_on:
      - postgres
    networks:
      - billing-network
    env_file:
      - .env.users-service
    restart: "no"  # dev mode

  postgres:
    container_name: raioz-billing-platform-postgres
    image: postgres:15
    ports:
      - "5432:5432"
    volumes:
      - postgres-data:/var/lib/postgresql/data
    networks:
      - billing-network
    restart: "unless-stopped"
```

#### 10. Ejecución de Docker Compose

**Comando ejecutado:**
```bash
docker compose -f docker-compose.generated.yml up -d
```

**Qué hace:**
- Levanta todos los servicios en modo detached
- Espera a que los servicios estén corriendo
- Muestra logs de inicio

#### 11. Guardado de Estado

**Guarda estado local:**
- Archivo: `{base}/workspaces/{project}/.state.json`
- Contiene: configuración completa del proyecto

**Actualiza estado global:**
- Archivo: `{base}/state.json`
- Contiene: información de todos los proyectos activos
- Incluye: nombre, workspace, última ejecución, servicios activos

#### 12. Resumen Final

**Muestra:**
```
✔ Proyecto 'billing-platform' iniciado exitosamente

Resumen:
- Servicios: 4 (users-service, billing-service, auth-service, payments-service)
- Infra: 3 (postgres, redis, rabbit)
- Tiempo: 2m 15s
```

## 📊 Ejemplo Completo

### Configuración Inicial

**.raioz.json:**
```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "billing-platform",
    "network": "billing-network"
  },
  "services": {
    "api": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/api.git",
        "branch": "develop",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"],
        "dependsOn": ["database"]
      }
    },
    "worker": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/worker.git",
        "branch": "main",
        "path": "services/worker",
        "access": "readonly"
      },
      "docker": {
        "mode": "prod",
        "ports": ["3001:3000"],
        "dependsOn": ["database", "redis"]
      }
    }
  },
  "infra": {
    "database": {
      "image": "postgres",
      "tag": "15",
      "ports": ["5432:5432"],
      "volumes": ["postgres-data:/var/lib/postgresql/data"]
    },
    "redis": {
      "image": "redis",
      "tag": "7",
      "ports": ["6379:6379"]
    }
  }
}
```

### Ejecución

```bash
$ raioz up

✔ Validando configuración...
✔ Workspace creado: /opt/raioz-proyecto/workspaces/billing-platform
✔ Clonando api (develop)...
✔ api clonado
ℹ️  worker ya existe (readonly), saltando actualización
✔ Verificando imágenes Docker...
✔ postgres:15 encontrado
✔ redis:7 encontrado
✔ Creando red billing-network...
✔ Red creada
✔ Creando volúmenes...
✔ postgres-data creado
✔ Resolviendo variables de entorno...
✔ Generando docker-compose.generated.yml...
✔ Levantando servicios...
✔ Servicios iniciados
✔ Estado guardado

✔ Proyecto 'billing-platform' iniciado exitosamente

Resumen:
- Servicios: 2 (api, worker)
- Infra: 2 (database, redis)
- Tiempo: 1m 30s
```

### Verificación

```bash
$ raioz status

Project: billing-platform
Network: billing-network

Services:
api      running  healthy  1m  0.5% CPU  50MB  abc123def456
worker   running  healthy  1m  0.3% CPU  45MB  xyz789uvw012 (readonly)

Infra:
database  running  healthy  1m  0.1% CPU  30MB  postgres:15
redis     running  healthy  1m  0.1% CPU  25MB  redis:7
```

## 🔍 Detalles Técnicos

### Orden de Ejecución

1. **Validación** (no destructiva)
2. **Workspace** (creación de directorios)
3. **Git** (clonado, checkout)
4. **Docker** (imágenes, red, volúmenes)
5. **Env** (resolución de variables)
6. **Compose** (generación)
7. **Up** (levantamiento de servicios)
8. **Estado** (guardado)

### Idempotencia

**Raioz es idempotente:**
- Puedes ejecutar `raioz up` múltiples veces
- Si un servicio ya está corriendo, no lo reinicia
- Si un repo ya está clonado, verifica la rama
- Si un recurso ya existe, lo reutiliza

**Detección de cambios:**
- Compara estado actual con estado guardado
- Detecta cambios en configuración
- Actualiza solo lo necesario

### Manejo de Errores

**Errores no fatales:**
- Warnings sobre migración de servicios legacy
- Advertencias sobre volúmenes readonly con `:rw`
- Información sobre servicios disabled

**Errores fatales:**
- Configuración inválida
- Puertos en conflicto
- Imágenes Docker no encontradas
- Fallos al clonar repositorios
- Fallos al levantar servicios

## ⚠️ Casos Especiales

### Servicios Disabled

**Si un servicio tiene `enabled: false`:**
- No se clona
- No se incluye en docker-compose
- Se muestra en `raioz status` como "disabled"

### Feature Flags

**Si un servicio tiene feature flags:**
- Se evalúa según variables de entorno
- Puede ser reemplazado por un mock
- Puede ser excluido completamente

### Servicios Readonly

**Comportamiento especial:**
- No se actualizan automáticamente
- Volúmenes montados como `:ro`
- `restart: unless-stopped` (se recrean si fallan)
- Protegidos de modificaciones

## 📝 Checklist de Verificación

Después de `raioz up`, verifica:

- [ ] Todos los servicios están `running`
- [ ] Health status es `healthy` (en modo prod)
- [ ] Puertos están accesibles
- [ ] Logs no muestran errores críticos
- [ ] Dependencias están resueltas

## 🔗 Comandos Relacionados

- `raioz status`: Ver estado detallado
- `raioz logs`: Ver logs de servicios
- `raioz check`: Verificar alineación
- `raioz down`: Detener proyecto
- `raioz ports`: Ver puertos en uso
