# WorkticAI V27.30 — Endurecimiento multiempresa y rutas heredadas

**Base:** ZIP completo V27.29. **No se ha desplegado a GitHub ni Render.** Los cambios están desactivados por defecto.

## Cambios verificables
1. Cuando `WORKTIC_COMMERCIAL_ENABLE=1` y `WORKTIC_TENANT_HARDENING_V2730=1`, el middleware detiene 6 endpoints que usan sesiones o configuraciones globales (`/api/connect`, `/api/telegram`, `/api/messenger`, entre otros). El Inbox que ya filtra `tenant_id` conserva las rutas `/api/conversations`, `/api/messages`, `/api/read` y `/api/contact`, y `/api/send` continúa habilitado para `wacloud:` y `messenger:`; los envíos por sesión global se bloquean. No se bloquean los webhooks firmados de terceros ni las rutas modernas tenant-scoped.
2. `/api/rules` no puede entrar a la tabla global cuando está activado el modo nuevo. Para editar reglas hace falta activar también `WORKTIC_CONSUMPTION_GUARD_V2723=1` y `WORKTIC_TENANT_RULES_V2723=1` en staging. Las reglas legacy no se reasignan ni se borran.
3. Si faltan dependencias (`WORKTIC_MODULE_GATES_V2722`, `WORKTIC_RESOURCE_GUARD_V2724`, `WORKTIC_CONSUMPTION_GUARD_V2723`, `WORKTIC_TENANT_RULES_V2723`, contratos V27.29), las rutas funcionales se bloquean con 503 en vez de funcionar sin control.
4. Altas de canales en `/api/channels/connections` ahora usan un bloqueo por empresa, conteo total/por tipo e inserción atómicos, incluyendo registro de auditoría en la misma transacción.
5. Sincronización del CRM desde chats/leads (`syncCRMContactAt`) valida el máximo contratado bajo el mismo bloqueo. Incluye nuevos contactos y reactivaciones de eliminados; actualizaciones ordinarias no gastan otro contacto. **Cuando el cupo se agota devuelve error**. Revisar cómo cada importador maneja ese error y no perder leads.
6. Los recursos V27.24 consultan el snapshot de contrato, cuando el endurecimiento y el versionado de V27.29 están activados.
7. `GET /api/admin/commercial/v2730/coverage` muestra bandera, dependencias y pendientes al superadministrador. Nunca aprueba producción.

## Limitaciones críticas
- Las rutas globales legacy quedan bloqueadas al activar el piloto. No activar hasta comprobar que la interfaz actual usa los endpoints empresariales. Los webhooks antiguos sin tenant deben migrarse.
- El filtro de módulos V27.22 sigue opt-in y NO cubre automáticamente todos los servicios internos/cron/webhooks. Cerrar otras rutas automáticas en V27.31/32 antes de vender.
- La sincronización inicial histórica hace best-effort en algunas rutas: faltan colas/dead letters y reconciliación para contactos rechazados por cupo.
- Las operaciones de proveedor, cuotas y contrataciones aún requieren PostgreSQL real, Go 1.26, pruebas multiinstancia y conciliación económica.
- Ningún precio de Meta/WhatsApp ni rentabilidad real se certifica aquí.

## Evidencia de aceptación (staging exclusivamente)
- Dos tenants simultáneos, 5 canales permitidos en uno: 20 altas paralelas jamás crean 6. Una solicitud de otro tenant no altera el conteo.
- Un mismo tenant con contactos al cupo: un webhook importador no puede crear contactos adicionales; actualizar un contacto existente continúa funcionando.
- `/api/telegram` y `/api/messenger` devuelven 403; `/api/send` acepta solo destino tenant-scoped `wacloud:` o `messenger:`. Los endpoints de lectura tenant-scoped del Inbox permanecen accesibles.
- `/api/rules` lee únicamente reglas por empresa cuando el piloto de tenant rules está habilitado; no vuelve a la tabla global.
- Contrato Empresa/Negocio versionado: una edición del catálogo no amplía beneficios ya pagados.
- Revisar y corregir todas las llamadas heredadas desde `static/` antes de activar.

## Pruebas y estado
- Comprobación AST del código Go y pruebas unitarias aisladas posibles sin dependencias de red.
- **Pendientes**: compilación Go 1.26 (`go test ./... && go build ./...`), staging PostgreSQL, migración de frontend, colas de error y regresión end-to-end.
- Esta versión cuenta como progreso de la fase 2/5; no certifica cierre completo de todos los módulos.
