# Recomendación para Proyecto Gateway Nginx

## 📋 Análisis de Tu Proyecto

Tu proyecto `gateway` tiene:
- ✅ `docker-compose.yml` completo y funcional
- ✅ `installer.sh` que prepara el entorno (genera configs, SSL, etc.)
- ✅ Configuración extensa de nginx/lua
- ✅ Scripts de generación de configuraciones desde templates
- ✅ Lógica compleja de instalación previa

## ❌ Problema con Tu `.raioz.json` Actual

Tu `.raioz.json` actual está:
- ❌ Duplicando toda la configuración del `docker-compose.yml`
- ❌ Usando campos no válidos en Raioz (`image`, `containerName`, `environment`, etc.)
- ❌ Ignorando tu `installer.sh` (que es crítico para el funcionamiento)
- ❌ No ejecuta la generación de configuraciones desde templates

## ✅ Solución Recomendada

### Migración: De `docker: {}` a `project.commands`

**Reemplaza tu `.raioz.json` actual por este**:

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
      "down": "cd docker && docker compose -f docker-compose.yml down",
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

---

## 🔄 ¿Qué Cambia?

### Antes (tu `.raioz.json` actual)

```json
{
  "services": {
    "nginx": {
      "docker": {
        "mode": "dev",
        "image": "openresty/openresty:alpine",  // ❌ Campo no válido
        "containerName": "raioz-roax-nginx",    // ❌ Campo no válido
        "ports": ["80:80", "443:443"],
        // ... duplicas TODO el docker-compose.yml
      }
    }
  }
}
```

**Problemas**:
- ❌ Duplicas toda la configuración
- ❌ Campos no válidos (`image`, `containerName`, `environment`)
- ❌ No ejecuta `installer.sh` (generación de configs)
- ❌ No genera certificados SSL
- ❌ Más difícil de mantener (dos fuentes de verdad)

### Después (recomendado)

```json
{
  "project": {
    "commands": {
      "up": "bash installer.sh --deploy",
      "down": "cd docker && docker compose -f docker-compose.yml down"
    }
  }
}
```

**Ventajas**:
- ✅ Usa tu `docker-compose.yml` existente (sin duplicar)
- ✅ Ejecuta `installer.sh` automáticamente (genera configs, SSL)
- ✅ Una sola fuente de verdad (tu `docker-compose.yml`)
- ✅ Más simple y mantenible
- ✅ Funciona con Raioz (`status`, `logs`, etc.)

---

## 🚀 Flujo de Trabajo con la Nueva Configuración

### `raioz up`

1. Raioz ejecuta: `bash installer.sh --deploy`
2. `installer.sh`:
   - Genera configuraciones desde templates (`generate-configs.sh`)
   - Crea directorios necesarios
   - Ejecuta: `cd docker && docker compose up -d nginx`
   - Genera certificados SSL (si `--ssl`)
3. Raioz detecta automáticamente el `docker-compose.yml`
4. Servicios aparecen en `raioz status` y `raioz logs`

### `raioz down`

1. Raioz ejecuta: `cd docker && docker compose -f docker-compose.yml down`
2. Detiene todos los servicios correctamente

### `raioz status` / `raioz logs`

- Detecta servicios del `docker-compose.yml` automáticamente
- Muestra estado y logs correctamente

---

## 🎯 ¿Por Qué NO Usar Dockerfile?

### ❌ Si usaras Dockerfile + `docker: {}`:

**Problemas**:
1. **Perderías `installer.sh`**:
   - No se ejecutaría la generación de configs desde templates
   - No se generarían certificados SSL automáticamente
   - Deberías ejecutar manualmente antes de cada `raioz up`

2. **Duplicarías configuración**:
   ```json
   {
     "docker": {
       "volumes": [
         "./scripts:/code/scripts:rw",
         "./templates:/code/templates:ro",
         "./conf:/etc/nginx/conf.d:rw",
         // ... 9 volúmenes más
       ]
     }
   }
   ```
   Ya tienes esto en `docker-compose.yml`, ¿por qué duplicarlo?

3. **Más complejo**:
   - Tienes que crear `Dockerfile.dev`
   - Mantener dos configuraciones sincronizadas
   - Más propenso a errores

---

## ✅ Ventajas del Enfoque Recomendado

| Aspecto | Con `docker: {}` | Con `project.commands` |
|---------|------------------|----------------------|
| **Duplicación** | ❌ Duplicas `docker-compose.yml` | ✅ Usas el existente |
| **Installer.sh** | ❌ No se ejecuta | ✅ Se ejecuta automáticamente |
| **Generación de configs** | ❌ Manual | ✅ Automática (via installer.sh) |
| **SSL/Certificados** | ❌ Manual | ✅ Automático (via installer.sh) |
| **Mantenimiento** | ❌ Dos archivos a mantener | ✅ Solo `docker-compose.yml` |
| **Simplicidad** | ❌ Verboso | ✅ Simple |
| **Compatibilidad** | ❌ Campos no válidos | ✅ 100% válido |

---

## 📝 Pasos para Migrar

1. **Backup tu `.raioz.json` actual**:
   ```bash
   cp .raioz.json .raioz.json.backup
   ```

2. **Reemplaza con la configuración recomendada**:
   ```bash
   # Copia el contenido recomendado arriba
   ```

3. **Verifica que `installer.sh` funciona**:
   ```bash
   ./installer.sh --deploy
   ```

4. **Prueba con Raioz**:
   ```bash
   raioz up
   raioz status
   raioz logs
   raioz down
   ```

---

## 🔧 Mejoras Opcionales al `installer.sh`

Puedes hacer que `installer.sh` sea más amigable para Raioz:

```bash
#!/bin/bash
# ... código existente ...

# Detectar si se ejecuta desde Raioz
if [ -n "$RAIOZ_MODE" ]; then
    echo "Ejecutando en modo Raioz..."
    # Regenerar configuraciones siempre antes de deployar
    INSTALLER_MODE=true ./scripts/generate-configs.sh
fi

# ... resto del código ...
```

Raioz automáticamente setea `RAIOZ_MODE=dev` o `RAIOZ_MODE=prod` según el modo.

---

## 📊 Comparación Final

### Tu Situación Actual

```
gateway/
├── .raioz.json              ← Duplica docker-compose.yml
├── docker/
│   └── docker-compose.yml   ← Configuración real (funciona)
├── installer.sh             ← No se ejecuta desde Raioz
└── scripts/
    └── generate-configs.sh  ← No se ejecuta automáticamente
```

**Problema**: Dos fuentes de verdad, `installer.sh` no se ejecuta.

### Con la Migración

```
gateway/
├── .raioz.json              ← Simple, usa project.commands
├── docker/
│   └── docker-compose.yml   ← Única fuente de verdad
├── installer.sh             ← Se ejecuta automáticamente en raioz up
└── scripts/
    └── generate-configs.sh  ← Se ejecuta via installer.sh
```

**Ventaja**: Una sola fuente de verdad, todo se ejecuta automáticamente.

---

## 🎯 Resumen

### ✅ Recomendación Final

**Para tu proyecto gateway, usa `project.commands`** porque:

1. ✅ Ya tienes `docker-compose.yml` completo y funcional
2. ✅ Tienes `installer.sh` que hace trabajo importante (genera configs, SSL)
3. ✅ No duplicas configuración
4. ✅ Más simple y mantenible
5. ✅ Funciona perfectamente con Raioz

### ❌ NO uses `docker: {}` porque:

1. ❌ Duplicas toda la configuración del `docker-compose.yml`
2. ❌ Campos inválidos en Raioz (`image`, `containerName`, etc.)
3. ❌ No ejecuta `installer.sh` (pierdes generación de configs, SSL)
4. ❌ Más propenso a errores y difícil de mantener

---

## 🚀 Siguiente Paso

**Reemplaza tu `.raioz.json` actual** con la configuración recomendada arriba y prueba:

```bash
raioz up
```

Todo debería funcionar automáticamente, incluyendo la ejecución de `installer.sh`.
