🧠 Qué SÍ deberías agregar (mínimo indispensable)

Hay 3 cosas pequeñas que sí recomiendo antes de declararlo cerrado.

1️⃣ Documento de límites (muy importante)

Un archivo tipo:

/docs/
└── limits.md

Que diga:

Qué Raioz NO hace

Casos no soportados

Decisiones conscientes

Esto evita frustración futura.

2️⃣ Estado local mínimo

Un archivo:

~/.raioz/state.json

Para saber:

Qué servicios están activos

Qué modo

Qué commit / imagen

Última ejecución

No para telemetría.
Para consistencia.

3️⃣ Convención estricta de nombres

Ej:

raioz-<workspace>-<service>

## Esto ahorra horas de debugging.

---

Principio clave

El workspace es de Raioz, no del desarrollador.

El dev:

solo edita código

solo corre comandos Raioz

Raioz:

decide rutas

decide contenedores

decide redes

decide volúmenes

Estructura real en disco

Raioz mantiene un workspace global (una sola vez):

/opt/raioz/
├── workspaces/
│ └── billing-platform/
│ ├── local/
│ │ ├── users/
│ │ └── payments/
│ ├── deps/
│ │ ├── auth/
│ │ └── orders/
│ ├── infra/
│ └── .state.json
├── cache/
└── bin/

Caso concreto paso a paso
Estado inicial

.raioz.json:

{
"services": {
"users": { "mode": "local" },
"payments": { "mode": "image" }
}
}

Raioz:

clona users en local/

payments corre como imagen

Escenario: ahora quieres trabajar en payments
Lo único que haces

Cambias el archivo:

"payments": {
"mode": "local",
"repo": "git@github.com:org/payments.git",
"branch": "feature/x"
}

Y ejecutas:

raioz up

Qué hace Raioz internamente

1️⃣ Detecta cambio de modo
2️⃣ Clona repo en:

/opt/raioz/workspaces/billing-platform/local/payments

3️⃣ Reconstruye contenedor
4️⃣ Reconfigura red y env
5️⃣ Mantiene los demás servicios intactos

👉 No tumbó nada innecesario.

¿Qué pasa si tú clonas el repo “a mano”?
Escenario

Clonas payments en:

~/dev/payments

Resultado

Raioz lo ignora.

Porque:

no está en su workspace

no cumple convenciones

no está versionado por Raioz

👉 Esto es intencional.

¿Por qué esto es bueno?

Evita rutas arbitrarias

Evita conflictos

Evita “funciona en mi máquina”

Permite idempotencia

¿Y cómo editas el código entonces?

Dos opciones (ambas válidas):

Opción 1 – Abres el repo dentro de /opt/raioz/...

IDE directo ahí.

Opción 2 – Symlink (opcional)

Raioz puede permitir:

raioz link payments ~/dev/payments

Pero esto es opcional, no obligatorio.

¿Dónde viven las dependencias en modo local?

Siempre en:

/opt/raioz/workspaces/<workspace>/local/<service>

Nunca:

en el home

en carpetas sueltas

duplicadas

¿Qué pasa si borro la carpeta local?

Nada grave.

Raioz en el próximo up:

detecta ausencia

vuelve a clonar

reconstruye

Regla de oro (importante)

Raioz no “detecta repos externos”.
Solo gestiona lo que él mismo creó.

Eso garantiza consistencia.

--

1️⃣ Respuesta directa

Sí, es posible,
pero NO debe hacerse reemplazando “automágicamente” el servicio solo porque apareció otro repo corriendo.

La solución correcta NO es “pisar”,
es resolver por identidad y precedencia explícita.

Y esto es importante:
👉 hacerlo automático sin una señal explícita rompe reproducibilidad.

2️⃣ El error conceptual a evitar (muy importante)

La tentación sería:

“Si detecto que hay otro servicio con el mismo nombre/ID, lo reemplazo”

❌ Esto es peligroso porque:

rompe determinismo

genera entornos distintos entre devs

introduce efectos colaterales invisibles

hace debugging imposible

Raioz no debe reaccionar a cosas externas por defecto.

3️⃣ El enfoque correcto: identidad + override explícito (sin tocar configs)

La solución buena es esta:

Raioz decide por IDENTIDAD, no por ubicación.

Cada microservicio tiene:

un serviceId (estable)

un source (raioz-managed o external)

una precedence clara

4️⃣ Cómo se ve esto en la práctica (sin cambiar .raioz.json)
Estado inicial

Proyecto A:

{
"services": {
"billing": { "mode": "local" },
"users": { "mode": "git" },
"orders": { "mode": "git" }
}
}

Raioz clona:

/opt/raioz/workspaces/proj/deps/orders

5️⃣ Ahora el nuevo escenario

Te asignan orders.

Tú:

clonas manualmente orders

lo corres localmente

NO quieres tocar ningún .raioz.json

Perfecto.
Aquí entra el concepto clave:

6️⃣ Registro explícito de override (no implícito)

Raioz necesita una señal explícita, pero no en el repo.

Ejemplo de comando:
raioz override orders --path ~/dev/orders

Esto:

NO toca .raioz.json

NO modifica repos

NO rompe reproducibilidad

NO pisa nada automáticamente

7️⃣ Qué hace Raioz internamente

Detecta que orders ya existe como dependencia

Registra un override local en:

~/.raioz/overrides.json

Ejemplo:

{
"orders": {
"path": "/home/user/dev/orders",
"mode": "local",
"source": "external"
}
}

Aplica precedencia:

override > .raioz.json > default

Apaga el contenedor anterior

Levanta el nuevo

Mantiene:

puertos

red

env

nombres

👉 Nada se choca

8️⃣ ¿Y los dos .raioz.json?

Clave importante:

Solo existe UN .raioz.json activo por workspace.

El repo externo puede tener su .raioz.json,
pero Raioz no lo evalúa automáticamente.

Eso evita:

cascadas de configuración

conflictos circulares

magia oculta

9️⃣ ¿Por qué este enfoque es el correcto?

Porque cumple TODO:

Requisito Cumple
No tocar configs ✅
No choques ✅
No magia ✅
Reversible ✅
Determinista ✅
Escalable ✅
10️⃣ Regla de oro (importantísima)

Raioz nunca “detecta” overrides.
Los overrides se DECLARAN.

Esto es lo que separa una herramienta seria
de un sistema impredecible.

11️⃣ Frase para cerrar con el equipo

“Si quieres tomar control de un microservicio, lo declaras como override local.
Raioz respeta eso sin tocar ningún repo.”

12️⃣ Conclusión clara

✔ Sí se puede

❌ No debe ser automático

✅ Se resuelve con override explícito fuera del repo

✅ No se cambia ningún .raioz.json

✅ No hay choques

✅ El sistema sigue siendo predecible

---

Escenario real (bien planteado)

Tienes esta topología:

billing
├── users
└── orders
├── users
└── products ← dependencia que billing NO conoce

Situación:

billing es tu proyecto principal

orders es ahora un microservicio que tú mantienes

orders tiene sus propias dependencias

No quieres:

duplicar servicios

conflictos

cascadas raras

tocar configs existentes

❌ El error que NO debe cometer Raioz

“Leer automáticamente las dependencias del .raioz.json de orders
y fusionarlas con las del proyecto principal”

❌ Esto rompe:

control

predictibilidad

ownership

reproducibilidad

👉 Raioz NO hace resolución recursiva automática.

✅ El principio correcto (clave)

Solo existe UN grafo de dependencias activo:
el del workspace raíz.

Los microservicios no traen su mundo consigo.

Cómo se resuelve correctamente
1️⃣ El .raioz.json raíz manda

Ejemplo del proyecto billing:

{
"services": {
"billing": { "mode": "local" },
"users": { "mode": "git" },
"orders": { "mode": "local" },
"products": { "mode": "git" }
}
}

👉 Aunque products “pertenezca” conceptualmente a orders,
el workspace decide.

2️⃣ ¿Y si no declaras products?

Raioz tiene dos comportamientos válidos (elige uno como estándar):

Opción A – Falla explícita (recomendado)
orders requires service "products" but it is not defined in workspace

✔ Evita magia
✔ Evita servicios invisibles

Opción B – Stub/Missing mode (opcional)
"products": {
"mode": "missing"
}

Raioz:

no levanta nada

deja el endpoint apuntando a nowhere

útil para mocks

¿Qué pasa con users (dependencia compartida)?
Regla clara:

Una dependencia compartida se levanta UNA sola vez por workspace.

Entonces:

billing usa users

orders usa users

Raioz levanta un solo users

No hay duplicación.
No hay dos versiones.
No hay conflicto.

¿Y si orders necesita otra versión de users?

⚠️ Ese es un caso NO soportado en el mismo workspace.

Soluciones correctas:

workspace separado

perfil separado

o actualizar contrato

Esto es una limitación consciente, no un bug.

Cómo se cablean las dependencias

Raioz expone los servicios por nombre lógico, no por puerto duro.

Ejemplo:

USERS_BASE_URL=http://users:3000
PRODUCTS_BASE_URL=http://products:3000

Así:

no importa quién depende de quién

no importa quién es raíz

no hay ciclos peligrosos

¿Y el .raioz.json dentro de orders?

Punto importante:

Raioz lo ignora por defecto.

Ese archivo:

sirve para cuando orders sea el proyecto raíz

sirve para CI

sirve para documentación

NO se auto-importa

Esto evita:

dependencias fantasma

cascadas infinitas

conflictos de ownership

Resumen en una tabla
Caso ¿Funciona? Cómo
Dependencia compartida (users) ✅ Una sola instancia
Dependencia exclusiva (products) ✅ Declarada en raíz
Dependencias recursivas automáticas ❌ No permitido
Versiones distintas del mismo servicio ❌ Workspace separado
Override local de orders ✅ Sin romper dependencias
La regla de oro (memorízala)

El workspace define el grafo completo.
Los microservicios no arrastran dependencias.

Esta regla es lo que hace que:

Raioz sea predecible

el stack no se rompa

el equipo confíe

Frase perfecta para el equipo / tech lead

“Cada proyecto define su mundo.
Los microservicios se adaptan al workspace, no al revés.”

Conclusión final

✔ El escenario está cubierto
✔ No hay choques
✔ No hay magia
✔ Hay límites claros
✔ El sistema sigue simple

---

Idea central (importante)

Raioz puede ayudar a descubrir dependencias faltantes,
pero NUNCA debe incorporarlas al workspace sin consentimiento explícito.

La palabra clave es: conciencia.

Flujo correcto: Dependency Resolution Assist
1️⃣ Lanzamiento normal
raioz up

Raioz evalúa el grafo del .raioz.json raíz.

2️⃣ Detecta dependencia faltante

Ejemplo:

orders declara que necesita products

products no está en el workspace raíz

Raioz NO la levanta automáticamente.

3️⃣ Modo asistido (interactivo o dry-run)

Raioz muestra algo como:

⚠ Dependency missing: products
Required by: orders

Found definition in:
/opt/raioz/workspaces/billing/local/orders/.raioz.json

Differences:
root: (not defined)
orders: mode=git, repo=org/products, branch=main

Choose action:
[1] Add products to root workspace
[2] Ignore (service will fail)
[3] Add as stub/missing

👉 Nada pasa hasta que el usuario decide.

4️⃣ Si el usuario elige “Add to root”

Raioz hace tres cosas clave:

a) Copia SOLO la definición necesaria

No importa todo el .raioz.json de orders.

Solo:

"products": {
"mode": "git",
"repo": "git@github.com:org/products.git",
"branch": "main"
}

b) Marca el origen

Ejemplo en el root:

"products": {
"mode": "git",
"repo": "git@github.com:org/products.git",
"branch": "main",
"origin": "orders",
"addedBy": "raioz assist"
}

Esto es oro puro para trazabilidad.

c) Registra el evento

En:

~/.raioz/audit.log

Ejemplo:

2026-01-08 Added dependency "products" to workspace (origin: orders)

5️⃣ ¿Qué pasa si YA existe en root?

Raioz entra en modo diff, nunca override automático.

Ejemplo:

⚠ Dependency conflict detected: users

Root definition:
branch: release/1.2

Orders requires:
branch: main

Choose:
[1] Keep root (recommended)
[2] Replace root
[3] Abort

👉 Siempre consciente. Siempre explícito.

6️⃣ ¿Y si luego cambia el .raioz.json de orders?

Raioz puede avisar, no actuar.

Ejemplo:

⚠ Dependency drift detected:
orders requires products@develop
root uses products@main

✔ No cambia nada
✔ No rompe nada
✔ Informa

Por qué este enfoque es correcto (muy importante)
✔ Evita cascadas automáticas

Nada entra sin permiso.

✔ Mantiene un solo grafo

El root sigue siendo la verdad.

✔ Hace visibles las decisiones

No hay magia oculta.

✔ Permite evolución consciente

El sistema acompaña, no impone.

Qué NO debe hacer Raioz (regla estricta)

❌ No auto-importar
❌ No merge silencioso
❌ No resolver conflictos solo
❌ No cambiar versiones sin aviso

Resumen del comportamiento
Situación Acción
Falta dependencia Avisar
Definición encontrada Mostrar
Diferencia detectada Diff + prompt
Aprobación del usuario Aplicar
Cambio posterior Notificar
Frase clave para el equipo

“Raioz no decide por ti.
Te muestra el impacto y te deja elegir.”

---

1️⃣ Validación general

👉 Sí, el concepto de workspace como unidad principal es correcto.
👉 Sí, Raioz debe resolver un grafo completo de dependencias.
👉 Sí, Raioz puede asistir en conflictos y decidir precedencias.

Pero hay una frase tuya que marca el límite:

“automáticamente convierte el proyecto que se despliega en el principal”

⚠️ Ahí hay riesgo si no se controla bien.

2️⃣ Lo que está BIEN en tu planteamiento
✅ Workspace como verdad absoluta

Correctísimo:

“decidir un workspace, y un nombre de proyecto”

El workspace es:

la frontera de aislamiento

el namespace

la unidad de verdad

✅ Dependencias declaradas por cada microservicio

Correcto:

cada microservicio puede declarar sus dependencias

Raioz puede leerlas

eso permite asistencia inteligente

✅ Construir un .raioz.json raíz único

También correcto, con un matiz:

✔ Existe un solo grafo activo
✔ Centralizado
✔ Visible
✔ Auditable

✅ Avisar conflictos y pedir decisión

Esto es excelente diseño:

“avisar al usuario de cuál dependencia quiere usar”

Nunca automático. Siempre explícito.

✅ Registro y reversibilidad

Muy bien:

registrar decisiones

poder volver al estado anterior

Esto es nivel herramienta madura.

3️⃣ Donde hay que corregir (muy importante)

Aquí vienen los dos ajustes clave.

❌ Ajuste 1: El .raioz.json raíz NO debe ser generado “automáticamente” y persistido sin control

Tú dices:

“raioz arma el .raioz.json raíz guardado en la configuración general”

⚠️ Problema:

pierdes reproducibilidad

el workspace deja de ser declarativo

dos devs podrían tener roots distintos

✔ Corrección

El root debe ser:

derivado + confirmado + persistido conscientemente

Es decir:

Raioz propone

El usuario acepta

Se guarda como estado del workspace, no como verdad implícita

❌ Ajuste 2: No “convertir automáticamente” proyectos en principales

Esta parte:

“automáticamente convierte el proyecto que se despliega en el principal”

⚠️ Esto rompe:

determinismo

predictibilidad

confianza del equipo

✔ Corrección conceptual

Regla clave:

El workspace tiene un principal explícito.
No cambia sin que el usuario lo declare.

Ejemplo:

raioz up billing

vs

raioz up orders --set-root

Sin --set-root, no hay cambio de jerarquía.

4️⃣ El modelo final recomendado (ajustado y correcto)

Aquí está el modelo cerrado, coherente y seguro.

🧠 Modelo definitivo de Raioz
1️⃣ Workspace
Workspace = frontera + estado + decisiones

Tiene:

nombre

root project

grafo activo

overrides

audit log

2️⃣ Root Project (explícito)

Uno solo.

Declarado por:

primera creación

o --set-root

Nunca implícito.

3️⃣ Resolución de dependencias (asistida)

Cuando haces:

raioz up

Raioz:

Lee .raioz.json del root

Lee dependencias declaradas por servicios locales

Construye un grafo candidato

Detecta:

faltantes

duplicados

conflictos

Presenta decisiones

Nada se aplica sin aprobación.

4️⃣ Conflictos entre proyectos

Si aparece otro proyecto con dependencias superpuestas:

Raioz NO pisa.

Muestra:

Conflict detected: users

Current root uses:
users@release/1.2

New project requires:
users@main

Choose:
[1] Keep root
[2] Override (record decision)
[3] Abort

5️⃣ Ignorar ≠ borrar

Si eliges ignorar:

se registra en:

~/.raioz/ignore.json

no se pierde info

es reversible

6️⃣ Reversión automática (esto lo pensaste muy bien)

Esto que dijiste es correctísimo:

“si al volver a usar up y no encontrar el local lo devuelve al git”

✔ Sí.

Regla:

override existe → se usa

override no existe → fallback al root

Totalmente sano.

📌 La regla de oro final (memorízala)

Raioz propone, el usuario decide, el workspace recuerda.

Si respetas esto:

no hay magia

no hay miedo

no hay caos

---

1️⃣ Conceptos base (para entender el flujo)

Antes del flujo, estos son los pilares del sistema:

🔹 Workspace

Es el contexto raíz donde viven uno o varios microservicios.

Ejemplo:

~/workspaces/empresa-x/

🔹 Proyecto (microservicio)

Cada microservicio:

Tiene su propio repo

Tiene su propio .raioz.json

Define sus dependencias directas

🔹 Configuración Raíz de Raioz

Existe UN SOLO archivo raíz generado por Raioz:

NO vive dentro del repo

Vive en la configuración global del workspace

Ejemplo:

~/.raioz/workspaces/empresa-x/raioz.root.json

Este archivo representa:
👉 el árbol completo de dependencias resueltas

2️⃣ Flujo lógico completo de funcionamiento
🟢 Paso 1: Selección de workspace
raioz workspace use empresa-x

Raioz:

Verifica si el workspace existe

Si no existe → lo crea

Carga su raioz.root.json si existe

🟢 Paso 2: raioz up de un proyecto

Ejemplo:

raioz up billing

Raioz ejecuta:

2.1 Identificación del proyecto

Lee el .raioz.json del proyecto billing

Extrae:

id

name

version

dependencias

Ejemplo:

{
"id": "billing",
"dependencies": ["users", "orders"]
}

🟢 Paso 3: Resolución de dependencias (recursiva)

Raioz construye el árbol completo:

Ejemplo:

billing
├── users
└── orders
├── users
└── products

Reglas:

Las dependencias duplicadas se unifican por ID

No se duplican carpetas

Se mantiene un solo nodo lógico por microservicio

🟢 Paso 4: Evaluación de estado de cada dependencia

Para cada microservicio del árbol:

Raioz evalúa:

Estado Descripción
git Clonado automáticamente desde repo
local Clonado manualmente por el usuario
override Reemplaza temporalmente al git
ignored Existe pero no se usa
missing No existe local ni git
🟢 Paso 5: Conflictos de dependencias (caso clave)

Ejemplo:

orders ya existe como dependencia git

El usuario clona manualmente orders y hace raioz up orders

Raioz detecta:

⚠️ Conflicto: el microservicio orders ya existe

Se muestra al usuario:
orders ya existe en el workspace

1. Usar versión actual (git)
2. Reemplazar por versión local (override)
3. Mantener ambas (una activa, otra ignorada)

🟢 Paso 6: Override consciente

Si el usuario elige reemplazar:

Raioz:

Marca el orders local como:

source: local
overridden: true

El orders git:

source: git
status: ignored

📌 No se modifica ningún .raioz.json
📌 Todo queda registrado en el raioz.root.json

🟢 Paso 7: Registro de decisiones (auditabilidad)

El raioz.root.json guarda:

{
"projects": {
"orders": {
"activeSource": "local",
"previousSource": "git",
"reason": "manual override",
"timestamp": "2026-01-08T14:20:00"
}
}
}

👉 Esto permite:

Saber qué cambió

Saber que fue una decisión consciente

Poder revertir

3️⃣ Qué pasa si falta una dependencia

Ejemplo:

raioz up billing

Y products no existe.

Raioz:

products no existe en el workspace

¿Deseas agregarlo?
✔ Clonar desde git
✔ Registrar como dependencia

Si acepta:

Clona

Agrega al árbol

Actualiza raioz.root.json

4️⃣ Cambio posterior en dependencias

Si luego:

Cambia orders

Cambia users

Raioz:

Detecta diferencia de hashes / commits

Muestra:

orders cambió respecto al estado raíz
¿Confirmas este cambio?

Esto mantiene:
✅ control
✅ trazabilidad
✅ cero sorpresas

5️⃣ Rollback automático

Si luego:

raioz up billing

Y el override local ya no existe

Raioz:

Detecta que el override no está disponible

Restaura automáticamente:

orders → git original

Actualiza el registro

6️⃣ Resultado final del sistema

✔ No hay choques
✔ No se cambian repos
✔ No se tocan .raioz.json
✔ Overrides conscientes
✔ Árbol completo unificado
✔ Rollback automático
✔ Escala a N microservicios
