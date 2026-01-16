# Ejemplos de Configuración Raioz

Esta carpeta contiene ejemplos de configuración `.raioz.json` para diferentes tipos de proyectos. Cada ejemplo es funcional y demuestra diferentes casos de uso de Raioz.

## 📁 Ejemplos Disponibles

### 01-basic-web-app
Aplicación web básica con frontend, API y worker en Node.js, con PostgreSQL y Redis.

**Servicios:**
- `frontend` - Frontend Next.js Hello World (puerto 3000) - [vercel/next.js](https://github.com/vercel/next.js)
- `api` - JSON Server API (puerto 3001) - [typicode/json-server](https://github.com/typicode/json-server)
- `worker` - Bull Queue Worker Example (puerto 3002) - [OptimalBits/bull](https://github.com/OptimalBits/bull)

**Infraestructura:**
- PostgreSQL 15 (puerto 5432)
- Redis 7 (puerto 6379)

**Uso:**
```bash
cd examples/01-basic-web-app
raioz up
```

---

### 02-multi-language-stack
Stack multi-lenguaje con servicios en Go, Node.js, Python y Rust.

**Servicios:**
- `go-api` - API REST Gin RealWorld (puerto 8080) - [gin-gonic/examples](https://github.com/gin-gonic/examples)
- `node-service` - JSON Server (puerto 3000) - [typicode/json-server](https://github.com/typicode/json-server)
- `python-worker` - FastAPI Worker (puerto 8000) - [tiangolo/fastapi](https://github.com/tiangolo/fastapi)
- `rust-processor` - Actix Web Basics (puerto 8081) - [actix/examples](https://github.com/actix/examples)

**Infraestructura:**
- PostgreSQL 15 (puerto 5432)
- Redis 7 (puerto 6379)
- RabbitMQ 3 (puertos 5672, 15672)

**Uso:**
```bash
cd examples/02-multi-language-stack
raioz up
```

---

### 03-ecommerce-platform
Plataforma de e-commerce completa con múltiples microservicios.

**Servicios:**
- `frontend` - Vercel Commerce Frontend (puerto 3000) - [vercel/commerce](https://github.com/vercel/commerce)
- `api` - Medusa E-commerce API (puerto 8000) - [medusajs/medusa](https://github.com/medusajs/medusa)
- `auth` - NextAuth.js (puerto 8001) - [nextauthjs/next-auth](https://github.com/nextauthjs/next-auth)
- `payments` - Stripe Node Examples (puerto 8002) - [stripe/stripe-node](https://github.com/stripe/stripe-node)
- `notifications` - Bull Queue Notifications (puerto 8003) - [OptimalBits/bull](https://github.com/OptimalBits/bull)

**Infraestructura:**
- PostgreSQL 15 (puerto 5432)
- Redis 7 (puerto 6379)
- RabbitMQ 3 (puertos 5672, 15672)

**Uso:**
```bash
cd examples/03-ecommerce-platform
raioz up
```

---

### 04-host-and-docker
Ejemplo que muestra servicios ejecutándose directamente en el host (`source.command`) y otros en Docker (`docker.command`).

**Servicios Host (ejecución directa):**
- `host-frontend` - Next.js Hello World ejecutándose en host - [vercel/next.js](https://github.com/vercel/next.js)
- `host-worker` - FastAPI ejecutándose en host (puerto 8000) - [tiangolo/fastapi](https://github.com/tiangolo/fastapi)
- `host-scheduler` - Node Cron ejecutándose en host - [node-cron/node-cron](https://github.com/node-cron/node-cron)

**Servicios Docker:**
- `docker-api` - JSON Server en Docker (puerto 3001) - [typicode/json-server](https://github.com/typicode/json-server)
- `docker-processor` - Gin RealWorld en Docker (puerto 8080) - [gin-gonic/examples](https://github.com/gin-gonic/examples)

**Infraestructura:**
- PostgreSQL 15 (puerto 5432)
- Redis 7 (puerto 6379)

**Uso:**
```bash
cd examples/04-host-and-docker
raioz up
```

---

## 🚀 Cómo Usar los Ejemplos

1. **Copiar un ejemplo:**
   ```bash
   cp examples/01-basic-web-app/.raioz.json .
   ```

2. **Editar la configuración:**
   - Reemplazar los repositorios Git con los tuyos
   - Ajustar puertos si hay conflictos
   - Modificar comandos según tus necesidades

3. **Inicializar el proyecto:**
   ```bash
   raioz up
   ```

4. **Verificar estado:**
   ```bash
   raioz status
   ```

5. **Ver logs:**
   ```bash
   raioz logs --follow
   ```

6. **Detener servicios:**
   ```bash
   raioz down
   ```

## 📝 Notas Importantes

- **Repositorios Git:** Los repositorios en los ejemplos son repositorios públicos reales de GitHub. Estos son ejemplos funcionales que puedes usar como referencia.
- **Puertos:** Verifica que los puertos no estén en uso en tu sistema antes de ejecutar `raioz up`.
- **Variables de entorno:** Los ejemplos usan archivos `.env` que debes crear según tus necesidades.
- **Imágenes Docker:** Las imágenes de infraestructura son oficiales de Docker Hub y funcionan sin configuración adicional.
- **Servicios Host:** Los servicios con `source.command` se ejecutan directamente en el host, asegúrate de tener las dependencias instaladas (Node.js, Python, Go, etc.).

## 🔧 Personalización

Cada ejemplo puede ser personalizado según tus necesidades:

- **Agregar servicios:** Agrega nuevas entradas en `services`
- **Cambiar infraestructura:** Modifica o agrega servicios en `infra`
- **Ajustar dependencias:** Modifica `dependsOn` para cambiar el orden de inicio
- **Usar perfiles:** Agrega `profiles` para filtrar servicios por contexto

## 📚 Más Información

Para más información sobre la configuración de Raioz, consulta:
- `docs/COMMANDS.md` - Documentación completa de comandos
- `docs/casos-de-uso/` - Casos de uso detallados
