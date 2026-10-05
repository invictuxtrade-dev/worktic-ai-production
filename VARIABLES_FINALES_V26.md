# WorkticAI V26 — Variables finales de producción

Este es el inventario final para el despliegue acumulativo. No todas contienen secretos y algunas son opcionales.

## A. Infraestructura obligatoria

```env
APP_ENV=production
APP_NAME=Worktic AI
BASE_URL=https://TU-DOMINIO
DATA_DIR=/var/data

DATABASE_DRIVER=postgres
DATABASE_URL=<inyectada por Render Postgres>

REDIS_URL=<inyectada por Render Key Value>
REDIS_REQUIRED=true
TRUST_PROXY=true
BACKGROUND_WORKERS_ENABLED=true
CHANNEL_RUNTIMES_ENABLED=true

LEGACY_WHATSAPP_DRIVER=sqlite
LEGACY_WHATSAPP_DSN=file:/var/data/worktic.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)
```

## B. Seguridad

```env
CHANNEL_ENCRYPTION_KEY=<CONSERVAR EXACTAMENTE LA CLAVE ACTUAL>
MESSENGER_VERIFY_TOKEN=<secreto largo>
WHATSAPP_VERIFY_TOKEN=<secreto largo>
META_LEADS_VERIFY_TOKEN=<secreto largo>
```

`CHANNEL_ENCRYPTION_KEY` no debe regenerarse durante la actualización si ya existen tokens/credenciales cifradas.

### Temporales / excepciones

Durante el corte:

```env
MAINTENANCE_MODE=true
```

Al terminar:

```env
MAINTENANCE_MODE=false
```

Solo para una instalación completamente nueva y vacía:

```env
BOOTSTRAP_ADMIN_EMAIL=
BOOTSTRAP_ADMIN_PASSWORD=
```

Eliminar ambas después del primer arranque.

Solo emergencia temporal:

```env
ALLOW_SQLITE_PRODUCTION=false
```

No habilitar en operación normal.

## C. OpenAI

```env
OPENAI_API_KEY=
OPENAI_MODEL=gpt-5-mini
```

## D. Meta / Facebook / Instagram / Messenger / WhatsApp / Lead Ads

```env
META_GRAPH_VERSION=v25.0
META_APP_ID=
META_APP_SECRET=
META_WHATSAPP_CONFIG_ID=
META_SYSTEM_USER_ACCESS_TOKEN=
```

`META_SYSTEM_USER_ACCESS_TOKEN` es opcional y solo debe crearse si el flujo final Tech Provider lo necesita.

## E. Social Hub

```env
LINKEDIN_CLIENT_ID=
LINKEDIN_CLIENT_SECRET=
TIKTOK_CLIENT_KEY=
TIKTOK_CLIENT_SECRET=
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
TELEGRAM_BOT_TOKEN=
```

## F. Ads Center

```env
TIKTOK_BUSINESS_APP_ID=
TIKTOK_BUSINESS_SECRET=
TIKTOK_BUSINESS_AUTH_URL=
GOOGLE_ADS_DEVELOPER_TOKEN=
```

Meta Ads reutiliza `META_APP_ID` / `META_APP_SECRET`.
Google Ads reutiliza `GOOGLE_CLIENT_ID` / `GOOGLE_CLIENT_SECRET`.

## G. Billing

```env
USDT_BEP20_ADDRESS=
USDT_TRC20_ADDRESS=
PAYMENT_CONFIRMATIONS=12
```

## H. Operación

```env
MAX_MESSAGE_LENGTH=2000
SEND_COOLDOWN_SECONDS=3
AUTO_REPLY_COOLDOWN_SECONDS=30
ALLOW_GROUP_MESSAGES=false
MESSENGER_SYNC_ACTIVE_SECONDS=10
MESSENGER_SYNC_IDLE_SECONDS=60
MESSENGER_TOKEN_CHECK_HOURS=6
MESSENGER_OUTBOX_MAX_ATTEMPTS=5
```

## I. Automation Webhook — opcional

```env
AUTOMATION_WEBHOOK_ALLOWED_HOSTS=
```

Ejemplo:

```env
AUTOMATION_WEBHOOK_ALLOWED_HOSTS=api.empresa.com,hooks.partner.com
```

Si está vacío, V26 acepta únicamente destinos HTTPS públicos y sigue bloqueando redes privadas/reservadas.

# URLs finales que deben quedar registradas

Reemplaza `{BASE_URL}` por el dominio HTTPS real.

## Social OAuth

```text
{BASE_URL}/api/social/oauth/callback/facebook
{BASE_URL}/api/social/oauth/callback/linkedin
{BASE_URL}/api/social/oauth/callback/tiktok
{BASE_URL}/api/social/oauth/callback/youtube
```

## Ads OAuth

```text
{BASE_URL}/api/ads/v25/oauth/callback/meta
{BASE_URL}/api/ads/v25/oauth/callback/tiktok
{BASE_URL}/api/ads/v25/oauth/callback/google
```

## Meta Webhooks

```text
Messenger:      {BASE_URL}/webhooks/meta/messenger
WhatsApp Cloud: {BASE_URL}/webhooks/meta/whatsapp
Lead Ads:       {BASE_URL}/webhooks/meta/leads
```

## Permisos que obligan a reautorizar conexiones existentes

Facebook / Instagram Social + Lead Ads:

```text
pages_show_list
pages_read_engagement
pages_manage_posts
pages_manage_metadata
pages_manage_engagement
leads_retrieval
instagram_basic
instagram_content_publish
instagram_manage_comments
business_management
```

Meta Ads:

```text
ads_read
ads_management
business_management
```

YouTube:

```text
https://www.googleapis.com/auth/youtube.upload
https://www.googleapis.com/auth/youtube.readonly
https://www.googleapis.com/auth/youtube.force-ssl
```

La autorización final disponible para clientes externos depende de las revisiones/aprobaciones de cada proveedor.
