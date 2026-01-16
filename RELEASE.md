# Guía de Release para Raioz

Esta guía explica cómo preparar y publicar un release de Raioz que incluya el script de instalación automático.

## 📋 Prerrequisitos

- Acceso al repositorio de GitHub
- Permisos para crear releases
- Go 1.21+ instalado
- Make instalado

## 🚀 Proceso de Release

### 1. Preparar el Release

```bash
# Asegúrate de estar en la rama principal y tener los cambios más recientes
git checkout main
git pull origin main

# Ejecuta todos los checks
make check

# Genera el binario y prepara el instalador
make release-installer
```

### 2. Compilar Binarios para Múltiples Plataformas

Para que el instalador funcione, necesitas subir binarios compilados para diferentes plataformas a GitHub releases. Puedes hacer esto manualmente o con un script.

**Opción A: Compilación manual**

```bash
# Linux AMD64
GOOS=linux GOARCH=amd64 go build \
  -ldflags "-X 'raioz/cmd.Version=v1.0.0' -X 'raioz/cmd.Commit=$(git rev-parse --short HEAD)' -X 'raioz/cmd.BuildDate=$(date +%Y-%m-%dT%H:%M:%S)'" \
  -o raioz-linux-amd64 ./cmd/raioz

# Linux ARM64
GOOS=linux GOARCH=arm64 go build \
  -ldflags "-X 'raioz/cmd.Version=v1.0.0' -X 'raioz/cmd.Commit=$(git rev-parse --short HEAD)' -X 'raioz/cmd.BuildDate=$(date +%Y-%m-%dT%H:%M:%S)'" \
  -o raioz-linux-arm64 ./cmd/raioz

# macOS AMD64 (Intel)
GOOS=darwin GOARCH=amd64 go build \
  -ldflags "-X 'raioz/cmd.Version=v1.0.0' -X 'raioz/cmd.Commit=$(git rev-parse --short HEAD)' -X 'raioz/cmd.BuildDate=$(date +%Y-%m-%dT%H:%M:%S)'" \
  -o raioz-darwin-amd64 ./cmd/raioz

# macOS ARM64 (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build \
  -ldflags "-X 'raioz/cmd.Version=v1.0.0' -X 'raioz/cmd.Commit=$(git rev-parse --short HEAD)' -X 'raioz/cmd.BuildDate=$(date +%Y-%m-%dT%H:%M:%S)'" \
  -o raioz-darwin-arm64 ./cmd/raioz
```

**Opción B: Usar goreleaser (recomendado para automatización)**

Crea un archivo `.goreleaser.yml` para automatizar la compilación y publicación:

```yaml
# .goreleaser.yml
project_name: raioz
builds:
  - main: ./cmd/raioz
    binary: raioz
    goos:
      - linux
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -X 'raioz/cmd.Version={{.Version}}'
      - -X 'raioz/cmd.Commit={{.Commit}}'
      - -X 'raioz/cmd.BuildDate={{.Date}}'

release:
  github:
    owner: raioz
    name: raioz
  draft: false
  prerelease: false

archives:
  - replacements:
      darwin: Darwin
      linux: Linux
      windows: Windows
      386: i386
      amd64: x86_64
```

Luego ejecuta:

```bash
goreleaser release --clean
```

### 3. Crear el Release en GitHub

1. Ve a GitHub → Releases → "Draft a new release"
2. Crea un nuevo tag (ej: `v1.0.0`)
3. Agrega notas de release
4. Sube los binarios compilados con estos nombres:
   - `raioz-linux-amd64`
   - `raioz-linux-arm64`
   - `raioz-darwin-amd64`
   - `raioz-darwin-arm64`

### 4. Estructura de Archivos en GitHub Release

Los archivos deben estar nombrados según el patrón:

```
raioz-<os>-<arch>
```

Ejemplos:

- `raioz-linux-amd64`
- `raioz-linux-arm64`
- `raioz-darwin-amd64`
- `raioz-darwin-arm64`

### 5. Actualizar el Repositorio en el Script de Instalación

Asegúrate de que `install.sh` tenga el repositorio correcto:

```bash
GITHUB_REPO="${GITHUB_REPO:-ingeniomaps/raioz}"  # Cambiar por tu repo
```

### 6. Probar el Instalador

Después de subir los binarios, prueba el instalador:

```bash
# Instalar desde GitHub
curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash

# O instalar una versión específica
VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash
```

### 7. Verificar la Instalación

```bash
raioz version
raioz --help
```

## 🔄 Proceso Automatizado con GitHub Actions

Para automatizar el proceso completo, puedes usar GitHub Actions. Ejemplo:

```yaml
# .github/workflows/release.yml
name: Release

on:
  push:
    tags:
      - "v*"

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: "1.21"

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v4
        with:
          version: latest
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

## 📝 Notas

- El script `install.sh` debe estar en la raíz del repositorio
- El script debe ser ejecutable (usar `chmod +x install.sh`)
- Los binarios deben seguir el patrón de nombres: `raioz-<os>-<arch>`
- Para actualizar el instalador, simplemente edita `install.sh` y haz commit
- El instalador funciona con `curl` y `wget`

## 🔗 URLs de Instalación

Una vez publicado, los usuarios pueden instalar con:

```bash
# Instalación directa desde GitHub
curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash

# O usando un dominio corto (si configuras un redirect)
curl -fsSL https://raioz.dev/install | bash
```

## ✅ Checklist de Release

- [ ] Código probado y tests pasando
- [ ] Version bump en el código
- [ ] Changelog actualizado
- [ ] Binarios compilados para todas las plataformas
- [ ] Release creado en GitHub con binarios adjuntos
- [ ] Script `install.sh` actualizado y probado
- [ ] Documentación actualizada
- [ ] Release notes escritas
