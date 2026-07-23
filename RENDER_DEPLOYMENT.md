# Worktic AI V14 — despliegue en Render

## Arquitectura incluida

Esta entrega usa un servicio web Docker de Render con un disco persistente montado en `/var/data`. Allí se conservan la base SQLite, las sesiones multi-tenant de WhatsApp y los archivos operativos. Es adecuada para una primera producción controlada de una sola instancia.

## Despliegue con Blueprint

1. Sube este proyecto a un repositorio privado de GitHub.
2. En Render elige **New > Blueprint**.
3. Conecta el repositorio y selecciona `render.yaml`.
4. Completa las variables marcadas como `sync: false`.
5. Configura `BASE_URL` con el dominio final, por ejemplo `https://app.workticai.com`.
6. Despliega y verifica `/healthz` y `/readyz`.
7. Cambia inmediatamente la contraseña inicial del superadministrador.

## Variables obligatorias

- `CHANNEL_ENCRYPTION_KEY`: secreto estable. No debe cambiar después de conectar canales.
- `OPENAI_API_KEY`: clave central de Worktic.
- `BASE_URL`: URL pública HTTPS.
- Direcciones USDT si se habilitan pagos.

## Persistencia

El disco debe permanecer montado en `/var/data`. No cambies `DATA_DIR` después del despliegue. Realiza snapshots y descargas de respaldo de:

- `/var/data/worktic.db`
- `/var/data/wa_sessions/`

## Limitación de escalado

SQLite y las sesiones de WhatsApp almacenadas en un único disco obligan a utilizar una sola instancia web. No aumentes el número de instancias. Para escalado horizontal se requiere la migración final a PostgreSQL, Redis/Key Value y almacenamiento S3.
