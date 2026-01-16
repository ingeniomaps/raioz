# Guía de Troubleshooting

Esta guía ayuda a resolver problemas comunes al usar Raioz.

## Tabla de Contenidos

- [Problemas Comunes](#problemas-comunes)
- [Errores de Configuración](#errores-de-configuración)
- [Problemas con Docker](#problemas-con-docker)
- [Problemas con Git](#problemas-con-git)
- [Problemas de Workspace](#problemas-de-workspace)
- [Problemas de Performance](#problemas-de-performance)
- [Debugging Avanzado](#debugging-avanzado)

---

## Problemas Comunes

### "Failed to acquire lock"

**Síntoma:**
```
Error [LOCK_ERROR]: Failed to acquire lock (another raioz process may be running)
```

**Causas:**
- Otro proceso `raioz` está ejecutándose
- Un proceso anterior se quedó colgado
- El archivo `.raioz.lock` existe pero el proceso no está corriendo

**Solución:**
1. Verificar si hay otro proceso ejecutándose:
   ```bash
   ps aux | grep raioz
   ```

2. Si no hay proceso, eliminar el lock manualmente:
   ```bash
   # Encontrar el workspace
   raioz status --project <project-name>

   # Eliminar el lock
   rm <workspace-path>/.raioz.lock
   ```

3. Si el proceso está colgado, matarlo primero:
   ```bash
   kill <PID>
   # Luego eliminar el lock
   ```

---

### "Port conflict detected"

**Síntoma:**
```
Error [PORT_CONFLICT]: Port conflicts detected
```

**Causa:**
- Otro servicio está usando el mismo puerto
- Múltiples proyectos están usando el mismo puerto

**Solución:**
1. Ver qué puertos están en uso:
   ```bash
   raioz ports
   ```

2. Verificar si hay otros contenedores usando el puerto:
   ```bash
   docker ps | grep <port>
   ```

3. Cambiar el puerto en `.raioz.json`:
   ```json
   {
     "services": {
       "api": {
         "docker": {
           "ports": ["3001:3000"]  // Cambiar 3000 a 3001
         }
       }
     }
   }
   ```

4. Si es otro proyecto de Raioz, usar diferentes puertos o detener el otro proyecto:
   ```bash
   raioz down --project other-project
   ```

---

### "Docker daemon is not running"

**Síntoma:**
```
Error [DOCKER_NOT_RUNNING]: Docker daemon is not running
```

**Solución:**
1. Verificar estado de Docker:
   ```bash
   docker ps
   ```

2. Iniciar Docker:
   ```bash
   # Linux (systemd)
   sudo systemctl start docker

   # macOS
   open -a Docker
   ```

3. Verificar que Docker esté corriendo:
   ```bash
   docker version
   ```

---

### "Failed to clone repository"

**Síntoma:**
```
Error [GIT_CLONE_FAILED]: Failed to clone repository
```

**Causas:**
- Problemas de red
- Credenciales incorrectas
- Repositorio no existe o no es accesible
- Permisos insuficientes

**Solución:**
1. Verificar conectividad:
   ```bash
   ping github.com  # o el host del repo
   ```

2. Verificar credenciales SSH:
   ```bash
   ssh -T git@github.com  # para GitHub
   ```

3. Verificar que el repositorio existe y es accesible:
   ```bash
   git ls-remote <repo-url>
   ```

4. Si es un repositorio privado, verificar que las credenciales estén configuradas:
   ```bash
   # Para HTTPS
   git config --global credential.helper store

   # Para SSH
   ssh-add ~/.ssh/id_rsa
   ```

5. Forzar re-clonado:
   ```bash
   raioz up --force-reclone
   ```

---

### "Schema validation failed"

**Síntoma:**
```
Error [SCHEMA_VALIDATION]: Configuration validation errors
```

**Causa:**
- El archivo `.raioz.json` no cumple con el schema
- Campos faltantes o incorrectos
- Tipos de datos incorrectos

**Solución:**
1. Verificar el error específico en el mensaje

2. Validar el JSON:
   ```bash
   # Verificar sintaxis JSON
   cat .raioz.json | jq .
   ```

3. Revisar la documentación del schema en `README.md`

4. Ejemplos comunes:
   - Campo faltante: agregar el campo requerido
   - Tipo incorrecto: `"ports": 3000` → `"ports": ["3000:3000"]`
   - Valor inválido: `"mode": "development"` → `"mode": "dev"`

---

### "Service not found" o "Service depends on X which does not exist"

**Síntoma:**
```
Error [INVALID_FIELD]: Service 'api': depends on 'database' which does not exist
```

**Causa:**
- Referencia a un servicio/infra que no existe
- Error tipográfico en el nombre

**Solución:**
1. Verificar que el servicio/infra existe en `.raioz.json`:
   ```bash
   cat .raioz.json | jq '.services, .infra'
   ```

2. Verificar el nombre exacto (case-sensitive):
   ```json
   {
     "services": {
       "api": {
         "docker": {
           "dependsOn": ["database"]  // Debe existir en services o infra
         }
       }
     },
     "infra": {
       "database": { ... }  // ✓ Existe
     }
   }
   ```

3. Corregir el nombre en `dependsOn` si hay error tipográfico

---

## Errores de Configuración

### "Project name validation failed"

**Síntoma:**
```
Error [INVALID_FIELD]: Project name validation failed
```

**Causa:**
- Nombre del proyecto contiene caracteres inválidos
- Nombre demasiado largo (>63 caracteres)

**Solución:**
1. El nombre debe contener solo:
   - Letras minúsculas (a-z)
   - Números (0-9)
   - Guiones (-)
   - Guiones bajos (_)

2. Máximo 63 caracteres

3. Ejemplo correcto:
   ```json
   {
     "project": {
       "name": "my-project"  // ✓ Válido
     }
   }
   ```

4. Ejemplos inválidos:
   ```json
   {
     "project": {
       "name": "My Project",  // ✗ Mayúsculas y espacios
       "name": "my.project",  // ✗ Punto no permitido
       "name": "my/project"   // ✗ Slash no permitido
     }
   }
   ```

---

### "Missing required field"

**Síntoma:**
```
Error [MISSING_FIELD]: Service 'api': git source requires 'repo' field
```

**Solución:**
1. Revisar el mensaje de error para identificar el campo faltante

2. Agregar el campo requerido según el tipo de fuente:

   **Para `source.kind: "git"`:**
   ```json
   {
     "source": {
       "kind": "git",
       "repo": "git@github.com:org/repo.git",  // ✓ Requerido
       "branch": "main",                        // ✓ Requerido
       "path": "./services/api"                 // ✓ Requerido
     }
   }
   ```

   **Para `source.kind: "image"`:**
   ```json
   {
     "source": {
       "kind": "image",
       "image": "nginx",    // ✓ Requerido
       "tag": "latest"      // ✓ Requerido
     }
   }
   ```

---

## Problemas con Docker

### "Image pull failed"

**Síntoma:**
```
Error [IMAGE_PULL_FAILED]: Failed to validate or pull Docker images
```

**Causas:**
- Imagen no existe
- Tag incorrecto
- Problemas de red
- Permisos insuficientes

**Solución:**
1. Verificar que la imagen existe:
   ```bash
   docker pull <image>:<tag>
   ```

2. Verificar conectividad:
   ```bash
   docker pull hello-world
   ```

3. Si es imagen privada, hacer login:
   ```bash
   docker login
   ```

4. Verificar el tag en `.raioz.json`:
   ```json
   {
     "source": {
       "image": "nginx",
       "tag": "latest"  // Verificar que el tag existe
     }
   }
   ```

---

### "Failed to start Docker Compose services"

**Síntoma:**
```
Error [DOCKER_NOT_RUNNING]: Failed to start Docker Compose services
```

**Solución:**
1. Verificar logs de Docker Compose:
   ```bash
   docker compose -f docker-compose.generated.yml logs
   ```

2. Verificar que Docker tiene recursos suficientes:
   ```bash
   docker system df
   ```

3. Verificar que no hay conflictos de puertos:
   ```bash
   raioz ports
   ```

4. Verificar el archivo generado:
   ```bash
   cat docker-compose.generated.yml
   ```

5. Intentar levantar manualmente para más detalles:
   ```bash
   docker compose -f docker-compose.generated.yml up
   ```

---

### "Volume error" o "Network error"

**Síntoma:**
```
Error [VOLUME_ERROR]: Failed to ensure Docker volume
Error [NETWORK_ERROR]: Failed to ensure Docker network
```

**Solución:**
1. Verificar permisos de Docker:
   ```bash
   docker volume ls
   docker network ls
   ```

2. Limpiar recursos conflictivos:
   ```bash
   # Ver volúmenes
   docker volume ls

   # Eliminar volumen conflictivo (si es seguro)
   docker volume rm <volume-name>

   # Ver redes
   docker network ls

   # Eliminar red conflictiva (si es seguro)
   docker network rm <network-name>
   ```

3. Verificar espacio en disco:
   ```bash
   df -h
   docker system df
   ```

---

## Problemas con Git

### "Branch not found"

**Síntoma:**
```
Error [GIT_BRANCH_NOT_FOUND]: Branch 'main' not found
```

**Solución:**
1. Verificar que la rama existe en el repositorio:
   ```bash
   git ls-remote --heads <repo-url> | grep main
   ```

2. Verificar el nombre de la rama (puede ser `master` en lugar de `main`):
   ```json
   {
     "source": {
       "branch": "master"  // o "main", "develop", etc.
     }
   }
   ```

3. Forzar re-clonado:
   ```bash
   raioz up --force-reclone
   ```

---

### "Git conflict" o "Has uncommitted changes"

**Síntoma:**
```
Error [GIT_CONFLICT]: Repository has uncommitted changes
```

**Causa:**
- Cambios locales sin commit en el repositorio
- Conflictos de merge

**Solución:**
1. Verificar cambios:
   ```bash
   cd <workspace-path>/services/<service-name>
   git status
   ```

2. Opciones:
   - **Guardar cambios:**
     ```bash
     git stash
     # o
     git commit -m "WIP: local changes"
     ```

   - **Descartar cambios:**
     ```bash
     git reset --hard HEAD
     ```

   - **Forzar re-clonado:**
     ```bash
     raioz up --force-reclone
     ```

---

## Problemas de Workspace

### "Failed to resolve workspace"

**Síntoma:**
```
Error [WORKSPACE_ERROR]: Failed to resolve workspace
```

**Causas:**
- Permisos insuficientes
- Espacio en disco insuficiente
- Directorio no accesible

**Solución:**
1. Verificar permisos:
   ```bash
   ls -la /opt/raioz-proyecto  # o ~/.raioz
   ```

2. Verificar espacio en disco:
   ```bash
   df -h
   ```

3. Usar workspace alternativo:
   ```bash
   export RAIOZ_HOME=~/custom-raioz
   raioz up
   ```

4. Verificar que el directorio existe y es accesible:
   ```bash
   mkdir -p /opt/raioz-proyecto
   chmod 755 /opt/raioz-proyecto
   ```

---

### "Permission denied"

**Síntoma:**
```
Error [PERMISSION_DENIED]: Cannot create workspace directory
```

**Solución:**
1. Verificar permisos del directorio:
   ```bash
   ls -la /opt/raioz-proyecto
   ```

2. Cambiar permisos si es necesario:
   ```bash
   sudo chown -R $USER:$USER /opt/raioz-proyecto
   chmod 755 /opt/raioz-proyecto
   ```

3. Usar workspace en home directory:
   ```bash
   export RAIOZ_HOME=~/.raioz-workspace
   raioz up
   ```

---

## Problemas de Performance

### "Low disk space"

**Síntoma:**
```
Error [DISK_SPACE_LOW]: Low disk space: X.XX GB available
```

**Solución:**
1. Limpiar recursos Docker no usados:
   ```bash
   raioz clean --images --volumes --networks
   ```

2. Limpiar sistema Docker:
   ```bash
   docker system prune -a --volumes
   ```

3. Verificar espacio:
   ```bash
   df -h
   docker system df
   ```

---

### Comandos lentos

**Síntoma:**
- `raioz up` tarda mucho tiempo
- Operaciones de Git son lentas

**Solución:**
1. Verificar conectividad de red:
   ```bash
   ping github.com
   ```

2. Usar `--skip-pull` si las imágenes ya están locales:
   ```bash
   raioz up --skip-pull  # Si existe este flag
   ```

3. Verificar estado de Docker:
   ```bash
   docker ps
   docker system df
   ```

4. Limpiar recursos no usados:
   ```bash
   raioz clean --images
   ```

---

## Debugging Avanzado

### Habilitar logs de debug

```bash
raioz up --log-level debug
```

Esto mostrará información detallada de todas las operaciones.

---

### Verificar estado interno

```bash
# Ver estado del proyecto
cat <workspace-path>/.state.json | jq .

# Ver configuración raíz
cat <workspace-path>/raioz.root.json | jq .

# Ver overrides
cat ~/.raioz/overrides.json | jq .

# Ver servicios ignorados
cat ~/.raioz/ignore.json | jq .
```

---

### Verificar archivos generados

```bash
# Ver docker-compose generado
cat <workspace-path>/docker-compose.generated.yml

# Ver variables de entorno resueltas
cat <workspace-path>/.env
```

---

### Verificar locks

```bash
# Ver si hay lock activo
ls -la <workspace-path>/.raioz.lock

# Ver contenido del lock (si existe)
cat <workspace-path>/.raioz.lock
```

---

### Verificar logs de audit

```bash
# Ver log de auditoría
tail -f ~/.raioz/audit.log
```

---

## Obtener Ayuda

Si el problema persiste:

1. **Verificar versión:**
   ```bash
   raioz version
   ```

2. **Recopilar información:**
   ```bash
   # Estado del proyecto
   raioz status --json > status.json

   # Logs de debug
   raioz up --log-level debug > debug.log 2>&1

   # Información del sistema
   docker version
   docker info
   git --version
   ```

3. **Revisar documentación:**
   - `README.md` - Documentación principal
   - `docs/COMMANDS.md` - Guía de comandos
   - `docs/limits.md` - Limitaciones conocidas

4. **Reportar el problema:**
   - Incluir versión de Raioz
   - Incluir mensaje de error completo
   - Incluir configuración relevante (sin secrets)
   - Incluir logs de debug si es posible
