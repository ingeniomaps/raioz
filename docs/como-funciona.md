Excelente, esta es la pregunta correcta, porque aquí se define si esto va a ser adoptado o rechazado por el equipo.

Voy a responder paso a paso, separando qué NO debe pasar, qué SÍ, y cómo se ve en la práctica para un dev nuevo.

❌ Lo que NO debe pasar

Para que esto funcione de verdad, estas cosas no pueden ocurrir:

❌ No se clona un “repo del orquestador”

❌ No se copian archivos dentro de cada microservicio

❌ No se obliga a modificar Dockerfiles existentes

❌ No se requiere tocar .env en cada repo

❌ No se impone una estructura interna en los servicios

👉 Si pasa algo de esto, la herramienta fracasa.

✅ Cómo SÍ debe funcionar (modelo correcto)
1️⃣ El orquestador es un binario, no un proyecto

Se instala una sola vez por máquina.

curl -fsSL https://raioz.dev/install | sh

Resultado:

raioz version

✔ No hay repos nuevos
✔ No hay dependencias
✔ No hay configuración global obligatoria

2️⃣ Cada proyecto solo tiene UN archivo

En el repo del proyecto (o raíz del frontend, o repo “padre”):

deps.json

Nada más.

👉 Ese archivo vive con el proyecto, se versiona y se revisa en PR.

3️⃣ Los microservicios NO saben que Raioz existe

Esto es clave.

Los microservicios:

siguen teniendo su propio Dockerfile.dev

siguen corriendo con docker compose up

siguen siendo independientes

Raioz no invade su diseño.

4️⃣ ¿Qué hace raioz up realmente?

Cuando un dev corre:

raioz up

Raioz:

Lee deps.json

Crea (si no existe):

/opt/raioz-proyecto/

Clona SOLO los repos listados

Checkout de ramas declaradas

Verifica imágenes Docker necesarias

Lee variables de entorno desde carpeta compartida

Genera un docker-compose.generated.yml

Ejecuta Docker

Guarda estado local

👉 Todo fuera de los repos.

5️⃣ ¿Dónde viven los .env?

No en los microservicios.

/opt/raioz-proyecto/env/
├── global.env
├── services/
│ ├── users-service.env
│ └── payments-service.env
└── projects/
└── billing-dashboard.env

✔ Centralizado
✔ Reutilizable
✔ Evita duplicación

6️⃣ ¿Hay que copiar archivos a los micros?

👉 NO.

Los microservicios solo requieren:

Dockerfile.dev

comandos estándar (npm run dev, go run, etc.)

Raioz no inyecta nada dentro del repo.

7️⃣ ¿Y si un microservicio no tiene Dockerfile.dev?

Dos opciones limpias:

Opción A (recomendada)

Agregar Dockerfile.dev al repo (una sola vez).

Opción B (fallback)

Definir en deps.json:

"docker": {
"command": "npm run dev"
}

Raioz genera un wrapper temporal, no modifica el repo.

8️⃣ ¿Qué pasa si no tengo permisos para /opt?

Raioz detecta el sistema y usa fallback:

Linux/macOS:

~/.raioz/

Windows:

%USERPROFILE%\.raioz\

Transparente para el dev.

🧠 Flujo real de onboarding (ejemplo)
Dev nuevo
git clone git@github.com:raioz/billing-dashboard.git
cd billing-dashboard
raioz up

Salida:

✔ Workspace creado
✔ users-service clonado
✔ payments-service usando imagen
✔ mongo levantado
✔ rabbit levantado
✔ entorno listo

Tiempo: 3–5 minutos

🎯 Por qué este diseño es el correcto

Porque:

no rompe repos existentes

no añade fricción

no impone estructura

no exige aprendizaje adicional

no centraliza lógica de negocio

👉 Raioz vive al costado, no encima.

🏁 Resumen en una frase

El orquestador se instala una vez, el proyecto solo define dependencias, y los microservicios no se enteran de nada.
