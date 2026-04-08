# Mejores Prácticas: Proyectos Gateway/Nginx con Configuración Compleja

## Caso de Estudio: Gateway Nginx/OpenResty

Este documento analiza el mejor enfoque para proyectos gateway complejos con:
- ✅ Configuración extensa de nginx/openresty
- ✅ Scripts de instalación/preparación (`installer.sh`)
- ✅ Generación de configuraciones desde templates
- ✅ Manejo de SSL/certificados
- ✅ `docker-compose.yml` ya existente y funcional

---

## 📊 Análisis del Proyecto

### Estructura Actual

```
gateway/
├── installer.sh              # Script de instalación/preparación
├── docker/
│   └── docker-compose.yml   # Ya existe y funciona
├── scripts/
│   ├── generate-configs.sh  # Genera configs desde templates
│   ├── init-ssl.sh          # Maneja certificados SSL
│   └── ...
├── conf/                     # Configuración extensa de nginx
│   ├── config/
│   ├── services/
│   ├── templates/
│   └── ...
├── lua/                      # Scripts Lua personalizados
├── nginx.conf               # Configuración principal
└── .env                     # Variables de entorno
```

### Características Especiales

1. **Instalación previa requerida**:
   - `installer.sh` debe ejecutarse antes de `docker compose up`
   - Genera configuraciones desde templates
   - Crea directorios necesarios
   - Prepara entorno SSL

2. **docker-compose.yml complejo**:
   - Múltiples volúmenes mapeados
   - Configuración de red externa
   - IP estática
   - Healthcheck personalizado
   - Servicio certbot adicional

3. **Configuración dinámica**:
   - Templates que se generan según variables de entorno
   - Certificados SSL que se generan/renuevan
   - Configuraciones que cambian según modo (local/prod)

---

## 🎯 Opciones Disponibles con Raioz

### Opción 1: `project.commands` (RECOMENDADO para tu caso)

**Configuración**:

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "nginx",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    },
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose down",
      "health": "cd docker && docker compose ps nginx | grep -q 'Up'"
    },
    "env": ["."]
  },
  "env": {
    "useGlobal": false,
    "files": [".env"]
  }
}
```

**¿Cómo funciona?**:

1. **`raioz up`**:
   - Ejecuta `bash installer.sh --deploy`
   - El `installer.sh`:
     - Genera configuraciones
     - Ejecuta `docker compose up -d`
     - Genera certificados SSL (si `--ssl`)
   - Raioz detecta automáticamente el `docker-compose.yml`
   - Los servicios aparecen en `raioz status` y `raioz logs`

2. **`raioz down`**:
   - Ejecuta `cd docker && docker compose down`
   - Detiene los servicios correctamente

**✅ Ventajas**:
- ✅ No duplica configuración (usa tu `docker-compose.yml` existente)
- ✅ Mantiene tu lógica de instalación (`installer.sh`)
- ✅ Compatible con tus scripts personalizados
- ✅ Funciona con Raioz (`status`, `logs`, etc.)
- ✅ No necesitas Dockerfile
- ✅ Más simple y mantenible

**❌ Desventajas**:
- ⚠️ No aparece en el `docker-compose.generated.yml` de Raioz
- ⚠️ Debes mantener el `docker-compose.yml` manualmente

---

### Opción 2: Migrar a `docker: {}` con Dockerfile

**Configuración**:

```json
{
  "services": {
    "nginx": {
      "source": {
        "kind": "local",
        "path": "."
      },
      "docker": {
        "mode": "dev",
        "ports": ["80:80", "443:443"],
        "volumes": [
          "./scripts:/code/scripts:rw",
          "./templates:/code/templates:ro",
          "./conf:/etc/nginx/conf.d:rw",
          "./lua:/code/lua:rw",
          "./nginx.conf:/usr/local/openresty/nginx/conf/nginx.conf:ro",
          "./logs:/usr/local/openresty/nginx/logs:rw",
          "./ssl:/etc/ssl/private:ro",
          "./certbot/www:/var/www/certbot:ro",
          "./certbot/conf:/etc/letsencrypt:ro"
        ],
        "healthcheck": {
          "test": ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost/health"],
          "interval": "30s",
          "timeout": "5s",
          "retries": 3,
          "start_period": "10s"
        },
        "ip": "192.160.1.10"
      },
      "env": ["."]
    }
  }
}
```

**Dockerfile.dev**:

```dockerfile
FROM openresty/openresty:alpine

WORKDIR /usr/local/openresty/nginx

# Copiar configuración
COPY nginx.conf /usr/local/openresty/nginx/conf/nginx.conf
COPY conf/ /etc/nginx/conf.d/
COPY lua/ /code/lua/
COPY scripts/ /code/scripts/
COPY templates/ /code/templates/

# Crear directorios necesarios
RUN mkdir -p /usr/local/openresty/nginx/logs \
    /etc/ssl/private \
    /var/www/certbot \
    /etc/letsencrypt

# Comando de inicio
CMD ["nginx", "-g", "daemon off;"]
```

**❌ Problemas**:
- ❌ Duplica toda la configuración del `docker-compose.yml`
- ❌ Pierdes la lógica de `installer.sh` (generación de configs, SSL, etc.)
- ❌ Debes ejecutar `installer.sh` manualmente antes de cada `raioz up`
- ❌ No maneja la generación de configuraciones desde templates
- ❌ No genera certificados SSL automáticamente
- ❌ Más verboso y propenso a errores

---

### Opción 3: Híbrido - Dockerfile + `project.commands.up`

**Configuración**:

```json
{
  "project": {
    "name": "nginx",
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose down"
    },
    "env": ["."]
  },
  "services": {
    "nginx": {
      "source": {
        "kind": "local",
        "path": "."
      },
      "docker": {
        "mode": "dev",
        "ports": ["80:80", "443:443"],
        // ... configuración simplificada
      }
    }
  }
}
```

**❌ Problemas**:
- ❌ Confusión: ¿cuál se ejecuta?
- ❌ Duplicación de lógica
- ❌ No recomendado

---

## 🏆 Recomendación Final

### ✅ **Opción 1: `project.commands` (Recomendado)**

Para tu proyecto gateway, la **mejor práctica es usar `project.commands`** porque:

1. **Tu proyecto ya tiene `docker-compose.yml` completo y funcional**
   - No necesitas duplicarlo
   - Ya está probado y funciona

2. **Tienes lógica compleja de instalación (`installer.sh`)**
   - Genera configuraciones desde templates
   - Maneja certificados SSL
   - Prepara el entorno antes de docker compose

3. **El `docker-compose.yml` es extenso y específico**
   - Muchos volúmenes mapeados
   - Configuración personalizada de red/IP
   - Healthcheck personalizado
   - Duplicarlo sería error-prone

### Configuración Recomendada

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "nginx",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    },
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose -f docker/docker-compose.yml down",
      "health": "cd docker && docker compose -f docker/docker-compose.yml ps nginx | grep -q 'Up' && docker compose -f docker/docker-compose.yml exec nginx wget --quiet --tries=1 --spider http://localhost/health"
    },
    "env": ["."]
  },
  "env": {
    "useGlobal": false,
    "files": [".env"]
  }
}
```

### Modificaciones Sugeridas al `installer.sh`

Puedes hacer que `installer.sh` sea más amigable para Raioz:

```bash
#!/bin/bash
# installer.sh --deploy [--ssl]

# ... tu lógica existente ...

# Si se ejecuta desde Raioz, usar flags específicos
if [ -n "$RAIOZ_MODE" ]; then
    # Modo Raioz: asegurar que siempre regenera configs
    INSTALLER_MODE=true ./scripts/generate-configs.sh
fi

# Desplegar usando docker-compose
cd docker
docker compose -f docker-compose.yml up -d nginx

# ... resto de la lógica ...
```

---

## 🔄 Flujo de Trabajo Recomendado

### Desarrollo Local

```bash
# 1. Configurar (primera vez)
cd /home/manuel/Code/roax/raioz/gateway
cp .env-template .env
# Editar .env

# 2. Usar Raioz
raioz up     # Ejecuta installer.sh --deploy automáticamente
raioz status # Ve el estado
raioz logs   # Ve los logs
raioz down   # Detiene el gateway
```

### Cambios en Configuración

```bash
# Editas conf/ o templates/
vim conf/services/app.conf

# Regenerar configuraciones
./scripts/generate-configs.sh

# Recargar nginx (sin reiniciar)
docker compose -f docker/docker-compose.yml exec nginx nginx -s reload

# O reiniciar completamente
raioz down && raioz up
```

### Generar/Actualizar SSL

```bash
# Generar certificados iniciales
./scripts/init-ssl.sh

# Renovar certificados (desde crontab)
./scripts/renew-ssl.sh
```

---

## 🆚 Comparación: ¿Cuándo Usar Cada Enfoque?

| Característica | `project.commands` | `docker: {}` con Dockerfile |
|---------------|-------------------|---------------------------|
| **docker-compose.yml existente** | ✅ Ideal | ❌ Duplicas todo |
| **Lógica compleja de instalación** | ✅ Mantiene scripts | ❌ Pierdes lógica |
| **Configuraciones desde templates** | ✅ Funciona igual | ❌ Debes ejecutar manualmente |
| **SSL/Certificados automáticos** | ✅ Funciona igual | ❌ Debes ejecutar manualmente |
| **Configuración simple** | ⚠️ Puede ser overkill | ✅ Ideal |
| **Menos archivos a mantener** | ✅ Solo `.raioz.json` | ❌ `.raioz.json` + `Dockerfile.dev` |
| **Mantenibilidad** | ✅ Alta | ⚠️ Media (duplicación) |

---

## 📝 Ejemplo Completo: Configuración Final Recomendada

**`.raioz.json`**:

```json
{
  "schemaVersion": "1.0",
  "workspace": "roax",
  "project": {
    "name": "nginx",
    "network": {
      "name": "roax",
      "subnet": "192.160.0.0/16"
    },
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose -f docker/docker-compose.yml down",
      "health": "docker compose -f docker/docker-compose.yml exec nginx wget --quiet --tries=1 --spider http://localhost/health || exit 1"
    },
    "env": ["."]
  },
  "env": {
    "useGlobal": false,
    "files": [".env"]
  }
}
```

**`installer.sh`** (modificado para Raioz):

```bash
#!/bin/bash
# ... código existente ...

# Si se ejecuta desde Raioz
if [ -n "$RAIOZ_MODE" ]; then
    echo "Ejecutando en modo Raioz..."
    # Regenerar configuraciones siempre
    INSTALLER_MODE=true ./scripts/generate-configs.sh
fi

# ... resto del código ...

# Desplegar
cd docker
docker compose -f docker-compose.yml up -d nginx
```

---

## ✅ Ventajas del Enfoque Recomendado

1. **Simplicidad**: Un solo archivo de configuración (`.raioz.json`)
2. **Mantenibilidad**: No duplicas configuración Docker
3. **Flexibilidad**: Mantienes tu lógica compleja de instalación
4. **Integración**: Funciona con Raioz (`status`, `logs`, etc.)
5. **Portabilidad**: Otros desarrolladores pueden usar `docker compose` directamente si prefieren
6. **Evolución**: Si cambias `docker-compose.yml`, Raioz lo detecta automáticamente

---

## 🚀 Próximos Pasos

1. ✅ Crear `.raioz.json` con `project.commands`
2. ✅ Asegurar que `installer.sh` funciona correctamente
3. ✅ Probar `raioz up` y verificar que todo funciona
4. ✅ Verificar que servicios aparecen en `raioz status`
5. ✅ Probar `raioz logs` para ver logs

---

## Referencias

- [Cómo Funciona Docker](./COMO_FUNCIONA_DOCKER.md)
- [Tipos de Servicios](./TIPOS_DE_SERVICIOS.md)
- [Ejemplos de Conflictos](./ejemplos/conflicto-servicios.md)
