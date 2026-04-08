# Raioz: Instalacion, Release y Binarios

## Requisitos Previos

### Para Usuarios

- **Docker**: Instalado y corriendo ([instalar Docker](https://docs.docker.com/get-docker/))
- **Git**: Instalado ([instalar Git](https://git-scm.com/downloads))
- **Bash**: Disponible (viene con Linux/macOS)

### Para Desarrolladores / Mantenedores

- **Go 1.21+**: Para compilar el codigo fuente
- **Make**: Para usar los comandos del Makefile
- Acceso al repositorio de GitHub con permisos para crear releases

---

## Instalacion

### Desde GitHub (Usuarios)

```bash
curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash
```

El script:
1. Detecta tu sistema operativo (Linux/macOS) y arquitectura
2. Descarga el binario pre-compilado desde GitHub releases
3. Instala raioz en `/usr/local/bin` (requiere sudo)
4. Verifica que Docker y Git esten instalados

Para instalar en otro directorio:

```bash
INSTALL_DIR=$HOME/.local/bin curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash
```

Para instalar una version especifica:

```bash
VERSION=v1.0.0 curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash
```

### Desde Codigo Local (Desarrolladores)

```bash
# Compilar e instalar
make build
make install
```

`make install` compila el binario, detecta el binario local (`./raioz`) y usa `install.sh` en modo desarrollo (no descarga de GitHub).

Instalacion manual alternativa:

```bash
make build
sudo cp raioz /usr/local/bin/
sudo chmod +x /usr/local/bin/raioz
```

### Verificar Instalacion

```bash
raioz version
raioz --help
```

Si el comando no se encuentra, agrega `/usr/local/bin` a tu PATH:

```bash
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc  # o ~/.zshrc
source ~/.bashrc
```

### Actualizar Raioz

Usuarios: ejecuta el instalador nuevamente. Desarrolladores: `make install`.

---

## Proceso de Release

### 1. Preparar el Release

```bash
git checkout main
git pull origin main
make check
make release-installer
```

### 2. Crear el Release

#### Automatico con GitHub Actions (Recomendado)

Cuando creas un tag, GitHub Actions automaticamente compila binarios para todas las plataformas, genera archivos comprimidos y sube todo a GitHub Releases.

```bash
git tag v1.0.0
git push origin v1.0.0
```

O desde la interfaz web: **Releases** -> **Draft a new release** -> crea un tag `v1.0.0`.

Tambien puedes usar **workflow_dispatch**: **Actions** -> **Release** -> **Run workflow** -> ingresa la version.

Ejemplo de workflow:

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

#### Manual

Si prefieres crear los binarios manualmente:

```bash
# Usando el Makefile
make build-release-binaries
```

O compilando individualmente:

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

### 3. Subir Binarios a GitHub Releases

Con GitHub CLI:

```bash
gh auth login
gh release upload v1.0.0 raioz-* --clobber
```

O desde la interfaz web: arrastra los archivos `raioz-*` a la seccion de assets del release.

### 4. Actualizar el Repositorio en install.sh

```bash
GITHUB_REPO="${GITHUB_REPO:-ingeniomaps/raioz}"  # Cambiar por tu repo
```

### Checklist de Release

- [ ] Codigo probado y tests pasando
- [ ] Version bump en el codigo
- [ ] Changelog actualizado
- [ ] Binarios compilados para todas las plataformas
- [ ] Release creado en GitHub con binarios adjuntos
- [ ] Script `install.sh` actualizado y probado
- [ ] Documentacion actualizada
- [ ] Release notes escritas

---

## Compilacion de Binarios

### Estructura de Archivos en GitHub Release

Binarios directos (patron: `raioz-<os>-<arch>`):

- `raioz-linux-amd64`
- `raioz-linux-arm64`
- `raioz-darwin-amd64`
- `raioz-darwin-arm64`

Archivos comprimidos (generados por GoReleaser):

- `raioz_1.0.0_linux_amd64.tar.gz`
- `raioz_1.0.0_linux_arm64.tar.gz`
- `raioz_1.0.0_darwin_amd64.tar.gz`
- `raioz_1.0.0_darwin_arm64.tar.gz`
- `raioz_1.0.0_windows_amd64.zip`
- `raioz_1.0.0_windows_arm64.zip`
- `raioz_1.0.0_checksums.txt`

### Configuracion GoReleaser

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

Ejecucion manual:

```bash
goreleaser release --clean
```

El script de instalacion intenta primero binarios directos, luego archivos comprimidos.

---

## Troubleshooting

### El script no encuentra los binarios

1. Verifica que los binarios esten en el release: `https://github.com/ingeniomaps/raioz/releases/latest`
2. Verifica los nombres: deben ser exactamente `raioz-<os>-<arch>`
3. Si solo hay archivos comprimidos, el script los extraera automaticamente

### El script no detecta el binario local

- El binario `raioz` debe existir en el mismo directorio que `install.sh`, o en el directorio actual donde ejecutas `make install`

### Error de permisos

```bash
INSTALL_DIR=$HOME/.local/bin ./install.sh
```

Y agrega ese directorio a tu PATH.

### Docker no esta corriendo

```bash
# Linux
sudo systemctl start docker

# macOS
open -a Docker
```

### El workflow de release falla

1. Verifica que el tag tenga formato correcto: `v1.0.0`
2. Verifica permisos para crear releases
3. Revisa los logs en **Actions** -> **Release**

### Notas

- El script `install.sh` debe estar en la raiz del repositorio y ser ejecutable (`chmod +x`)
- Los binarios directos son opcionales pero recomendados para instalacion mas rapida
- El instalador funciona con `curl` y `wget`
- Para actualizar el instalador, edita `install.sh` y haz commit
