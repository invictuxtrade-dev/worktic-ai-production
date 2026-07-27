# Worktic AI — Messenger Channel Center

Esta versión completa la conexión multitenant de Facebook Messenger.

## Incluye
- Endpoint exclusivo por conexión: `/webhooks/messenger/{public_id}`.
- Verificación GET de Meta mediante token exclusivo.
- Recepción POST de mensajes y respuesta del agente IA.
- Validación opcional de `X-Hub-Signature-256` mediante App Secret.
- Validación del Page Access Token y Page ID con Meta Graph API.
- Suscripción automática de la página a `messages`, `messaging_postbacks`, `message_reads` y `message_deliveries` cuando Meta lo permite.
- Diagnóstico con página detectada, token, webhook, verify token, suscripción y actividad.
- Botones para copiar URL de devolución y token de verificación.
- Reparación de conexiones antiguas: al pulsar **Probar conexión**, Worktic genera los datos de webhook que faltaban.

## Después de desplegar
1. Confirma `BASE_URL=https://www.workticai.com` o el dominio HTTPS final que sirve el backend.
2. En Canales, pulsa **Probar conexión** en Messenger.
3. Copia URL de devolución y token de verificación.
4. Pégalos en Meta Developers → Messenger → Configurar webhooks.
5. En la página, pulsa **Agregar suscripciones** y activa los campos indicados.
6. Prueba desde una cuenta con rol en la app mientras Meta esté en modo desarrollo.
