📌 Caso real: Proyecto “Billing Dashboard”
Contexto de la empresa

1 frontend (React)

5 microservicios backend

Infra compartida

Equipo de 6 devs

Cada dev toca 1–2 servicios máximo

Onboarding frecuente

🧱 Arquitectura real del proyecto
Microservicios
Servicio Estado Uso
auth-service estable compartido
users-service desarrollo activo backend
billing-service desarrollo activo core
payments-service estable crítico
notifications-service poco usado background
Infra

PostgreSQL

Redis

RabbitMQ

❌ Situación SIN Raioz (estado típico)
Onboarding de un dev nuevo

Clonar 6 repos

Resolver versiones de Node / Go

Copiar .env.example → .env

Ajustar puertos manualmente

Resolver conflictos de Docker

Preguntar “¿qué servicios necesito?”

Esperar a que alguien ayude

⏱️ Tiempo real: 1–2 días
💥 Errores comunes:

servicios que no levantan

versiones incorrectas

“en mi máquina sí funciona”

✅ Situación CON Raioz
📄 deps.json en el repo del proyecto
{
"schemaVersion": "1.0",
"project": {
"name": "billing-dashboard",
"network": "raioz-net"
},
"services": {
"users-service": {
"source": {
"kind": "git",
"repo": "git@github.com:org/users-service.git",
"branch": "develop",
"path": "users-service"
},
"docker": {
"mode": "dev",
"dockerfile": "Dockerfile.dev",
"ports": ["3001:3000"],
"dependsOn": ["postgres"]
}
},
"billing-service": {
"source": {
"kind": "git",
"repo": "git@github.com:org/billing-service.git",
"branch": "feature/taxes",
"path": "billing-service"
},
"docker": {
"mode": "dev",
"dockerfile": "Dockerfile.dev",
"ports": ["3002:3000"],
"dependsOn": ["postgres", "rabbit"]
}
},
"payments-service": {
"source": {
"kind": "image",
"image": "org/payments-service",
"tag": "2.4.1"
},
"docker": {
"mode": "prod",
"ports": ["3003:3000"]
}
}
},
"infra": {
"postgres": {
"image": "postgres",
"tag": "15"
},
"redis": {
"image": "redis",
"tag": "7"
},
"rabbit": {
"image": "rabbitmq",
"tag": "3-management"
}
}
}

🧑‍💻 Flujo real de un dev nuevo
git clone git@github.com:org/billing-dashboard.git
cd billing-dashboard
raioz up

Qué pasa internamente

Clona solo users-service y billing-service

No clona auth, payments, notifications

Usa imagen de payments-service

Levanta infra compartida

Usa .env centralizados

No pisa nada existente

⏱️ Tiempo real: 5–10 minutos
🧠 Carga cognitiva: casi nula

🔍 Validación punto por punto
1️⃣ “¿No es demasiado acoplado?”

❌ No.

Los microservicios siguen siendo autónomos

Se pueden correr sin Raioz

Raioz solo describe cómo se combinan

👉 Es orquestación, no dependencia.

2️⃣ “¿Y si cambio de rama?”

El dev hace:

cd /opt/raioz-proyecto/services/users-service
git checkout feature/x

Raioz:

NO fuerza cambios

Solo avisa si hay drift

👉 Control humano, no magia.

3️⃣ “¿Qué pasa si mañana necesito otro servicio?”

Se agrega al deps.json.

PR visible

Cambio explícito

Impacto controlado

👉 Infra como contrato.

4️⃣ “¿Esto reemplaza docker-compose?”

❌ No.

Raioz genera docker-compose.
Docker sigue siendo la verdad final.

👉 Raioz = traductor + guardián.

5️⃣ “¿Y si Raioz no existiera mañana?”

Riesgo real, buena pregunta.

Respuesta:

Los repos siguen intactos

El docker-compose.generated.yml existe

El deps.json es legible

Nada queda bloqueado

👉 No hay lock-in fuerte.

📊 Comparación objetiva
Métrica Sin Raioz Con Raioz
Onboarding 1–2 días 10 min
Repos clonados 6 2
Errores de env frecuentes raros
Conflictos de puertos comunes casi cero
Cambio de proyecto doloroso trivial
Disciplina requerida alta media
🧠 Insight clave (el más importante)

Raioz no acelera Docker.
Raioz acelera decisiones.

Hace explícito:

qué servicios importan

cuáles son estables

cuáles están en desarrollo

qué depende de qué

Eso es oro arquitectónico.
