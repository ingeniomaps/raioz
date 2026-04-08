# Feature: `raioz snapshot` (Guardar y restaurar estado de volumes)

## Resumen

Comando para capturar y restaurar el estado de los volumes Docker de un proyecto. Permite al desarrollador guardar el contenido de bases de datos, colas y otros servicios con estado en un punto en el tiempo, y restaurarlo cuando necesite volver a ese estado (por ejemplo, después de una migración fallida o para compartir un dataset de prueba).

```bash
raioz snapshot create seed-data       # Captura el estado actual de todos los volumes
raioz snapshot create pre-migration --only db  # Solo el volume del servicio db
raioz snapshot restore seed-data      # Restaura todos los volumes desde el snapshot
raioz snapshot list                   # Lista snapshots del proyecto actual
raioz snapshot delete old-backup      # Elimina un snapshot
```

## Valor para el desarrollador

**Sin snapshot:**
```
1. Ejecutar migración destructiva                   (2 seg)
2. Descubrir que la migración rompió datos           (5 min)
3. raioz down -v                                    (3 seg)
4. raioz up                                         (15 seg)
5. Esperar que servicios estén healthy               (10 seg)
6. Re-importar seed SQL manualmente                  (2 min)
7. Verificar que todo quedó bien                     (1 min)
Total: ~8 min para volver al estado anterior
```

**Con snapshot:**
```
1. raioz snapshot create pre-migration               (5 seg)
2. Ejecutar migración destructiva                    (2 seg)
3. Descubrir que la migración rompió datos           (5 min)
4. raioz snapshot restore pre-migration              (8 seg)
5. Verificar que todo quedó bien                     (30 seg)
Total: ~6 min, sin pasos manuales de re-importación
```

Otros casos de uso:
- **Compartir datasets:** exportar snapshot, copiarlo a otro equipo, restaurar
- **Reproducir bugs:** guardar el estado exacto que causa el error
- **Tests de integración:** partir siempre del mismo estado conocido
- **Reset rápido:** volver a un "estado limpio" después de probar flujos destructivos

## Diseno tecnico

### Arquitectura

```
┌───────────────────┐
│  raioz snapshot   │
│  (Cobra command)  │
└────────┬──────────┘
         │
┌────────▼──────────┐
│  SnapshotUseCase  │
│  (internal/app/)  │
└────────┬──────────┘
         │
    ┌────┴─────────────────┐
    │                      │
┌───▼────────┐    ┌───────▼────────┐
│ SnapshotMgr│    │  DockerRunner  │
│ (metadata, │    │  (export/import│
│  storage)  │    │   via tar)     │
└────────────┘    └────────────────┘
         │                │
    ┌────▼────┐    ┌─────▼──────┐
    │ ~/.raioz│    │  Docker    │
    │/snapshot│    │  volumes   │
    │ s/{proj}│    │            │
    └─────────┘    └────────────┘
```

### Componentes

#### 1. Snapshot Manager (`internal/snapshot/manager.go`)

Gestiona la metadata y el almacenamiento de snapshots en disco:

```go
type Manager struct {
    baseDir string // ~/.raioz/snapshots
}

type Snapshot struct {
    Name      string            `json:"name"`
    Project   string            `json:"project"`
    CreatedAt time.Time         `json:"created_at"`
    Volumes   []VolumeSnapshot  `json:"volumes"`
}

type VolumeSnapshot struct {
    VolumeName  string `json:"volume_name"`
    ServiceName string `json:"service_name"`
    SizeBytes   int64  `json:"size_bytes"`
    ArchiveFile string `json:"archive_file"`
}
```

Responsabilidades:
- Crear/leer/eliminar directorios de snapshots
- Serializar/deserializar metadata (`snapshot.json`)
- Calcular espacio en disco disponible antes de crear
- Listar snapshots de un proyecto con sus tamanios

#### 2. Volume Exporter (`internal/snapshot/exporter.go`)

Exporta el contenido de un volume Docker a un archivo `.tar.gz`:

```go
type Exporter struct {
    docker interfaces.DockerRunner
}

func (e *Exporter) Export(volumeName, destPath string) (int64, error)
```

Internamente ejecuta:
```bash
docker run --rm \
  -v {volumeName}:/data:ro \
  -v {destDir}:/backup \
  alpine tar czf /backup/{name}.tar.gz -C /data .
```

El volume se monta como `:ro` (read-only) para evitar modificaciones accidentales durante la exportacion.

#### 3. Volume Importer (`internal/snapshot/importer.go`)

Restaura el contenido de un `.tar.gz` en un volume Docker:

```go
type Importer struct {
    docker interfaces.DockerRunner
}

func (i *Importer) Import(archivePath, volumeName string) error
```

Internamente ejecuta:
```bash
docker run --rm \
  -v {volumeName}:/data \
  -v {archiveDir}:/backup:ro \
  alpine sh -c "rm -rf /data/* /data/.[!.]* && tar xzf /backup/{name}.tar.gz -C /data"
```

Nota: tambien elimina archivos ocultos (`.[!.]*`) para asegurar un restore limpio.

#### 4. Volume Resolver (`internal/snapshot/resolver.go`)

Resuelve que volumes pertenecen al proyecto actual y los mapea a servicios:

```go
type Resolver struct {
    config interfaces.ConfigLoader
    state  interfaces.StateManager
}

func (r *Resolver) ResolveVolumes(only []string) ([]VolumeInfo, error)
```

Lee la configuracion del proyecto y el estado actual para determinar:
- Nombres reales de volumes Docker (con prefijo del proyecto)
- Que servicio usa cada volume
- Filtrado por `--only` si se especifica

## Subcomandos

### `raioz snapshot create <name>`

Captura el estado actual de los volumes del proyecto.

```bash
raioz snapshot create seed-data
raioz snapshot create pre-migration --only db,redis
```

| Flag | Descripcion | Default |
|------|-------------|---------|
| `--only` | Lista de servicios a incluir (separados por coma) | todos |

**Flujo:**
1. Validar que el nombre no exista (o `--force` para sobreescribir)
2. Resolver volumes del proyecto (filtrados por `--only` si aplica)
3. Para cada volume: exportar con `docker run ... tar czf`
4. Guardar metadata en `snapshot.json`
5. Mostrar resumen con tamanios

**Output:**
```
Creating snapshot 'pre-migration'...

  db-data        42.3 MB  (3.2s)
  redis-data      1.1 MB  (0.4s)

Snapshot 'pre-migration' created (43.4 MB total)
Stored in ~/.raioz/snapshots/billing/pre-migration/
```

### `raioz snapshot restore <name>`

Restaura el estado de volumes desde un snapshot.

```bash
raioz snapshot restore seed-data
raioz snapshot restore pre-migration --only db
```

| Flag | Descripcion | Default |
|------|-------------|---------|
| `--only` | Lista de servicios a restaurar (separados por coma) | todos los del snapshot |
| `--stop` | Detener servicios antes de restaurar | `true` |

**Flujo:**
1. Leer metadata del snapshot
2. Validar que los volumes existan
3. Si `--stop` (default): detener servicios que usan los volumes afectados
4. Para cada volume: importar con `docker run ... tar xzf`
5. Si servicios fueron detenidos: reiniciarlos
6. Mostrar resumen

**Output:**
```
Restoring snapshot 'pre-migration'...

Stopping services: db, redis
  db-data        42.3 MB  (2.8s)
  redis-data      1.1 MB  (0.3s)
Restarting services: db, redis

Snapshot 'pre-migration' restored successfully
```

### `raioz snapshot list`

Lista los snapshots disponibles para el proyecto actual.

```bash
raioz snapshot list
```

**Output:**
```
Snapshots for 'billing':

  NAME             CREATED              SIZE      VOLUMES
  seed-data        2026-04-01 10:30     43.4 MB   db-data, redis-data
  pre-migration    2026-04-05 14:22     42.3 MB   db-data
  clean-state      2026-03-28 09:15     85.7 MB   db-data, redis-data, minio-data

Total: 3 snapshots (171.4 MB)
```

### `raioz snapshot delete <name>`

Elimina un snapshot y libera su espacio en disco.

```bash
raioz snapshot delete old-backup
```

**Output:**
```
Deleted snapshot 'old-backup' (43.4 MB freed)
```

## Como funciona internamente

### Exportacion (create)

```
raioz snapshot create seed-data
│
├── 1. Resolver volumes del proyecto
│      - Leer .raioz.json → servicios con volumes
│      - Leer .state.json → nombres reales de volumes Docker
│      - Aplicar filtro --only si existe
│
├── 2. Verificar espacio en disco
│      - Estimar tamano: docker system df -v | filtrar volumes
│      - Comparar con espacio libre en ~/.raioz/snapshots/
│
├── 3. Crear directorio del snapshot
│      ~/.raioz/snapshots/{project}/{name}/
│
├── 4. Para cada volume:
│      docker run --rm \
│        -v {volume}:/data:ro \
│        -v {snapshotDir}:/backup \
│        alpine tar czf /backup/{volume}.tar.gz -C /data .
│
├── 5. Guardar metadata
│      ~/.raioz/snapshots/{project}/{name}/snapshot.json
│      {
│        "name": "seed-data",
│        "project": "billing",
│        "created_at": "2026-04-05T14:22:00Z",
│        "volumes": [
│          {
│            "volume_name": "billing_db-data",
│            "service_name": "db",
│            "size_bytes": 44347392,
│            "archive_file": "billing_db-data.tar.gz"
│          }
│        ]
│      }
│
└── 6. Mostrar resumen
```

### Importacion (restore)

```
raioz snapshot restore seed-data
│
├── 1. Leer metadata del snapshot
│      ~/.raioz/snapshots/{project}/{name}/snapshot.json
│
├── 2. Validar que volumes existan en Docker
│      docker volume inspect {volume} para cada uno
│
├── 3. Detener servicios afectados (si --stop=true)
│      docker compose stop {service1} {service2}
│
├── 4. Para cada volume:
│      docker run --rm \
│        -v {volume}:/data \
│        -v {snapshotDir}:/backup:ro \
│        alpine sh -c "rm -rf /data/* /data/.[!.]* && \
│                       tar xzf /backup/{volume}.tar.gz -C /data"
│
├── 5. Reiniciar servicios detenidos
│      docker compose start {service1} {service2}
│
└── 6. Mostrar resumen
```

### Estructura de almacenamiento

```
~/.raioz/snapshots/
└── billing/                        # Nombre del proyecto
    ├── seed-data/                  # Nombre del snapshot
    │   ├── snapshot.json           # Metadata
    │   ├── billing_db-data.tar.gz  # Backup del volume db-data
    │   └── billing_redis-data.tar.gz
    └── pre-migration/
        ├── snapshot.json
        └── billing_db-data.tar.gz
```

## Edge cases a manejar

| Caso | Comportamiento |
|------|---------------|
| Servicio corriendo durante create | Montar volume como `:ro`. Warning de posible inconsistencia si el servicio escribe activamente |
| Servicio corriendo durante restore | Detener servicio primero (`--stop=true` por defecto). Si `--stop=false`, error con sugerencia |
| Fallo parcial en restore | Revertir volumes ya restaurados al estado previo (crear backup temporal antes de restaurar). Mostrar que volumes quedaron en estado inconsistente si la reversion tambien falla |
| Disco lleno durante create | Detectar error de tar, limpiar archivos parciales, mostrar espacio requerido estimado |
| Volume no existe al restaurar | Warning, skip ese volume, continuar con los demas. Opcion `--create-missing` para crear volumes vacios antes de restaurar |
| Snapshot con mismo nombre ya existe | Error con sugerencia de usar `--force` para sobreescribir |
| Proyecto sin volumes | Mensaje informativo, no crear snapshot vacio |
| Imagen alpine no disponible | `docker pull alpine` automatico con mensaje. Fallo si no hay conexion |
| Nombre de snapshot invalido | Validar: solo alfanumericos, guiones y guiones bajos. Max 64 caracteres |
| Volumes muy grandes (>1 GB) | Warning con el tamano estimado y confirmacion interactiva. Flag `--yes` para skip |
| Snapshot creado con version vieja de raioz | Validar version del schema de metadata. Migrar si es posible |

## Archivos a crear

```
internal/snapshot/
├── manager.go          # Gestion de metadata y almacenamiento      (~100 lineas)
├── exporter.go         # Exportar volume → tar.gz                  (~70 lineas)
├── importer.go         # Importar tar.gz → volume                  (~80 lineas)
├── resolver.go         # Resolver volumes del proyecto              (~60 lineas)
├── manager_test.go     # Tests del manager                         (~150 lineas)
├── exporter_test.go    # Tests del exporter (mock DockerRunner)     (~100 lineas)
├── importer_test.go    # Tests del importer                        (~100 lineas)
└── resolver_test.go    # Tests del resolver                        (~80 lineas)

internal/app/
├── snapshot.go         # Use case: SnapshotUseCase con Execute()    (~90 lineas)
└── snapshot_test.go    # Tests del use case                        (~120 lineas)

cmd/raioz/
└── snapshot.go         # Cobra commands: create, restore, list, delete  (~120 lineas)
```

### Cambios en archivos existentes

| Archivo | Cambio |
|---------|--------|
| `cmd/raioz/main.go` | Registrar `snapshotCmd` como subcomando |
| `internal/domain/interfaces/snapshot.go` | Interfaz `SnapshotManager` (nuevo archivo en domain) |
| `internal/app/dependencies.go` | Agregar `SnapshotManager` a `Dependencies` |
| `internal/infra/adapters.go` | Adapter para `SnapshotManager` |
| `internal/mocks/snapshot.go` | Mock de `SnapshotManager` |
| `internal/i18n/locales/en.json` | ~25 keys para mensajes de snapshot |
| `internal/i18n/locales/es.json` | Traducciones al espanol |
| `cmd/raioz/zzz_i18n_descriptions.go` | Descripciones i18n de los subcomandos |

### Estimacion de complejidad

| Componente | Complejidad | Lineas estimadas |
|-----------|-------------|-----------------|
| Snapshot Manager | Media | ~100 |
| Exporter | Baja | ~70 |
| Importer | Media | ~80 |
| Resolver | Baja | ~60 |
| Use case | Media | ~90 |
| Cobra commands | Baja | ~120 |
| Domain interface | Baja | ~20 |
| Infra adapter | Baja | ~30 |
| Mock | Baja | ~40 |
| i18n keys | Baja | ~50 |
| Tests | Media | ~550 |
| **Total** | | **~1210 lineas** |

## Criterios de aceptacion

- [ ] `raioz snapshot create <name>` exporta todos los volumes del proyecto a `~/.raioz/snapshots/{project}/{name}/`
- [ ] `raioz snapshot create <name> --only db` exporta solo el volume del servicio especificado
- [ ] `raioz snapshot restore <name>` restaura todos los volumes del snapshot
- [ ] `raioz snapshot restore <name> --only db` restaura solo el volume especificado
- [ ] `raioz snapshot restore` detiene y reinicia servicios afectados automaticamente
- [ ] `raioz snapshot list` muestra nombre, fecha, tamano y volumes de cada snapshot
- [ ] `raioz snapshot delete <name>` elimina el directorio del snapshot y libera espacio
- [ ] Error claro si el nombre del snapshot ya existe (sugerir `--force`)
- [ ] Error claro si el snapshot no existe al restaurar/eliminar
- [ ] Warning si un servicio esta corriendo activamente durante create
- [ ] Fallo parcial en restore: intenta revertir volumes ya restaurados
- [ ] Validacion de espacio en disco antes de crear snapshot
- [ ] Metadata JSON con timestamp, volumes, tamanios
- [ ] Todos los mensajes al usuario pasan por `i18n.T()`
- [ ] Tests unitarios para manager, exporter, importer, resolver y use case
- [ ] Tests con mock de DockerRunner (no requieren Docker real)
- [ ] Funciona sin volumes en el proyecto (mensaje informativo, sin error)
