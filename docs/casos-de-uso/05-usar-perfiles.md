# Caso de Uso 5: Usar Perfiles (Frontend/Backend)

## 📋 Descripción

Un desarrollador solo necesita trabajar en una parte del stack (por ejemplo, solo frontend o solo backend). Los perfiles permiten filtrar servicios para levantar solo lo necesario.

## 🎯 Objetivo

Permitir que un desarrollador:
- Levante solo los servicios relevantes para su trabajo
- Reduzca el consumo de recursos
- Acelere el tiempo de inicio
- Mantenga el stack mínimo necesario

## 🔄 Flujo Completo

### Configuración en .raioz.json

```json
{
  "services": {
    "frontend": {
      "source": { ... },
      "docker": { ... },
      "profiles": ["frontend"]  // ← Solo se incluye con --profile frontend
    },
    "api": {
      "source": { ... },
      "docker": { ... },
      "profiles": ["backend"]  // ← Solo se incluye con --profile backend
    },
    "worker": {
      "source": { ... },
      "docker": { ... },
      "profiles": ["backend"]  // ← Solo se incluye con --profile backend
    },
    "database": {
      "source": { ... },
      "docker": { ... }
      // Sin profiles = siempre incluido
    }
  }
}
```

### Comandos Disponibles

#### Levantar Solo Frontend

```bash
raioz up --profile frontend
```

**Qué se incluye:**
- ✅ Servicios con `profiles: ["frontend"]`
- ✅ Servicios sin `profiles` (siempre incluidos)
- ✅ Infra siempre incluida
- ❌ Servicios con otros perfiles

#### Levantar Solo Backend

```bash
raioz up --profile backend
```

**Qué se incluye:**
- ✅ Servicios con `profiles: ["backend"]`
- ✅ Servicios sin `profiles` (siempre incluidos)
- ✅ Infra siempre incluida
- ❌ Servicios con otros perfiles

#### Levantar Todo (Sin Perfil)

```bash
raioz up
```

**Qué se incluye:**
- ✅ Todos los servicios (sin importar perfiles)
- ✅ Infra siempre incluida

## 🎯 Ejemplo Real

### Configuración Completa

**.raioz.json:**
```json
{
  "schemaVersion": "1.0",
  "project": {
    "name": "billing-platform",
    "network": "billing-network"
  },
  "services": {
    "frontend": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/frontend.git",
        "branch": "develop",
        "path": "services/frontend"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3000:3000"]
      },
      "profiles": ["frontend"]
    },
    "api": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/api.git",
        "branch": "develop",
        "path": "services/api"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3001:3000"],
        "dependsOn": ["database"]
      },
      "profiles": ["backend"]
    },
    "worker": {
      "source": {
        "kind": "git",
        "repo": "git@github.com:org/worker.git",
        "branch": "main",
        "path": "services/worker"
      },
      "docker": {
        "mode": "dev",
        "ports": ["3002:3000"],
        "dependsOn": ["database", "redis"]
      },
      "profiles": ["backend"]
    },
    "auth-service": {
      "source": {
        "kind": "image",
        "image": "org/auth-service",
        "tag": "1.5.2"
      },
      "docker": {
        "mode": "prod",
        "ports": ["3003:3000"]
      }
      // Sin profiles = siempre incluido
    }
  },
  "infra": {
    "database": {
      "image": "postgres",
      "tag": "15"
    },
    "redis": {
      "image": "redis",
      "tag": "7"
    }
  }
}
```

### Escenario 1: Desarrollador Frontend

**Desarrollador:** Ana (Frontend)

**Comando:**
```bash
raioz up --profile frontend
```

**Qué se levanta:**
- ✅ `frontend` (tiene perfil frontend)
- ✅ `auth-service` (sin perfil, siempre incluido)
- ✅ `database` (infra, siempre incluida)
- ✅ `redis` (infra, siempre incluida)
- ❌ `api` (tiene perfil backend, excluido)
- ❌ `worker` (tiene perfil backend, excluido)

**Resultado:**
- Solo levanta lo necesario para frontend
- Menos recursos consumidos
- Inicio más rápido
- Frontend puede trabajar con auth-service y base de datos

### Escenario 2: Desarrollador Backend

**Desarrollador:** Carlos (Backend)

**Comando:**
```bash
raioz up --profile backend
```

**Qué se levanta:**
- ✅ `api` (tiene perfil backend)
- ✅ `worker` (tiene perfil backend)
- ✅ `auth-service` (sin perfil, siempre incluido)
- ✅ `database` (infra, siempre incluida)
- ✅ `redis` (infra, siempre incluida)
- ❌ `frontend` (tiene perfil frontend, excluido)

**Resultado:**
- Solo levanta lo necesario para backend
- Menos recursos consumidos
- Inicio más rápido
- Backend puede trabajar con auth-service, base de datos y redis

### Escenario 3: Desarrollador Full-Stack

**Desarrollador:** Luis (Full-Stack)

**Comando:**
```bash
raioz up
```

**Qué se levanta:**
- ✅ Todos los servicios (sin importar perfiles)
- ✅ Infra completa

**Resultado:**
- Stack completo
- Puede trabajar en cualquier parte
- Más recursos consumidos
- Inicio más lento

## 🔍 Detalles Técnicos

### Filtrado por Perfil

**Función:** `config.FilterByProfile(deps, profile)`

**Lógica:**
1. Crea nuevo `Deps` con servicios filtrados
2. Para cada servicio:
   - Si no tiene `profiles`: siempre incluido
   - Si tiene `profiles` y coincide con el perfil: incluido
   - Si tiene `profiles` y no coincide: excluido
3. Infra siempre incluida (no se filtra)

**Código:**
```go
for name, svc := range deps.Services {
    if len(svc.Profiles) == 0 {
        // Sin profiles = siempre incluido
        filtered.Services[name] = svc
    } else {
        // Verificar si el perfil coincide
        for _, p := range svc.Profiles {
            if p == profile {
                filtered.Services[name] = svc
                break
            }
        }
    }
}
```

### Perfiles Válidos

**Actualmente soportados:**
- `"frontend"`
- `"backend"`

**Extensibilidad:**
- Fácil agregar más perfiles en el futuro
- Validación en schema JSON

### Servicios Sin Perfil

**Comportamiento:**
- Siempre se incluyen
- No importa qué perfil se use
- Útiles para servicios compartidos

**Ejemplos:**
- `auth-service`: Usado por frontend y backend
- `database`: Infra compartida
- Servicios críticos que siempre deben estar

## 📊 Comparación: Con vs Sin Perfil

### Sin Perfil (Todo)

**Recursos:**
- CPU: ~40%
- Memoria: ~2GB
- Tiempo de inicio: 3-5 minutos
- Servicios: 5

**Ventajas:**
- Stack completo
- Puedes trabajar en cualquier parte

**Desventajas:**
- Más recursos
- Inicio más lento

### Con Perfil Frontend

**Recursos:**
- CPU: ~15%
- Memoria: ~800MB
- Tiempo de inicio: 1-2 minutos
- Servicios: 2-3

**Ventajas:**
- Menos recursos
- Inicio más rápido
- Solo lo necesario

**Desventajas:**
- No puedes trabajar en backend
- Algunos servicios no están disponibles

### Con Perfil Backend

**Recursos:**
- CPU: ~25%
- Memoria: ~1.2GB
- Tiempo de inicio: 2-3 minutos
- Servicios: 3-4

**Ventajas:**
- Menos recursos que todo
- Inicio más rápido
- Solo lo necesario

**Desventajas:**
- No puedes trabajar en frontend
- Algunos servicios no están disponibles

## 🎯 Casos de Uso Específicos

### Caso 1: Desarrollo Frontend Solo

**Escenario:** Ana solo trabaja en el frontend, el backend está en producción.

**Configuración:**
```json
"frontend": {
  "profiles": ["frontend"]
},
"api": {
  "source": {
    "kind": "image",
    "image": "org/api",
    "tag": "latest"
  }
  // Sin profiles = siempre incluido
}
```

**Comando:**
```bash
raioz up --profile frontend
```

**Resultado:**
- Frontend en modo dev (hot-reload)
- API como imagen estable
- Base de datos para desarrollo
- Stack mínimo para frontend

### Caso 2: Desarrollo Backend Solo

**Escenario:** Carlos solo trabaja en el backend, el frontend está en producción.

**Configuración:**
```json
"api": {
  "profiles": ["backend"]
},
"worker": {
  "profiles": ["backend"]
},
"frontend": {
  "source": {
    "kind": "image",
    "image": "org/frontend",
    "tag": "latest"
  }
  // Sin profiles = siempre incluido
}
```

**Comando:**
```bash
raioz up --profile backend
```

**Resultado:**
- API y worker en modo dev (hot-reload)
- Frontend como imagen estable
- Base de datos y redis para desarrollo
- Stack mínimo para backend

### Caso 3: Múltiples Perfiles

**Escenario:** Un servicio puede estar en múltiples perfiles.

**Configuración:**
```json
"shared-service": {
  "profiles": ["frontend", "backend"]
}
```

**Comportamiento:**
- Se incluye con `--profile frontend`
- Se incluye con `--profile backend`
- Se incluye sin perfil (todos)

## ⚠️ Consideraciones

### Dependencias

**Si un servicio con perfil depende de otro:**
- Raioz valida dependencias después del filtrado
- Si una dependencia está excluida, muestra error
- Debes incluir dependencias necesarias

**Ejemplo:**
```json
"api": {
  "profiles": ["backend"],
  "docker": {
    "dependsOn": ["database", "auth-service"]
  }
}
```

**Si `auth-service` tiene perfil `frontend`:**
- Error: dependencia excluida
- Solución: quitar perfil de `auth-service` o agregar a ambos perfiles

### Infra Siempre Incluida

**Comportamiento:**
- Infra nunca se filtra por perfiles
- Siempre se incluye
- Necesaria para que funcionen los servicios

**Razón:**
- Base de datos, redis, etc. son compartidos
- No tiene sentido excluirlos
- Siempre necesarios

### Cambiar de Perfil

**Si cambias de perfil:**
```bash
# Estás con perfil frontend
raioz up --profile frontend

# Cambias a backend
raioz down
raioz up --profile backend
```

**Qué pasa:**
- Se detienen servicios del perfil anterior
- Se levantan servicios del nuevo perfil
- Estado se actualiza

## 📝 Mejores Prácticas

1. **Asigna perfiles a servicios específicos**
   - Frontend → `["frontend"]`
   - Backend → `["backend"]`

2. **Deja sin perfil servicios compartidos**
   - Auth, database, etc.
   - Siempre necesarios

3. **Documenta qué servicios tienen qué perfiles**
   - En README del proyecto
   - En comentarios de `.raioz.json`

4. **Usa perfiles para optimizar recursos**
   - Menos servicios = menos recursos
   - Inicio más rápido

5. **Revisa dependencias al usar perfiles**
   - Asegúrate de que dependencias estén incluidas
   - Valida con `raioz check`

## 🔗 Comandos Relacionados

- `raioz up --profile {profile}`: Levantar con perfil específico
- `raioz status`: Ver qué servicios están corriendo
- `raioz check`: Verificar dependencias y alineación
- `raioz down`: Detener servicios (respeta perfiles activos)
