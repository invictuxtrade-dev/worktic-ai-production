# WorkticAI V16 — Meta Official / WhatsApp Cloud API

## Qué agrega esta fase

- Nuevo tipo de canal `whatsapp_cloud`, paralelo a `whatsapp_qr`.
- Conexión oficial por WABA ID + Phone Number ID + Access Token.
- Validación del número contra Meta Graph API.
- Suscripción automática del WABA a la aplicación (`/{WABA-ID}/subscribed_apps`).
- Token cifrado con la misma capa segura de credenciales de WorkticAI.
- Webhook oficial `/webhooks/meta/whatsapp`.
- Verificación GET del webhook mediante `WHATSAPP_VERIFY_TOKEN`.
- Validación POST con `X-Hub-Signature-256` cuando `META_APP_SECRET` está configurado.
- Recepción de mensajes Cloud API dentro de `worktic_messages` y `worktic_contacts`.
- Actualización de estados de mensajes (`sent`, `delivered`, `read`, `failed` cuando Meta los envía).
- Envío de texto desde la bandeja omnicanal mediante Cloud API.
- Diagnóstico del WABA, Phone Number ID, nombre verificado y quality rating.
- Gestión inicial de plantillas: listar plantillas y crear una plantilla de texto para revisión de Meta.
- Render actualizado a `META_GRAPH_VERSION=v25.0` y nuevas variables oficiales de Meta.

## Variables de Render

- `META_APP_ID`
- `META_APP_SECRET`
- `META_GRAPH_VERSION=v25.0`
- `WHATSAPP_VERIFY_TOKEN`
- `CHANNEL_ENCRYPTION_KEY`
- `BASE_URL`

## Flujo de conexión en WorkticAI

1. Canales → Nueva conexión.
2. Elegir **WhatsApp Business Cloud (Oficial)**.
3. Crear la conexión.
4. Pulsar **Conectar**.
5. Ingresar WABA ID, Phone Number ID y Access Token.
6. Worktic valida el número y suscribe el WABA.
7. Copiar Callback URL y Verify Token mostrados por Worktic.
8. En Meta Developers → WhatsApp → Configuration registrar el webhook y suscribir `messages`.
9. Ejecutar **Probar conexión**.
10. Enviar un mensaje real al número para validar recepción en Inbox.

## Importante

Esta fase usa conexión manual segura porque funciona inmediatamente con una empresa/app ya configurada. El siguiente bloque puede añadir **Embedded Signup** para que cada cliente SaaS conecte su propio Business Portfolio/WABA desde WorkticAI sin copiar IDs o tokens manualmente.
