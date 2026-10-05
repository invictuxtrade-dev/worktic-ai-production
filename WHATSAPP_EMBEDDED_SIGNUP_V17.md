# WorkticAI V17 — Meta Embedded Signup

## Objetivo
Permitir que cada tenant conecte WhatsApp Business Platform mediante el onboarding oficial de Meta, sin copiar manualmente WABA ID, Phone Number ID o Access Token.

## Flujo
1. Crear un canal `whatsapp_cloud`.
2. Abrir **Conectar** y pulsar **Continuar con Meta**.
3. El frontend carga el Facebook JavaScript SDK con `META_APP_ID` y `META_WHATSAPP_CONFIG_ID`.
4. Meta ejecuta Embedded Signup y devuelve un `code`; el evento `WA_EMEDDED_SIGNUP/WA_EMBEDDED_SIGNUP` aporta WABA y Phone Number ID cuando están disponibles.
5. Worktic envía el `code` al backend. El App Secret nunca se expone al navegador.
6. Backend intercambia el code por token, descubre el WABA si es necesario, descubre el número si es necesario, registra el teléfono cuando se proporciona PIN, valida el número, suscribe la app al WABA y guarda credenciales cifradas por tenant.
7. Los webhooks siguen entrando por `/webhooks/meta/whatsapp`.

## Variables de Render
- `META_APP_ID`
- `META_APP_SECRET`
- `META_WHATSAPP_CONFIG_ID` — Configuration ID creado en Meta para Embedded Signup.
- `META_SYSTEM_USER_ACCESS_TOKEN` — opcional; útil como token de sistema para escenarios de Tech Provider.
- `WHATSAPP_VERIFY_TOKEN`
- `META_GRAPH_VERSION=v25.0`

## Endpoints nuevos
- `GET /api/whatsapp/embedded/config`
- `POST /api/whatsapp/embedded/complete`

## Seguridad
- Solo owner/admin/superadmin puede iniciar/completar onboarding.
- `META_APP_SECRET` permanece únicamente en backend.
- Tokens finales se cifran con `CHANNEL_ENCRYPTION_KEY`.
- El backend vuelve a validar WABA + Phone Number ID contra Graph API antes de marcar el canal como conectado.
- La conexión manual de V16 continúa disponible como fallback.

## Meta Dashboard necesario
Antes de usarlo en producción se debe crear una configuración de Facebook Login for Business / WhatsApp Embedded Signup y copiar su Configuration ID a `META_WHATSAPP_CONFIG_ID`. La app también debe tener los permisos y acceso avanzados requeridos por Meta para el modelo de negocio usado.
