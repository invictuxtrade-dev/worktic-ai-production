# WorkticAI V19 — Meta Lead Ads → CRM

## Objetivo

V19 conecta oficialmente los formularios instantáneos de Meta con la operación comercial de WorkticAI.

Flujo:

`Facebook / Instagram Lead Ad → Page Webhook leadgen → Graph API → marketing_leads → CRM → oportunidad → lead.created → V20 Automatizaciones`

## Funciones implementadas

- Nueva pestaña `Meta Lead Ads` dentro de `Growth & Campañas`.
- Descubrimiento de páginas Facebook ya conectadas mediante Social Hub.
- OAuth Meta ampliado con `leads_retrieval` y `pages_manage_metadata`.
- Suscripción de la página al campo Webhooks `leadgen`.
- Webhook global: `/webhooks/meta/leads`.
- Verificación GET mediante `META_LEADS_VERIFY_TOKEN`.
- Verificación HMAC SHA-256 de POST usando `META_APP_SECRET`.
- Deduplicación por `page_id + leadgen_id`.
- Recuperación del lead completo desde Graph API usando el Page Access Token cifrado que ya administra Social Hub.
- Normalización de nombre, teléfono, email, ciudad y primer campo comercial adicional.
- Score base configurable por página.
- Creación del lead en `marketing_leads`.
- Creación/actualización automática del contacto CRM.
- Creación/actualización de oportunidad mediante el motor existente.
- Registro de IDs externos de Meta: lead, página, formulario, anuncio, adset y campaña cuando están disponibles.
- Nueva cola persistente `lead_automation_outbox` con evento `lead.created` para V20.
- Seguimiento opcional mediante una plantilla aprobada de WhatsApp Cloud.
- El seguimiento automático WhatsApp solo ocurre si el administrador define un campo de opt-in y ese campo llega con un valor afirmativo.
- Consulta de formularios `leadgen_forms` por página.
- Monitor de eventos recientes y errores.

## Tablas nuevas

Se crean automáticamente al arrancar:

- `meta_lead_profiles`
- `meta_lead_events`
- `meta_lead_forms`
- `lead_automation_outbox`

También se agregan de forma automática columnas de atribución Meta a `marketing_leads`.

## Variable nueva

```env
META_LEADS_VERIFY_TOKEN=
```

No configurarla todavía si todavía no se va a desplegar la versión acumulativa.

## Configuración final pendiente en Meta

Al hacer el despliegue final:

1. Reconectar Facebook/Instagram desde Social Hub para autorizar los nuevos scopes de Lead Ads.
2. Confirmar App Review/Advanced Access aplicable para `leads_retrieval` y `pages_manage_metadata`.
3. En Webhooks > Page usar:
   - Callback URL: `https://TU-DOMINIO/webhooks/meta/leads`
   - Verify Token: igual a `META_LEADS_VERIFY_TOKEN`
   - Campo: `leadgen`
4. Activar cada página desde WorkticAI > Growth & Campañas > Meta Lead Ads.
5. Probar con un lead de prueba antes de tráfico real.

## Agentes IA unificados

V19 elimina la duplicidad visual entre `Agente IA clásico` y `Agentes IA`.

El producto muestra un único módulo `Agentes IA`:

- Agente Principal: fallback general del tenant.
- Agentes especializados: ventas, soporte, agenda, marketing, comunidad, etc.
- Enrutamiento.
- Métricas.
- Permisos.
- Simulador por agente.

La configuración legacy se conserva solo internamente para migración/compatibilidad. En una instalación legacy de un único tenant, si todavía no existe ningún registro en `ai_agents`, V19 crea el Agente Principal a partir de la configuración clásica existente.
