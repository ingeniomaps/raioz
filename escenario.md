.
Escenario real

Proyecto con 6 microservicios:

auth

users

payments

orders

notifications

search

Tú eres responsable de:

users

payments

Quieres:

Modificar ambos

Tenerlos corriendo al mismo tiempo

Sin romper el resto

Cómo lo maneja Raioz (concepto clave)

Raioz distingue 3 estados por microservicio:

Local (editable)
→ repo clonado, código vivo, hot reload

Remoto / imagen
→ corre como imagen Docker (estable)

Desactivado
→ ni clona ni levanta

Esto se define una sola vez en el deps.json.

Ejemplo de deps.json
{
"project": "billing-platform",

"services": {
"users": {
"mode": "local",
"repo": "git@github.com:org/users.git",
"branch": "feature/refactor"
},
"payments": {
"mode": "local",
"repo": "git@github.com:org/payments.git",
"branch": "feature/new-flow"
},
"auth": {
"mode": "image",
"image": "org/auth:stable"
},
"orders": {
"mode": "image",
"image": "org/orders:stable"
},
"notifications": {
"mode": "image",
"image": "org/notifications:stable"
},
"search": {
"mode": "disabled"
}
}
}

Qué pasa en la práctica
Cuando ejecutas:
raioz up

Raioz hace:

Clona users y payments

Corre ambos en modo desarrollo

Levanta los otros 3 como imágenes

Ignora search

Todo corre en paralelo

Cada servicio con:

su puerto

su red

su volumen

su env

👉 No hay que cambiar config al alternar entre servicios.

¿Y si mañana solo quiero tocar payments?

Dos opciones sin conflictos:

Opción A – Seguir con ambos vivos

No haces nada.
Simplemente trabajas en payments.

Opción B – Perfil de trabajo (opcional)

Raioz puede soportar perfiles:

raioz up --profile payments-only

Con un perfil que diga:

{
"services": {
"users": "image",
"payments": "local"
}
}

Pero esto es opcional, no obligatorio.

Punto CLAVE: no hay colisión

Raioz evita conflictos porque:

Puertos se asignan de forma determinística

Volúmenes están namespaced por proyecto

Contenedores tienen prefijos únicos

.env está centralizado y versionado

Ejemplo:

raioz-billing-users
raioz-billing-payments

Nunca chocan con:

raioz-otroproyecto-users

Comparación con el dolor actual
Hoy sin Raioz

Cambias docker-compose

Apagas servicios

Rompes algo sin querer

Te peleas con puertos

Copias envs

Rezas

Con Raioz

Declaras una vez

Trabajas tranquilo

Todo vive junto

Sin pisarse

Sin magia oculta

Resumen claro para el equipo

Raioz no fuerza a trabajar de a uno.
Permite trabajar en varios microservicios al mismo tiempo,
mientras el resto del sistema se mantiene estable.

---

Principio clave
👉 “No todo servicio que levanto es un servicio que controlo”

Raioz separa explícitamente:

Servicios bajo tu control

Servicios dependientes, solo lectura

Servicios completamente externos

Esto es normal en sistemas reales.

Tu escenario (muy real)

Tienes:

users → tu equipo

payments → tu equipo

auth → otro equipo

orders → otro equipo

notifications → otro equipo

No hay registry.
Todo vive en Git.
No deberías modificar auth ni orders.

¿Qué hace Raioz con eso?
1️⃣ Raioz NO asume que vas a tocar el repo

Clonar ≠ editar.

Clonar significa:

Leer código

Construir imagen local

Ejecutar servicio

Nada más.

Configuración explícita: modo readonly

Ejemplo de deps.json:

{
"services": {
"users": {
"mode": "local",
"repo": "git@github.com:org/users.git",
"branch": "feature/x"
},
"payments": {
"mode": "local",
"repo": "git@github.com:org/payments.git",
"branch": "feature/y"
},
"auth": {
"mode": "git",
"repo": "git@github.com:org/auth.git",
"branch": "main",
"access": "readonly",
"build": {
"dockerfile": "Dockerfile"
}
},
"orders": {
"mode": "git",
"repo": "git@github.com:org/orders.git",
"branch": "main",
"access": "readonly"
}
}
}

¿Qué significa readonly en la práctica?

El repo se clona en una carpeta protegida

Raioz:

no hace git checkout automático

no hace git pull sin pedirlo

no sobreescribe nada

El servicio corre como contenedor

Si rompes algo → se recrea

👉 Tu stack no se contamina

¿Por qué NO rompe el stack?

Porque:

✔ No hay side effects

No hay commits

No hay push

No hay cambios persistentes

✔ Infraestructura es efímera

Contenedores son descartables

Volúmenes pueden ser read-only

✔ Configuración centralizada

.env vive fuera del repo

No se escriben configs internas

¿Y si alguien modifica el repo por error?

Dos protecciones:

Protección 1 – Volumen read-only
volumes:

- ./auth:/app:ro

Docker literalmente impide escribir.

Protección 2 – Workspace separado
/opt/raioz/
├── workspaces/
│ ├── billing/
│ │ ├── local/ ← editable
│ │ └── readonly/ ← dependencias

¿Esto es peor que un registry?

No. Es lo mismo conceptualmente:

Registry Git readonly
Imagen versionada Commit hash
Pull Clone
Inmutable Inmutable
Difícil debug Debuggable

👉 Git como source registry es válido en desarrollo.

¿Y si mañana SÍ hay registry?

Raioz no cambia.

Solo cambias:

"auth": {
"mode": "image",
"image": "org/auth:1.12.3"
}

El resto queda igual.

Mensaje clave para el equipo

Raioz no te obliga a controlar servicios que no son tuyos.
Los consume como dependencias, de forma segura y aislada.

En resumen

✔ No rompe el stack

✔ Respeta ownership

✔ No requiere registry

✔ Escala a registry cuando exista

✔ Reduce fricción entre equipos
