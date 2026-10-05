# WorkticAI V27 — Growth Command Center

V27 se construye sobre la V26 actualmente operativa con PostgreSQL + Redis y añade tres capas integradas al producto sin sustituir los módulos existentes.

## 1. WhatsApp Marketing

Nuevo módulo independiente de Inbox y Agentes IA.

Flujo:

`Lead/Form/Meta Lead Ads con consentimiento → Segmento → Plantilla Meta aprobada → Cola V27 → WhatsApp Cloud → estados Meta → respuesta → Inbox/CRM/Analytics`

### Funciones implementadas

- Usa únicamente conexiones `whatsapp_cloud` ya conectadas por tenant.
- Descubre plantillas aprobadas desde el mismo WABA.
- Campañas en estados `draft`, `scheduled`, `running`, `paused`, `completed`, `cancelled`.
- Segmentación por origen, estado, score mínimo y campaña Growth.
- Audiencia basada en `marketing_leads` con `consent=1`.
- Deduplicación por teléfono dentro de campaña.
- Exclusión permanente de teléfonos en opt-out.
- Opt-out automático al recibir `SALIR`, `STOP`, `CANCELAR`, `BAJA`, `UNSUBSCRIBE` y variantes soportadas.
- Programación por fecha/hora.
- Rate limit configurable de 1 a 120 envíos/minuto.
- Variables de BODY por orden; soporta campos `name` y `phone` y literales.
- Métricas persistentes: enviados, entregados, leídos, respondidos, fallidos y bajas.
- Actualización de estados desde el webhook oficial de WhatsApp Cloud.
- Las respuestas entrantes marcan el destinatario como `replied` y continúan entrando al Inbox existente.
- Eventos de Analytics V24 para `whatsapp_marketing.sent` y `whatsapp_marketing.replied`.

### Tablas nuevas

- `whatsapp_marketing_campaigns_v27`
- `whatsapp_marketing_recipients_v27`
- `whatsapp_marketing_optouts_v27`

Se crean automáticamente al iniciar la aplicación. No requieren SQL manual.

## 2. Worktic Copilot

Asistente flotante global visible desde toda la aplicación.

### Capacidades

- Usa la variable `OPENAI_API_KEY` ya existente.
- Conoce los módulos reales de WorkticAI V27.
- Recibe contexto de la pantalla actual.
- Diagnostica mediante conteos reales del tenant: canales conectados, agentes activos, workflows activos, productos, contactos, leads, oportunidades, campañas WhatsApp Marketing y pagos pendientes.
- No envía PII al prompt de diagnóstico; usa conteos y contexto funcional.
- Ofrece accesos rápidos hacia el módulo adecuado según la pregunta.
- Nunca solicita ni muestra secretos, tokens o API keys.
- No afirma que una integración esté conectada cuando el diagnóstico dice lo contrario.

Endpoint:

- `GET /api/copilot/v27`
- `POST /api/copilot/v27`

## 3. Dashboard Intelligence

El Resumen se transforma en Growth Command Center.

### KPIs

- Revenue 30 días.
- Ventas ganadas.
- Leads.
- Conversaciones entrantes.
- Tasa de conversión.
- Variación vs. 30 días anteriores.
- Contactos.
- Oportunidades abiertas.
- Valor de pipeline.
- Mensajes sin leer.

### Visualizaciones

- Tendencia diaria de revenue en 14 días.
- Embudo Lead → Contacto → Oportunidad → Venta.
- Distribución por fuente de lead.
- Operación activa: canales, agentes IA, workflows y campañas WhatsApp Marketing.
- Insights automáticos basados en datos reales del tenant.

Endpoint:

- `GET /api/dashboard/v27`

## Variables de entorno

V27 no añade variables obligatorias nuevas.

Reutiliza:

```env
OPENAI_API_KEY=
OPENAI_MODEL=
DATABASE_URL=
REDIS_URL=
BACKGROUND_WORKERS_ENABLED=true
CHANNEL_RUNTIMES_ENABLED=true
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
WHATSAPP_VERIFY_TOKEN=
CHANNEL_ENCRYPTION_KEY=
BASE_URL=https://workticai.com
```

`BACKGROUND_WORKERS_ENABLED=true` es necesario para ejecutar campañas programadas/en cola.

## Seguridad y cumplimiento

WhatsApp Marketing no crea una vía paralela de envío masivo sin control. V27 exige consentimiento persistido en `marketing_leads.consent`, utiliza plantillas oficiales aprobadas y respeta bajas futuras. El módulo no sustituye las políticas ni límites del proveedor Meta.
