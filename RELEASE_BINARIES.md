# Cómo Crear y Publicar Binarios para Releases

Este documento explica cómo generar los binarios que los usuarios descargarán cuando ejecuten el script de instalación.

## 🚀 Opción 1: Automático (Recomendado)

### Usando GitHub Actions (Automático)

Cuando creas un tag de versión, GitHub Actions automáticamente:

1. **Compila los binarios** para todas las plataformas usando GoReleaser
2. **Genera archivos comprimidos** (`.tar.gz` para Linux/macOS, `.zip` para Windows)
3. **Genera binarios directos** (`raioz-linux-amd64`, `raioz-darwin-amd64`, etc.)
4. **Sube todo a GitHub Releases**

### Pasos para crear un release:

```bash
# 1. Asegúrate de estar en la rama main y tener los últimos cambios
git checkout main
git pull origin main

# 2. Crea un tag de versión (formato: v1.0.0)
git tag v1.0.0

# 3. Push el tag (esto activa el workflow de release)
git push origin v1.0.0
```

O usa la interfaz web de GitHub:

- Ve a **Releases** → **Draft a new release**
- Crea un nuevo tag (ej: `v1.0.0`)
- El workflow se ejecutará automáticamente

### Usando workflow_dispatch (Manual desde GitHub)

1. Ve a **Actions** → **Release**
2. Click en **Run workflow**
3. Ingresa la versión (ej: `v1.0.0`)
4. Click en **Run workflow**

## 🔧 Opción 2: Manual

Si prefieres crear los binarios manualmente:

### 1. Compilar los binarios

```bash
# Usa el Makefile (más fácil)
make build-release-binaries
```

Esto generará:

- `raioz-linux-amd64`
- `raioz-linux-arm64`
- `raioz-darwin-amd64`
- `raioz-darwin-arm64`

### 2. Subir a GitHub Releases

#### Opción A: Usando GitHub CLI

```bash
# Instalar GitHub CLI si no lo tienes
# macOS: brew install gh
# Linux: https://cli.github.com/

# Autenticarse
gh auth login

# Subir binarios al release
gh release upload v1.0.0 raioz-* --clobber
```

#### Opción B: Desde la interfaz web

1. Ve a https://github.com/ingeniomaps/raioz/releases
2. Crea un nuevo release o edita uno existente
3. Arrastra los archivos `raioz-*` a la sección de assets
4. Publica el release

## 📦 Estructura de Archivos en GitHub Release

Después de crear un release, deberías tener:

### Archivos comprimidos (generados por GoReleaser):

- `raioz_1.0.0_linux_amd64.tar.gz`
- `raioz_1.0.0_linux_arm64.tar.gz`
- `raioz_1.0.0_darwin_amd64.tar.gz`
- `raioz_1.0.0_darwin_arm64.tar.gz`
- `raioz_1.0.0_windows_amd64.zip`
- `raioz_1.0.0_windows_arm64.zip`
- `raioz_1.0.0_checksums.txt`

### Binarios directos (generados por el workflow):

- `raioz-linux-amd64`
- `raioz-linux-arm64`
- `raioz-darwin-amd64`
- `raioz-darwin-arm64`

## ✅ Verificación

Después de crear el release, verifica que funciona:

```bash
# Probar instalación desde GitHub
curl -fsSL https://raw.githubusercontent.com/ingeniomaps/raioz/main/install.sh | bash

# Verificar que se instaló correctamente
raioz version
```

## 🔍 Troubleshooting

### El script no encuentra los binarios

1. Verifica que los binarios estén en el release:

   - URL: `https://github.com/ingeniomaps/raioz/releases/latest`
   - Deben aparecer en la sección "Assets"

2. Verifica los nombres de los archivos:

   - Deben ser exactamente: `raioz-<os>-<arch>`
   - Ejemplo: `raioz-linux-amd64`

3. Si solo hay archivos comprimidos, el script los extraerá automáticamente

### El workflow falla

1. Verifica que el tag tenga el formato correcto: `v1.0.0`
2. Verifica que tengas permisos para crear releases
3. Revisa los logs en **Actions** → **Release**

## 📝 Notas

- Los binarios directos son opcionales pero recomendados para instalación más rápida
- El script de instalación intenta primero binarios directos, luego archivos comprimidos
- GoReleaser genera automáticamente los archivos comprimidos
- El workflow genera automáticamente los binarios directos
