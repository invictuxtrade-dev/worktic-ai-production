# WorkticAI V24 — Analytics & Attribution

## Objetivo

V24 convierte los datos dispersos de Social Hub, Growth, CRM, Inbox, Agenda, Agentes IA y Oportunidades en una capa transversal de inteligencia de negocio. La meta no es sumar métricas vanity, sino reconstruir el recorrido verificable desde adquisición hasta revenue.

Esta versión se construye sobre V23.1 y sigue perteneciendo a la rama acumulativa que **no debe desplegarse todavía de forma aislada**.

## Nuevo módulo global: Analytics & Attribution

Se añade una entrada principal `Analytics` en WorkticAI con:

- KPIs de revenue, ventas, leads, citas, oportunidades, CAC, CPL, ROAS y ticket medio.
- Embudo completo: clic rastreado → lead → conversación → cita → oportunidad → venta.
- Evolución diaria.
- Rendimiento por canal/fuente.
- Rendimiento por campaña.
- Contenido que genera negocio.
- Resultados por responsable comercial.
- Resultados por Agente IA asociado.
- Customer Journeys de ventas ganadas.
- Exportación CSV de ventas atribuidas.

## Modelos de atribución

V24 incorpora dos lecturas configurables por tenant:

### Last Touch

Atribuye el resultado al último touch rastreable antes de la conversión.

Útil para responder:

- ¿qué contenido terminó convirtiendo?
- ¿qué canal cerró el recorrido?

### First Touch

Atribuye el resultado al primer touch rastreable conservado por Worktic.

Útil para responder:

- ¿qué canal descubrió al cliente?
- ¿qué contenido inició la adquisición?

El modelo se puede cambiar desde Analytics. Owner/Admin puede definir el modelo predeterminado del tenant.

## Tracking propio de Worktic

V24 añade enlaces de tracking para publicaciones con URL.

Al publicar un social post con enlace, el proveedor recibe una URL Worktic similar a:

```text
https://TU-DOMINIO/t/{tenant_id}/{social_post_id}
```

Cuando un usuario hace clic:

1. Worktic registra el evento `content_click`.
2. Conserva First Touch durante 90 días si todavía no existe.
3. Actualiza Last Touch.
4. Redirige al destino original.
5. Añade parámetros de atribución cuando corresponda:
   - `utm_source`
   - `utm_medium=social`
   - `utm_campaign`
   - `wt_post`
   - `wt_campaign`
   - `wt_channel`
   - `wt_source`

No se cambia el destino comercial guardado por el usuario; `tracking_url` se almacena por separado en `social_posts`.

## Landing Pages y formularios

Las landings/formularios Worktic propagan automáticamente los parámetros de atribución hacia el formulario.

Al registrar un lead se conservan:

- landing;
- post social;
- campaña;
- canal;
- first touch;
- last touch;
- UTM existente.

Esto permite reconstruir posteriormente:

```text
Publicación
→ clic Worktic
→ landing
→ lead
→ oportunidad
→ cita
→ venta
```

## CRM y oportunidades

V24 enriquece automáticamente oportunidades existentes/nuevas cuando puede resolver su lead de origen:

- `lead_id`
- `campaign_id`
- `social_post_id`
- `agent_id`

No sustituye `source`, `source_ref` ni `channel`; esos campos operativos siguen intactos.

## Agenda

Las citas se vinculan progresivamente con:

- contacto CRM;
- oportunidad;
- campaña;
- source/source_ref.

La vinculación automática usa teléfono/contacto y la oportunidad comercial más relevante disponible.

## Agentes IA y equipo humano

Analytics muestra:

- oportunidades/ventas/revenue por `owner` comercial;
- oportunidades/ventas/revenue por Agente IA asociado cuando existe una relación verificable.

Una venta sin agente/owner asociado se conserva como no asignada; Worktic no inventa atribución.

## Revenue y moneda

V24 usa el valor de oportunidades en etapa `Ganado` como revenue comercial.

Cada tenant puede configurar moneda de visualización:

- USD
- COP
- EUR
- MXN

Importante: V24 no realiza conversión FX. Se asume que un tenant registra oportunidades y gasto en una moneda operativa coherente. La normalización multimoneda puede añadirse después si el negocio la requiere.

## Spend, CPL, CAC y ROAS

Spend proviene de `marketing_metrics`.

Fórmulas:

```text
CPL = Spend / Leads
CAC = Spend / Ventas
ROAS = Revenue / Spend
Lead → Sale Rate = Ventas / Leads × 100
Ticket medio = Revenue / Ventas
```

Si aún no existe una integración de Ads que alimente gasto real, Spend será 0 y Worktic mostrará métricas derivadas en 0 en lugar de inventarlas. V25 será la fase de Ads Center.

## Customer Journey

La tabla de journeys cerrados muestra, cuando existe la relación:

- contacto;
- oportunidad;
- revenue;
- canal;
- campaña;
- contenido social;
- plataforma;
- agente IA;
- responsable comercial;
- fecha de cierre.

## Exportación

Endpoint:

```text
GET /api/analytics/v24/export.csv
```

Exporta ventas ganadas dentro del periodo visible con sus dimensiones atribuibles.

## Nuevos endpoints

```text
GET /api/analytics/v24/overview
GET /api/analytics/v24/journeys
GET /api/analytics/v24/settings
PUT /api/analytics/v24/settings
GET /api/analytics/v24/export.csv
GET /t/{tenant_id}/{social_post_id}
```

## Nuevas tablas

```text
analytics_attribution_events_v24
analytics_settings_v24
```

## Migraciones automáticas

Se añaden columnas de atribución a:

- `social_posts`
- `marketing_leads`
- `crm_opportunities`
- `crm_appointments`

No requiere SQL manual.

## Sincronización

V24 mantiene un sincronizador interno periódico que:

- enlaza leads con contactos;
- completa oportunidades con lead/campaña/post/agente cuando puede verificarse;
- enlaza citas con contactos/oportunidades;
- crea eventos de atribución idempotentes;
- registra la primera conversación por canal.

El endpoint de Analytics también ejecuta sincronización antes de calcular el reporte para que una actualización reciente no tenga que esperar al siguiente ciclo.

## Seguridad y multi-tenant

Todos los queries y eventos V24 están filtrados por `tenant_id`.

La configuración del modelo/moneda solo puede modificarse por Owner/Admin. Los demás perfiles pueden consultar Analytics y cambiar temporalmente la vista First/Last Touch sin cambiar la configuración global.

## Limitaciones intencionales

V24 no afirma más de lo que puede comprobar:

- No existe atribución retroactiva por publicación para clics ocurridos antes de instalar V24.
- Un clic hacia un sitio externo solo puede mantenerse hasta la venta si ese sitio devuelve el identificador/UTM a Worktic o usa formularios/landings Worktic.
- El tracking First/Last Touch por cookie es por navegador/dispositivo; no es identidad cross-device.
- Las métricas sociales dependen de permisos/API del proveedor.
- Spend depende de `marketing_metrics`; V25 conectará Ads de forma más profunda.
- Revenue se basa en oportunidades marcadas como `Ganado`.

Cuando falta una relación, la interfaz muestra `direct`, `sin campaña`, `sin contenido`, `sin agente` o `sin asesor`.

## Variables de entorno

V24 **no añade variables nuevas**.

`BASE_URL` ya estaba en la lista acumulativa y ahora pasa a ser especialmente importante porque los enlaces rastreables se generan con ese dominio. En producción debe ser HTTPS y corresponder al dominio público real de WorkticAI.

## Checklist para el despliegue final

48. Abrir Analytics y validar filtros de fechas.
49. Configurar moneda correcta del tenant.
50. Probar Last Touch y First Touch con el mismo lead.
51. Publicar un contenido con enlace desde Social Hub.
52. Confirmar que el enlace publicado usa `/t/{tenant}/{post}`.
53. Hacer clic y confirmar redirección al destino original.
54. Completar una landing/formulario Worktic y confirmar `social_post_id`/campaña en el lead.
55. Confirmar Lead → Oportunidad.
56. Crear/confirmar una cita y revisar atribución.
57. Marcar la oportunidad como `Ganado` y verificar revenue en Analytics.
58. Comparar First Touch vs Last Touch.
59. Verificar campaña, contenido, owner y agente asociados.
60. Exportar CSV y verificar aislamiento por tenant.
61. Confirmar que métricas sin evidencia aparecen en 0/no atribuidas y no como datos inventados.
