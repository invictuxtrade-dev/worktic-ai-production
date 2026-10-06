# WorkticAI — Actualización final acumulativa

> Documento final acumulativo V15→V26. Las notas de infraestructura de fases anteriores quedan reemplazadas por la sección V26 y por `DESPLIEGUE_FINAL_V26_RENDER.md`.

## Estrategia de actualización

La versión actualmente desplegada en Render permanece sin cambios mientras construimos las fases nuevas. Cada nueva versión se construyó sobre la anterior (V15 → V16 → … → V26). V26 es el paquete final acumulativo; el despliegue debe hacerse como un único salto siguiendo `DESPLIEGUE_FINAL_V26_RENDER.md`.

## Fases acumuladas hasta V26

### V15 — Social Hub Macro
- Reorganización del Social Hub como centro multired.
- Resumen, Composer, AI Content Studio, Calendario, Contenido, Analytics y Cuentas.
- Selector por Facebook, Instagram, TikTok, YouTube, LinkedIn y Telegram.
- Conserva OAuth, calendario persistente, publicaciones y analítica existentes.

### V16 — WhatsApp Business Cloud API
- Nuevo tipo de canal `whatsapp_cloud`, coexistiendo con `whatsapp_qr`.
- WABA ID, Phone Number ID y token cifrado por tenant.
- Validación de número y WABA.
- Webhook oficial `/webhooks/meta/whatsapp`.
- Recepción de mensajes y estados.
- Envío de texto desde Inbox.
- Gestión inicial de templates.

### V17 — Meta Embedded Signup
- Botón principal `Continuar con Meta` para onboarding oficial.
- Flujo OAuth/Embedded Signup por tenant.
- Intercambio de código temporal exclusivamente en backend.
- Descubrimiento y validación de WABA/Phone Number ID.
- Suscripción del WABA al webhook.
- Registro del número y soporte de PIN cuando aplica.
- Conexión manual conservada como fallback técnico.

### V18 — WhatsApp Business Center
- Nuevo módulo `WhatsApp Business` dentro de WorkticAI.
- Selector de múltiples conexiones WhatsApp Cloud por empresa.
- Dashboard operativo por número.
- Número y Quality Rating.
- Constructor visual de templates.
- Borradores locales persistentes.
- Header de texto, Body, Footer y botones.
- Variables `{{1}}`, `{{2}}` con ejemplos para revisión.
- Envío de borradores a Meta.
- Sincronización de templates oficiales y estados.
- Eliminación de templates en Meta.
- Envío de prueba para templates aprobados sin parámetros obligatorios.
- Contactos exclusivos de la conexión.
- Analítica de mensajes entrantes/salientes y estados registrada por Worktic.
- Accesos directos a Inbox, Automatizaciones y Campañas para evitar duplicar motores.
- Nuevas tablas creadas automáticamente al arrancar:
  - `whatsapp_business_events`
  - `whatsapp_template_drafts`

### V19 — Meta Lead Ads + Agentes IA unificados
- El menú deja de mostrar `Agente IA clásico`; existe un único centro `Agentes IA`.
- El Agente Principal funciona como fallback y los demás agentes se especializan por función/canal.
- Migración automática del agente legacy al Agente Principal cuando corresponde.
- OAuth Facebook/Instagram solicita adicionalmente `leads_retrieval` y `pages_manage_metadata`.
- Nueva pestaña `Meta Lead Ads` dentro de Growth & Campañas.
- Suscripción de páginas al webhook `leadgen`.
- Nuevo webhook `/webhooks/meta/leads` con Verify Token + validación HMAC.
- Recuperación de datos completos del lead desde Graph API.
- Lead → CRM → oportunidad → evento persistente `lead.created`.
- IDs de página/formulario/anuncio/adset/campaña conservados para atribución.
- Seguimiento opcional por template WhatsApp Cloud solo con campo de opt-in explícito configurado y afirmativo.
- Nuevas tablas automáticas: `meta_lead_profiles`, `meta_lead_events`, `meta_lead_forms`, `lead_automation_outbox`.

## Variables nuevas acumuladas desde la versión que está actualmente desplegada

Estas variables se configuran únicamente durante el corte final V26, siguiendo `VARIABLES_FINALES_V26.md` y `DESPLIEGUE_FINAL_V26_RENDER.md`.

### Nuevas obligatorias para Meta/WhatsApp

```env
META_WHATSAPP_CONFIG_ID=
WHATSAPP_VERIFY_TOKEN=
META_LEADS_VERIFY_TOKEN=
```

### Ya existentes, pero deben quedar correctamente configuradas

```env
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
CHANNEL_ENCRYPTION_KEY=
BASE_URL=https://TU-DOMINIO-PRODUCCION
```

### Opcional

```env
META_SYSTEM_USER_ACCESS_TOKEN=
```

`META_SYSTEM_USER_ACCESS_TOKEN` queda reservado para flujos de Tech Provider/sistema cuando se requiera. No debe crearse por obligación si el flujo final no lo necesita.

## Render — estado final V26

Las notas antiguas de “SQLite como base principal” quedan reemplazadas por V26:

1. PostgreSQL es la base principal mediante `DATABASE_URL`.
2. Render Key Value/Redis se usa mediante `REDIS_URL`.
3. El Persistent Disk `/var/data` se conserva para uploads, backups temporales y stores WhatsApp QR heredados.
4. `CHANNEL_ENCRYPTION_KEY` debe conservar exactamente el valor de producción actual.
5. El primer deploy V26 se realiza con `MAINTENANCE_MODE=true`; solo se cambia a `false` después del backup, migración y verificación.
6. La guía definitiva es `DESPLIEGUE_FINAL_V26_RENDER.md`; las variables definitivas están en `VARIABLES_FINALES_V26.md`.

## Meta Developers — cambios acumulados pendientes

Al momento del despliegue final habrá que completar/revisar:

1. App Meta en modo y configuración de producción apropiados.
2. Producto WhatsApp configurado.
3. Embedded Signup Configuration y copiar su Configuration ID a `META_WHATSAPP_CONFIG_ID`.
4. Callback URL de WhatsApp:
   `https://TU-DOMINIO/webhooks/meta/whatsapp`
5. Verify Token idéntico a `WHATSAPP_VERIFY_TOKEN`.
6. Suscripción del campo `messages`.
7. Permisos/App Review/Advanced Access requeridos para las funciones que se publiquen a clientes externos.
8. Dominios y URLs autorizadas del flujo de login/Embedded Signup.
9. Webhook Page Lead Ads: `https://TU-DOMINIO/webhooks/meta/leads`.
10. Verify Token igual a `META_LEADS_VERIFY_TOKEN`.
11. Suscribir el campo `leadgen`.
12. Solicitar/revisar acceso para `leads_retrieval` y `pages_manage_metadata`.
13. Reconectar Facebook/Instagram en Worktic para conceder los nuevos permisos.

## Migración de base de datos

Hasta V18 no se necesita ejecutar SQL manualmente. Las tablas nuevas se crean al iniciar la aplicación.

Antes del despliegue final se deberá hacer backup de:

- `/var/data/worktic.db`
- `/var/data/wa_sessions/`

No borrar el Persistent Disk de Render.

## Pruebas obligatorias cuando llegue el despliegue final

1. Login y separación multitenant.
2. CRM y oportunidades.
3. Agenda.
4. WhatsApp QR existente.
5. Telegram.
6. Messenger.
7. Social Hub y calendario persistente.
8. Facebook/Instagram OAuth.
9. TikTok/YouTube/LinkedIn existentes.
10. WhatsApp Cloud manual.
11. Meta Embedded Signup.
12. Recepción webhook WhatsApp.
13. Envío desde Inbox.
14. Templates: crear, revisar, sincronizar y probar.
15. Reiniciar servicio y comprobar persistencia.
16. Reconectar Facebook/Instagram con permisos Lead Ads.
17. Activar una página en Growth & Campañas → Meta Lead Ads.
18. Enviar un Lead Ad de prueba y confirmar: Lead → CRM → oportunidad → evento `lead.created`.
19. Si se activa seguimiento WhatsApp, validar primero el campo de opt-in explícito y una plantilla aprobada.

## Pendiente para fases siguientes

- Automatizaciones avanzadas orientadas a eventos.
- Campañas WhatsApp con consentimiento/segmentación/template.
- Handoff IA ↔ humano más avanzado.
- V25 — Ads Center (Meta/TikTok/Google/YouTube) para alimentar spend y performance publicitario real.
- Meta Ads Center.
- Evolución de infraestructura a PostgreSQL/Redis/workers cuando se decida escalar horizontalmente.


---

# Fase V20 — Automation Engine

## Cambios acumulados

- Se reemplaza la vista principal de reglas simples por un constructor de workflows.
- Las reglas heredadas (`worktic_auto_rules`) se mantienen por compatibilidad y no se eliminan en la actualización.
- Nuevas tablas creadas automáticamente al iniciar:
  - `automation_workflows`
  - `automation_events`
  - `automation_executions`
  - `automation_execution_steps`
- Worker interno para procesar eventos persistentes.
- Integración automática con la cola `lead_automation_outbox` de V19.
- `lead.created` ya puede disparar workflows activos.
- Historial auditable de ejecuciones y pasos.
- No se añaden variables de entorno nuevas en V20.

## Migración

Hasta V20 no hay SQL manual. El esquema nuevo se crea automáticamente en el arranque.

## Pruebas que se añadirán al checklist final

20. Crear un workflow de prueba y guardarlo como borrador.
21. Activarlo y ejecutar una prueba manual.
22. Confirmar registro en `automation_executions`.
23. Recibir un Lead Ad real/de prueba y confirmar que `lead.created` dispara el workflow activo.
24. Verificar acción CRM (etiqueta/pipeline) sobre el tenant correcto.
25. Verificar acción WhatsApp Cloud solo usando un número/conexión autorizados.
26. Pausar el workflow y confirmar que nuevos eventos ya no lo ejecutan.

## Variables acumuladas

V20 no incorpora variables nuevas. Se mantienen las acumuladas hasta V19:

```env
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
META_WHATSAPP_CONFIG_ID=
WHATSAPP_VERIFY_TOKEN=
META_LEADS_VERIFY_TOKEN=
CHANNEL_ENCRYPTION_KEY=
BASE_URL=
```

Opcional:

```env
META_SYSTEM_USER_ACCESS_TOKEN=
```

---

## V21 — Inbox Omnicanal definitivo

### Código / base de datos
Se añaden automáticamente:
- `inbox_conversations`
- `inbox_notes`
- `inbox_departments`

No ejecutar SQL manual.

### Operación
- Handoff IA ↔ humano por chat.
- Asignación de responsable, departamento, prioridad, SLA, etiquetas y notas internas.
- Eventos del Inbox conectados con Automation Engine V20.
- Listado/mensajes del Inbox reforzados con aislamiento por `tenant_id`.
- Envío manual por WhatsApp Cloud integrado en la misma bandeja.

### Variables nuevas
Ninguna variable nueva en V21.

### Pendiente para despliegue final
No configurar ni desplegar todavía. La rama acumulativa continúa hacia V22 y posteriores.


## V22 — Agentes IA 2.0
- Unifica definitivamente la experiencia de Agentes IA.
- Añade memoria persistente aislada por tenant/agente/cliente.
- Añade conocimiento empresarial compartido o por especialista.
- Añade capacidades autorizables: catálogo, CRM, agenda, pipeline, tareas y handoff.
- Integra memoria/conocimiento en respuestas reales de WhatsApp QR, Telegram y Messenger.
- No agrega variables de entorno nuevas.
- Crea automáticamente: `ai_agent_profiles_v22`, `ai_agent_memory_v22`, `ai_agent_knowledge_v22`, `ai_agent_tool_audit_v22`.


---

# V23 — Social Hub Advanced

## Cambios acumulados

- Social Hub incorpora las áreas **Multimedia**, **Reutilizar IA**, **Aprobaciones** y **Community**.
- Composer admite `mediaURLs` y recursos múltiples reales (hasta 10) manteniendo compatibilidad con `mediaURL` legado.
- Carrusel real implementado para Facebook, Instagram, TikTok photo posts y álbum Telegram según las capacidades del proveedor.
- YouTube mantiene un video por publicación.
- LinkedIn multi-asset se bloquea explícitamente en V23 hasta implementar su flujo específico de media/documentos; no se simula publicación exitosa.
- TikTok Direct Post consulta `creator_info` antes de publicar y usa los endpoints actuales de video/foto según el formato.
- Biblioteca multimedia tenant-scoped e indexación automática de uploads del Social Hub.
- Reutilización con IA genera variantes por red/formato y devuelve cada variante al Composer para revisión.
- Aprobaciones editor → revisor con estado `pending_approval`; el scheduler no publica piezas pendientes de revisión.
- Community sincroniza/responde comentarios oficiales de Facebook, Instagram y YouTube.
- Community genera eventos `social.comment` y `social.comment.replied` hacia Automation Engine V20.
- Nuevas tablas automáticas:
  - `social_media_assets`
  - `social_approvals`
  - `social_comments_v23`
  - `social_repurpose_jobs_v23`
- V23 no agrega variables de entorno nuevas.

## Permisos / reautorizaciones que se añaden al despliegue final

### Facebook / Instagram

Añadir/revisar en Meta y volver a autorizar conexiones existentes para incluir:

```text
pages_manage_engagement
instagram_manage_comments
```

Estos se suman a los permisos ya acumulados para Pages, publicación, Lead Ads, Instagram y Business. La disponibilidad para clientes externos dependerá de que la app tenga los niveles de acceso/revisión que correspondan en Meta.

### YouTube

El OAuth acumulativo añade:

```text
https://www.googleapis.com/auth/youtube.force-ssl
```

Este scope se requiere por las funciones de comentarios/respuestas de V23. Los canales YouTube autorizados antes de esta fase deberán reconectarse al hacer la actualización final.

### TikTok

No se añade variable nueva. Antes de liberar publicación pública para clientes se debe verificar en TikTok Developers que la aplicación/producto Content Posting tenga las revisiones/auditoría y dominios/URLs exigidos para Direct Post y PULL_FROM_URL.

## Checklist adicional para la actualización final

27. Reconectar Facebook/Instagram y confirmar que los permisos de Community fueron concedidos.
28. Reconectar YouTube y confirmar `youtube.force-ssl`.
29. Subir un archivo a Multimedia y comprobar persistencia tras reiniciar.
30. Crear carrusel Facebook/Instagram de prueba con 2+ assets y confirmar respuesta del proveedor.
31. Probar TikTok con una cuenta autorizada y verificar Creator Info + publicación según capacidades de esa cuenta.
32. Crear una pieza → Enviar a aprobación → Aprobar → comprobar publicación/programación.
33. Rechazar otra pieza y confirmar que vuelve a borrador.
34. Sincronizar comentarios Facebook/Instagram/YouTube.
35. Responder un comentario oficial y confirmar que Worktic lo marca `replied`.
36. Confirmar que `social.comment` llega a Automation Engine cuando existe workflow activo.
37. Generar una reutilización con IA y enviar una variante al Composer sin publicar automáticamente.

## Variables acumuladas tras V23

V23 no modifica la lista:

```env
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
META_WHATSAPP_CONFIG_ID=
WHATSAPP_VERIFY_TOKEN=
META_LEADS_VERIFY_TOKEN=
CHANNEL_ENCRYPTION_KEY=
BASE_URL=
```

Opcional:

```env
META_SYSTEM_USER_ACCESS_TOKEN=
```

**No configurar/desplegar todavía.** Seguimos construyendo sobre esta misma rama acumulativa.


---

# V23.1 — Social Governance & Autopilot

## Decisión de producto

Las aprobaciones de V23 pasan a ser **opcionales y configurables**, no un requisito universal. El modo de operación no depende del tamaño de la empresa.

Modos disponibles:

- `autopilot`: publicación/programación automática por defecto;
- `hybrid`: automático con reglas de revisión;
- `approval`: toda publicación/programación exige revisión.

## Gobernanza añadida

- Excepciones por red: heredar / Autopilot / Aprobación.
- Campañas específicas que siempre requieren aprobación.
- Palabras sensibles que disparan revisión en modo Híbrido.
- Palabras bloqueadas que impiden salida automática.
- Pausa global de emergencia del Social Hub.
- Auditoría de decisiones de gobernanza.
- Enforcement en backend para creación, edición, publicación directa y scheduler.

## Nuevas tablas automáticas

- `social_governance_v231`
- `social_governance_audit_v231`

No requiere SQL manual.

## Variables nuevas

Ninguna variable nueva en V23.1.

## Ajustes al checklist final

38. Definir el modo inicial de cada tenant (Autopilot/Híbrido/Aprobación).
39. Probar una publicación en Autopilot sin revisión.
40. Probar una publicación en Aprobación y confirmar `pending_approval`.
41. Probar Híbrido con palabra sensible.
42. Probar excepción por red.
43. Probar campaña marcada para revisión.
44. Probar palabra bloqueada y estado `governance_hold`.
45. Activar Pausa global y confirmar que ninguna publicación automática sale.
46. Reanudar y confirmar que los elementos retenidos por pausa vuelven a cola.
47. Revisar `social_governance_audit_v231` en una prueba de soporte/auditoría.

## Variables acumuladas tras V23.1

Se mantiene la lista de V23:

```env
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
META_WHATSAPP_CONFIG_ID=
WHATSAPP_VERIFY_TOKEN=
META_LEADS_VERIFY_TOKEN=
CHANNEL_ENCRYPTION_KEY=
BASE_URL=
```

Opcional:

```env
META_SYSTEM_USER_ACCESS_TOKEN=
```

**No configurar/desplegar todavía.** V23.1 será la base para V24.

---

# V24 — Analytics & Attribution

## Capa transversal de inteligencia

V24 añade el módulo global `Analytics` para conectar:

```text
Social / Ads / Landing
→ Lead
→ Conversación
→ Cita
→ Oportunidad
→ Venta
→ Revenue
```

Incluye:

- KPIs de revenue, ventas, leads, citas, oportunidades, CPL, CAC, ROAS y ticket medio.
- First Touch y Last Touch.
- Rendimiento por canal.
- Rendimiento por campaña.
- Rendimiento comercial de contenido social.
- Rendimiento por owner comercial.
- Rendimiento por Agente IA asociado.
- Customer Journeys de ventas cerradas.
- Exportación CSV.

## Tracking Worktic V24

Las publicaciones con enlace pueden usar automáticamente:

```text
BASE_URL/t/{tenant_id}/{social_post_id}
```

El redirect registra el clic, guarda First/Last Touch durante 90 días y propaga UTM/identificadores hacia landings y formularios Worktic.

`BASE_URL` ya era una variable obligatoria acumulada; **no se añade una variable nueva**, pero antes del despliegue final debe apuntar al dominio HTTPS real porque ahora también se usa para tracking.

## Nuevas tablas automáticas

- `analytics_attribution_events_v24`
- `analytics_settings_v24`

Se añaden columnas de atribución a tablas existentes de Social, Leads, Oportunidades y Agenda. No hay SQL manual.

## Moneda

Owner/Admin configura moneda operativa de Analytics:

- USD
- COP
- EUR
- MXN

V24 no hace conversión FX; se asume que revenue/spend del tenant están registrados en una unidad coherente.

## Nuevas rutas

```text
/api/analytics/v24/overview
/api/analytics/v24/journeys
/api/analytics/v24/settings
/api/analytics/v24/export.csv
/t/{tenant}/{post}
```

## Checklist adicional final

48. Configurar moneda del tenant.
49. Probar filtros de fecha.
50. Probar First Touch y Last Touch.
51. Publicar contenido con enlace y verificar tracking `/t/...`.
52. Hacer clic y comprobar redirección + UTM.
53. Completar una landing Worktic desde el enlace rastreado.
54. Confirmar atribución del lead a canal/campaña/post.
55. Confirmar atribución de oportunidad y cita.
56. Marcar oportunidad `Ganado` y validar revenue.
57. Revisar tablas por canal/campaña/contenido/owner/agente.
58. Exportar CSV.
59. Verificar aislamiento tenant.
60. Validar que no se inventan datos ausentes.

## Variables acumuladas tras V24

No se añaden variables nuevas:

```env
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
META_WHATSAPP_CONFIG_ID=
WHATSAPP_VERIFY_TOKEN=
META_LEADS_VERIFY_TOKEN=
CHANNEL_ENCRYPTION_KEY=
BASE_URL=https://TU-DOMINIO-PRODUCCION
```

Opcional:

```env
META_SYSTEM_USER_ACCESS_TOKEN=
```

> Nota histórica: V24 fue una base intermedia. No desplegar esta fase por separado; el destino final es V26.

---

# FASE V25 — Ads Center

V25 incorpora paid media oficial sobre Analytics V24.

## Proveedores

- Meta Ads
- TikTok Ads
- Google Ads / YouTube Ads

## Funciones acumuladas

- OAuth/autorización por tenant.
- Múltiples cuentas publicitarias por empresa.
- Descubrimiento de cuentas autorizadas.
- Soporte Google MCC de primer nivel cuando está disponible.
- Sincronización de campañas.
- Sincronización campaña → ad set/ad group → anuncio.
- Gasto, impresiones, clics, conversiones y valor del proveedor.
- Mapeo de campaña externa a campaña Worktic.
- Activar/pausar campañas cuando la API lo permite.
- Google VIDEO heredado en modo solo lectura.
- Worker automático cada 6 horas.
- Integración con V24 sin duplicar gasto.
- ROAS de plataforma separado de ROAS real de CRM.
- Protección multi-moneda: no se mezclan monedas sin FX explícito.

## Nuevas tablas automáticas

```text
ads_connections_v25
ads_campaigns_v25
ads_entities_v25
ads_daily_metrics_v25
ads_oauth_states_v25
ads_sync_log_v25
```

No ejecutar SQL manual.

## Variables NUEVAS V25

Agregar al entorno final:

```env
TIKTOK_BUSINESS_APP_ID=
TIKTOK_BUSINESS_SECRET=
TIKTOK_BUSINESS_AUTH_URL=
GOOGLE_ADS_DEVELOPER_TOKEN=
```

Variables existentes reutilizadas:

```env
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
CHANNEL_ENCRYPTION_KEY=
BASE_URL=https://TU-DOMINIO-PRODUCCION
```

## Redirect URLs nuevas

Registrar en los portales correspondientes:

```text
{BASE_URL}/api/ads/v25/oauth/callback/meta
{BASE_URL}/api/ads/v25/oauth/callback/google
{BASE_URL}/api/ads/v25/oauth/callback/tiktok
```

En TikTok, generar/configurar el Advertiser Authorization URL usando el callback anterior y guardar la URL resultante como `TIKTOK_BUSINESS_AUTH_URL`.

## Checklist final V25

61. Habilitar permisos Marketing API necesarios en Meta.
62. Registrar callback Meta Ads.
63. Crear/configurar app TikTok API for Business.
64. Habilitar Marketing API de TikTok.
65. Registrar callback TikTok y copiar Advertiser Authorization URL.
66. Crear `TIKTOK_BUSINESS_APP_ID`.
67. Crear `TIKTOK_BUSINESS_SECRET`.
68. Crear `TIKTOK_BUSINESS_AUTH_URL`.
69. Habilitar Google Ads API en el proyecto Google correspondiente.
70. Obtener `GOOGLE_ADS_DEVELOPER_TOKEN` desde Google Ads API Center.
71. Registrar callback Google Ads.
72. Confirmar OAuth scope de Google Ads.
73. Conectar una cuenta Meta Ads y sincronizar 30 días.
74. Conectar una cuenta TikTok Ads y sincronizar 30 días.
75. Conectar Google Ads y comprobar cuentas/MCC.
76. Verificar campañas, ad groups/ad sets y ads.
77. Mapear al menos una campaña externa a una campaña Worktic.
78. Confirmar gasto oficial en Ads Center y V24 Analytics sin duplicación.
79. Marcar una oportunidad asociada como `Ganado` y comprobar ROAS Worktic.
80. Comparar ROAS plataforma vs ROAS Worktic.
81. Probar First Touch y Last Touch.
82. Validar una cuenta con moneda distinta y confirmar que no se mezcla en ROAS consolidado.
83. Probar pausa/activación de campañas soportadas.
84. Confirmar que Google VIDEO heredado aparece en solo lectura.
85. Revisar logs `ads_sync_log_v25` ante errores de API.
86. Verificar aislamiento multitenant de cuentas/tokens/campañas.

## Variables acumuladas tras V25

```env
META_APP_ID=
META_APP_SECRET=
META_GRAPH_VERSION=v25.0
META_WHATSAPP_CONFIG_ID=
WHATSAPP_VERIFY_TOKEN=
META_LEADS_VERIFY_TOKEN=
CHANNEL_ENCRYPTION_KEY=
BASE_URL=https://TU-DOMINIO-PRODUCCION
TIKTOK_BUSINESS_APP_ID=
TIKTOK_BUSINESS_SECRET=
TIKTOK_BUSINESS_AUTH_URL=
GOOGLE_ADS_DEVELOPER_TOKEN=
```

Ya existentes para Social Hub y reutilizadas por V25 cuando corresponda:

```env
LINKEDIN_CLIENT_ID=
LINKEDIN_CLIENT_SECRET=
TIKTOK_CLIENT_KEY=
TIKTOK_CLIENT_SECRET=
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
```

Opcional acumulada:

```env
META_SYSTEM_USER_ACCESS_TOKEN=
```

> Nota histórica: V25 fue una base intermedia. No desplegar esta fase por separado; el destino final es V26.


---

# FASE V26 — Production / Enterprise Hardening

V26 es la fase final de infraestructura y seguridad de la rama acumulativa.

## Infraestructura

- PostgreSQL como base principal mediante `DATABASE_URL`.
- Render Key Value / Redis mediante `REDIS_URL`.
- Persistent Disk conservado para uploads y stores WhatsApp QR heredados.
- `BACKGROUND_WORKERS_ENABLED` y `CHANNEL_RUNTIMES_ENABLED` para separar responsabilidades de procesos.
- `MAINTENANCE_MODE` para un corte seguro y repetible.

## Migración

Incluye `/app/worktic-migrate` para copiar el SQLite actual a PostgreSQL preservando IDs y verificando conteos. `app_sessions` no se migra.

Incluye:

```text
/app/scripts/pre_cutover_backup.sh
/app/scripts/post_cutover_check.sh
```

## Seguridad

- Se elimina el admin inicial inseguro.
- Passwords nuevos: Argon2id.
- Hash legacy/bcrypt se actualiza automáticamente tras login correcto.
- Sesiones almacenadas como hash SHA-256.
- Rate limiting Redis.
- CSP/HSTS/headers de seguridad.
- Origin checks para mutaciones API.
- request IDs y logs HTTP.
- auditoría `security_audit_v26`.
- Automation webhooks HTTPS con protección SSRF.

## Nuevas tablas V26

```text
security_audit_v26
production_migrations_v26
```

## Nuevas variables V26

```env
DATABASE_DRIVER=postgres
DATABASE_URL=
REDIS_URL=
REDIS_REQUIRED=true
TRUST_PROXY=true
BACKGROUND_WORKERS_ENABLED=true
CHANNEL_RUNTIMES_ENABLED=true
LEGACY_WHATSAPP_DRIVER=sqlite
LEGACY_WHATSAPP_DSN=file:/var/data/worktic.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)
```

Temporal durante migración:

```env
MAINTENANCE_MODE=true
```

Opcional:

```env
AUTOMATION_WEBHOOK_ALLOWED_HOSTS=
```

Emergencia solamente:

```env
ALLOW_SQLITE_PRODUCTION=false
```

Para instalación vacía únicamente:

```env
BOOTSTRAP_ADMIN_EMAIL=
BOOTSTRAP_ADMIN_PASSWORD=
```

Consultar `VARIABLES_FINALES_V26.md` y `DESPLIEGUE_FINAL_V26_RENDER.md` antes de tocar producción.

---

# V27 — Growth Command Center

V27 añade WhatsApp Marketing, Worktic Copilot global y Dashboard Intelligence sobre la base de producción V26.

- WhatsApp Marketing utiliza conexiones Cloud oficiales y plantillas aprobadas.
- Solo incluye leads con consentimiento (`marketing_leads.consent=1`).
- Opt-out persistente desde respuestas SALIR/STOP/CANCELAR.
- Scheduling, rate limit, estados Meta y métricas persistentes.
- Copilot usa `OPENAI_API_KEY` existente y diagnóstico por tenant.
- Dashboard agrega revenue, ventas, leads, conversaciones, embudo, fuentes, operación e insights.
- No añade variables de entorno obligatorias.
- No requiere repetir la migración SQLite → PostgreSQL.

---

# V27.5 — Stability + Streaming Copilot

- Corrige el binding obsoleto de `metaConnectInfo` que podía detener `app.js`.
- Estados compartidos inicializados antes del bootstrap para evitar TDZ en Agentes IA, Grupos, Landings y módulos dependientes.
- Inbox V21 corrige error ASI/`forEach` y normaliza colecciones de API.
- Worktic Copilot usa streaming real mediante `/api/copilot/v27/stream`.
- Render progresivo de respuesta y autoscroll inteligente que no interrumpe al usuario si lee mensajes anteriores.
- Fallback al endpoint tradicional si el stream no puede iniciar.
- Cache busting V27.5.
- Sin migraciones nuevas de base de datos.
