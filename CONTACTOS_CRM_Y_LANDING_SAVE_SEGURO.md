# Contactos CRM unificados y guardado seguro de Landing Pages

## Contactos

- Los contactos entrantes de WhatsApp, Telegram y Messenger se sincronizan automáticamente con el CRM por `tenant_id`.
- Los leads capturados por formularios públicos y landing pages crean o enriquecen la ficha CRM.
- La deduplicación usa teléfono normalizado, correo e identificador externo del canal.
- Los registros históricos de conversaciones y formularios se migran automáticamente al iniciar la aplicación.
- El módulo incorpora búsqueda, filtros por canal, origen, etapa y fechas, orden, estadísticas y acciones para editar, eliminar, abrir conversación y crear oportunidad.
- La eliminación es segura: oculta la ficha sin borrar conversaciones ni leads; una interacción futura puede reactivarla.
- La cuota para contactos manuales se calcula por tenant, no globalmente.

## Landing Pages

- Se corrigió la validación del plan: ahora utiliza la cuenta propietaria de facturación y no confunde `tenant_id` con `user_id`.
- El guardado usa transacción, confirma el ID creado y devuelve errores JSON claros.
- Se validan slug, contenido JSON y relaciones con formularios/campañas del mismo tenant.
- El navegador conserva un borrador local automático mientras se edita.
- Si hay error, sesión vencida o timeout, el modal permanece abierto y los datos pueden recuperarse.
- El borrador local se elimina solo cuando el servidor confirma el guardado.

## Aislamiento de cambios

- `static/app.js` y `static/styles.css` permanecen idénticos a la base estable suministrada.
- La nueva interfaz de Contactos vive en archivos separados.
- La protección de guardado de Landing Pages vive en archivos separados.
