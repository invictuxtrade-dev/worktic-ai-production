# Worktic AI — Telegram Webhook Production Fix

## Corrección principal

Cada conexión Telegram multitenant ahora registra un webhook propio:

`https://TU_DOMINIO/webhooks/telegram/{public_id}`

Al conectar el canal, Worktic ejecuta automáticamente:

1. `getMe` para validar el token de BotFather.
2. Detección automática del username y nombre del bot.
3. `setWebhook` con un secreto exclusivo por conexión.
4. `getWebhookInfo` para confirmar que Telegram aceptó la URL.
5. Persistencia cifrada del token y del secreto.

## Botón Probar conexión

Cada tarjeta de canal tiene ahora **Probar conexión**. Para Telegram comprueba:

- token válido;
- identidad del bot;
- webhook configurado;
- coincidencia exacta de la URL;
- errores recientes de Telegram;
- actualizaciones pendientes.

## Recepción y respuesta

El webhook:

- identifica tenant y conexión por `public_id`;
- valida `X-Telegram-Bot-Api-Secret-Token`;
- guarda el mensaje en la conversación correcta;
- crea o actualiza el contacto CRM;
- usa el agente especializado asignado o el Asistente Principal;
- envía la respuesta por Telegram;
- registra consumo y errores del agente.

## Variable obligatoria en Render

Configura:

`BASE_URL=https://workticai.com`

Debe ser una URL pública HTTPS, sin `/` al final. Después de desplegar esta versión, pulsa **Reconfigurar** en el canal Telegram y vuelve a pegar el token para registrar el nuevo webhook.
