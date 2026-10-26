# WorkticAI V27.24 — Control transaccional y medición de recursos (solo staging)

**Base real del paquete:** ZIP completo WorkticAI V27.23. Este release es incremental, no borra versiones previas ni se publica en GitHub/Render. Requiere Go 1.26, PostgreSQL + Redis separados de producción para certificar el despliegue. No modifica el plan de compensación de Worktic Partners.

## Cambios implementados

| Módulo / ruta | Control nuevo | Alcance verdadero |
|---|---|---|
| Productos, `POST /api/products` | `COUNT` e `INSERT` dentro de bloqueo/transacción por empresa con `billing_plans.max_products` | Alta manual. Carga/importaciones/otros escritores pendientes de homogeneizar |
| Contactos, `POST /api/crm/contacts` | Búsqueda de duplicado, actualización o alta y cuota `max_contacts` en la misma transacción por tenant | Alta manual; sincronización de webhooks, importación, datos históricos y externos siguen requiriendo revisión |
| Agenda, `POST /api/appointments` | Cuota de citas activas y validación de horario en transacción por tenant | No controla citas creadas desde otras rutas ni costos de notificación |
| Agenda, `POST /api/agenda/professionals`, `/api/agenda/services` | Cuota de recursos por tenant, con variables explícitas por plan | No incluye límites de horas o bloqueos, quedan para la próxima certificación |
| Analytics, grabación de eventos `recordAnalyticsEventV24` | Metrificación mensual de eventos nuevos con reserva atómica, deduplicación del `event_key` y alta en misma transacción | **Si se agota la cuota se omite el evento y registra error técnico**. No habilitar para clientes sin política de retención/UX y medición de no-atribuidos; su uso está condicionado a staging |
| Ads Center, `syncAdsV25Tenant` | Cuenta cada solicitud de sincronización desde interfaz y cron cada 6h, incluso intentos fallidos | Número de solicitudes **no** equivale a gasto de publicidad. No cubre fan-out de cada cuenta ni llamadas secundarias de APIs; revisar rate limits y costo por proveedor |
| Estatus, `GET /api/consumption/v2724/resources` | API de solo lectura para empresa actual, `tenant_id` alternativo solo para superadmin, devuelve cupos y contadores | Recursos son recuentos actuales; Analytics/Ads son intentos mensuales, no facturas |
| Studio | Panel responsive de «Mis límites y consumo» | La interfaz refleja lectura de la API, no genera pagos ni cambia cupos |

## Activación y rollback

Dos condiciones simultáneas: `WORKTIC_COMMERCIAL_ENABLE=1` y `WORKTIC_RESOURCE_GUARD_V2724=1`. Por defecto apagado. Con flag apagado el código conserva la conducta de V27.23. Todas las tablas nuevas son aditivas. El control por plan para productos/contactos depende de `billing_plans.max_products` y `max_contacts` efectivos. Los de Agenda/Analytics/Ads se declaran mediante variables como `WORKTIC_APPOINTMENTS_NEGOCIO_V2724`, `WORKTIC_ANALYTICS_EVENTS_EMPRESA_V2724` y `WORKTIC_ADS_SYNCS_EMPRESA_V2724`; **sin valor explícito se asigna 0 y se bloquea el recurso** para los planes piloto.

No habilitar en producción hasta que estén comprobados los planes, las migraciones, backfill y todas las rutas creadoras. No es correcto habilitar restricciones parciales sin considerar procesos internos; podría impedir citas, análisis o sincronizaciones que el cliente pagó.

## Guía de pruebas antes del lanzamiento

1. Validar `go test ./...` y `go build ./...` usando Go 1.26 y dependencias completas. No se ha ejecutado en este sandbox (Go 1.23).
2. Crear 2 empresas staging con cuentas owner y planes distintos. Ejecutar 25+ altas POST concurrentes de productos y contactos con credenciales de dos tenants. Validar que nunca supera el cupo, que UPDATE/DELETE no gasta cupo, que una empresa no lee datos ajenos, y que las altas deduplicadas no gastan cupo doble.
3. En Agenda, intentar citas solapadas con dos workers reales, distintos profesionales y status Cancelada. Probar zona horaria, RFC3339, horario de verano del servidor, cancelación y recalendarización.
4. Analytics: reinsertar claves de evento iguales, generar eventos sin clave, reintentos y eventos > cupo; comprobar ausencia de atribución perdida antes de habilitar en plan comercial. No perder eventos críticos silenciosamente.
5. Ads: con proveedores simulados y dos instancias del worker, comprobar cron de 6 horas, errores/reintentos y cuota con simultaneidad. Confirmar que no se envía llamada externa después de agotar cupo. Registrar explícitamente gasto publicitario como gasto del cliente, no del SaaS salvo casos de agencia.
6. Configurar alertas de cuota y observabilidad; evaluar retención PostgreSQL, tamaño por evento y transferencia.
7. Probar en staging: `GET /api/consumption/v2724/resources`, roles y `?tenant_id=` cross-tenant. Revisión de privacidad, respuestas 403 y banner en la interfaz.
8. Confirmar respaldo/reversión de cada ruta. Nunca ejecutar migraciones SQLite→PostgreSQL anteriores, ni reutilizar la BD de producción.

## Criterios financieros (sin inventar costes)

- `crm_contacts`: crecimiento de BD, índices, auditoría, soporte y almacenamiento de adjuntos; costo unitario **pendiente de medir**.
- `crm_products`: número de registros, imágenes/video subidos, salida de red y retención; costo unitario **pendiente**.
- `crm_appointments`: consultas, calendario, correos/SMS/WhatsApp según servicio; costo de mensajes **desconocido hasta conciliación**.
- `analytics_attribution_events_v24`: eventos/mes y bytes por evento, índices, consultas/exportaciones, CPU + PostgreSQL; costo por evento **pendiente**.
- `ads_connections_v25`: solicitudes/mes, cuentas por cliente, proveedor/red y errores de cuota; gasto de campaña **no es costo del proveedor SaaS** si lo financia el anunciante.

## Bloqueadores para afirmar "todos los módulos listos"

**P0**: validar cuotas de rutas automáticas/importadoras de CRM y Agenda, consolidar coste real de OpenAI/Luma/WhatsApp/Meta, controlar acciones pagas de workflows, reconciliar cobros/recargas/reembolsos, probar suscripción/renovación/anualidad y comisiones Partners, staging PostgreSQL y pruebas 2 workers, seguridad entre tenants y rollback.

**P1**: política de pérdida cero de eventos de Analytics, presupuestos separados para sync Ads / fan-out por cuenta, almacenamiento/exportaciones y panel global de margen real por plan, rutas históricas de automatización y canales.

**Conclusión**: entrega de ingeniería lista para pruebas de staging, no para vender públicamente ni para activar con clientes. Los números de consumo son métricas, nunca sustituyen facturas reales.
