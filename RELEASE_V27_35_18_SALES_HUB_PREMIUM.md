# WorkticAI V27.35.18 — Sales Hub Premium

**Base requerida: V27.35.17 corregida y validada.** Esta actualización no sustituye el CRM ni el flujo existente de catálogo, ventas o Agenda. Conserva el menú y el estilo claro de WorkticAI.

## Entregado en esta versión

- Centro de Oportunidades con vistas **Seguimiento**, **Embudo** (agrupado), **Lista**, filtros y exportación CSV.
- Resumen de oportunidades abiertas, negocios que necesitan seguimiento, valor estimado y cierre comercial. Los importes de monedas distintas **no se suman**.
- Ficha lateral: cliente, título, etapa, enlace al teléfono, acceso a Conversaciones, Agenda y Catálogo, productos ya asociados, tareas, notas e historial.
- Tareas con fecha y hora, asignación, completar/reabrir y alertas visibles por vencimiento o ausencia de próxima acción.
- Responsable seleccionable desde usuarios activos **de la misma empresa**, moneda COP/USD/USDT/EUR y motivo de pérdida.
- Eventos de tareas y notas guardados en una transacción con su modificación; registro de creación/edición/archivo de la oportunidad en la API CRM principal (este último registro es de mejor esfuerzo).
- El embudo incluye la etapa «Contactado» omitida en el tablero anterior. Arrastrar entre grupos cambia la etapa sin tocar inventario ni pagos.
- En las consultas de últimas conversaciones se eliminó la excepción de mensajes `tenant_id=0` para evitar lecturas de información sin empresa asignada.
- Refresco de información cada 45 segundos si la vista está visible y no hay una ficha abierta.
- Esquema nuevo aditivo y compatible con PostgreSQL/SQLite: `opportunity_hub_meta_v273518`, `opportunity_hub_tasks_v273518` y `opportunity_hub_events_v273518`.

## Control de permisos

Todas las consultas de la nueva API restringen por tenant. Los responsables se validan contra `app_users.tenant_id` y `active=1`. Administradores/propietarios/supervisores pueden reasignar y actualizar; los demás usuarios solo pueden modificar seguimiento si son responsables de la oportunidad. El endpoint original de oportunidades conserva su modelo de acceso por tenant, salvo restricciones de modificación/archivo para usuarios no responsables. **No se declara auditado el conjunto de APIs CRM heredadas**, que debe verificarse durante las pruebas de seguridad previas a clientes reales.

## No confundir con funciones aún no implementadas

- Un negocio en estado «Ganado» **no confirma el pago**. Solo el flujo de ventas y verificación puede registrar pagos reales.
- No se crean ni aprueban pagos desde Sales Hub; el enlace a Catálogo conduce al flujo actual de registro de pagos.
- «Conversaciones» abre el módulo correspondiente; todavía no selecciona automáticamente el hilo exacto.
- Los recordatorios se destacan en la interfaz, pero no se envían todavía mensajes, correos o notificaciones automáticas.
- No se generan documentos PDF de cotización ni se crea un sistema paralelo de cotizaciones.
- Los eventos anteriores al despliegue no se reconstruyen y los cambios realizados directamente en SQL pueden no quedar auditados.
- La integración con agentes IA sigue usando los permisos y reglas opcionales de V27.35.17: no se habilitan nuevas capacidades de agentes automáticamente.

## Rutas y archivos

- `GET/POST /api/crm/sales-hub?opportunity_id=<id>` — tareas, actividad y metadatos.
- `main.go` — inicialización del esquema y registro de ruta.
- `crm_opportunities_premium.go` — «Contactado», auditoría adicional y filtrado de mensajes por empresa.
- `db_compat.go` — soporte para IDs de las tablas nuevas en PostgreSQL.
- `static/opportunities-premium.js` — etapa Contactado en el formulario anterior.
- `static/sales-hub-v273518.js/.css` — nueva experiencia premium sobre el módulo existente.

## Pruebas en Windows antes de publicar

1. Ejecutar el instalador con Windows PowerShell y Go 1.26; debe finalizar con `V27.35.18 VALIDADA LOCALMENTE`.
2. Respaldar la base PostgreSQL persistente antes de publicar cambios con migraciones.
3. Entrar en WorkticAI → Oportunidades: probar Seguimiento, Embudo, Lista y CSV.
4. Crear una oportunidad, asignar un responsable, añadir tarea y nota, reabrir la tarea y verificar el historial.
5. Crear una oportunidad con productos desde Catálogo y verificar que aparecen en la ficha lateral.
6. Probar usuarios de dos empresas: intentar consultar/modificar una oportunidad de la otra empresa. Debe fallar.
7. Verificar que marcar Ganada no altera ventas, pagos ni inventario.
8. Verificar con un agente IA autorizado que la asociación de productos V27.35.17 sigue funcionando.
9. Verificar que el servicio se mantiene en estado Live y que PostgreSQL arranca sin errores SQL.

**Validación en este entorno:** JS `node --check`, parseo Go (gofmt + parser), simulación de migraciones SQLite, navegación Chromium escritorio/móvil. No es posible completar aquí Go 1.26 ni probar el esquema contra PostgreSQL real; el instalador bloquea la publicación si la suite Go falla.
