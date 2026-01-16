# Guía de Instalación de Raioz

Esta guía explica cómo instalar Raioz tanto para usuarios finales como para desarrolladores.

## 👥 Para Usuarios Finales

### Instalación desde GitHub (Recomendada)

Instala raioz directamente desde GitHub releases:

```bash
curl -fsSL https://raw.githubusercontent.com/raioz/raioz/main/install.sh | sh
```

**Qué hace el script:**
1. Detecta tu sistema operativo (Linux/macOS) y arquitectura
2. Descarga el binario pre-compilado desde GitHub releases
3. Instala raioz en `/usr/local/bin` (requiere sudo)
4. Verifica que Docker y Git estén instalados
5. Configura los permisos necesarios

**Nota:** Si no tienes permisos para `/usr/local/bin`, puedes especificar otro directorio:

```bash
INSTALL_DIR=$HOME/.local/bin curl -fsSL https://raw.githubusercontent.com/raioz/raioz/main/install.sh | sh
```

## 👨‍💻 Para Desarrolladores

### Instalación desde Código Local

Si estás desarrollando raioz o quieres probar cambios locales:

```bash
# 1. Compilar el binario localmente
make build

# 2. Instalar desde el binario local
make install
```

**Qué hace `make install`:**
1. Compila el binario desde el código fuente (`make build`)
2. Detecta automáticamente el binario local (`./raioz`)
3. Usa el script `install.sh` en modo desarrollo
4. El script detecta el binario local y lo instala (no descarga de GitHub)
5. Instala el binario en `/usr/local/bin`

**Comportamiento inteligente:**
- Si hay un binario `raioz` en el directorio actual o donde está `install.sh`, usa ese binario
- Si no hay binario local, descarga desde GitHub releases
- Esto permite probar cambios locales antes de subir a GitHub

### Compilación Manual

Si prefieres compilar e instalar manualmente:

```bash
# Compilar
make build

# Instalar manualmente
sudo cp raioz /usr/local/bin/
sudo chmod +x /usr/local/bin/raioz
```

## 🔧 Preparar Release (Para Mantenedores)

Para preparar un release que los usuarios puedan instalar:

```bash
# 1. Compilar y preparar el instalador
make release-installer

# 2. Esto prepara el script install.sh
# 3. Sube el binario compilado a GitHub releases
# 4. Los usuarios pueden instalar con: curl | sh
```

**Nota:** Ver `RELEASE.md` para instrucciones completas de cómo crear releases.

## 📋 Requisitos Previos

### Para Usuarios

- **Docker**: Instalado y corriendo ([instalar Docker](https://docs.docker.com/get-docker/))
- **Git**: Instalado ([instalar Git](https://git-scm.com/downloads))
- **Bash**: Disponible (viene con Linux/macOS)

### Para Desarrolladores

Además de los requisitos de usuarios:
- **Go 1.21+**: Para compilar el código fuente
- **Make**: Para usar los comandos del Makefile

## ✅ Verificar Instalación

Después de instalar, verifica que funcione:

```bash
raioz version
raioz --help
```

Si el comando no se encuentra, asegúrate de que `/usr/local/bin` esté en tu PATH:

```bash
# Para bash/zsh
echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc  # o ~/.zshrc
source ~/.bashrc  # o ~/.zshrc
```

## 🔄 Actualizar Raioz

### Para Usuarios

Simplemente ejecuta el instalador nuevamente:

```bash
curl -fsSL https://raw.githubusercontent.com/raioz/raioz/main/install.sh | sh
```

### Para Desarrolladores

```bash
# Recompilar e instalar
make install
```

## ❓ Solución de Problemas

### El script no detecta el binario local

Asegúrate de que:
- El binario `raioz` existe en el mismo directorio que `install.sh`, o
- El binario `raioz` existe en el directorio actual (donde ejecutas `make install`)

### Error de permisos

Si no tienes permisos para `/usr/local/bin`, usa otro directorio:

```bash
INSTALL_DIR=$HOME/.local/bin ./install.sh
```

Y asegúrate de agregarlo a tu PATH.

### Docker no está corriendo

El script verifica que Docker esté corriendo. Si no lo está:

```bash
# Linux
sudo systemctl start docker

# macOS
open -a Docker
```

## 📚 Recursos Adicionales

- [README.md](./README.md) - Documentación principal
- [RELEASE.md](./RELEASE.md) - Guía para crear releases
- [docs/COMMANDS.md](./docs/COMMANDS.md) - Documentación de comandos
