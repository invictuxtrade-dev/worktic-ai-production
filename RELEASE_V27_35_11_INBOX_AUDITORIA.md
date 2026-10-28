# WorkticAI V27.35.11 — Conversaciones Premium y Auditoría

Base exacta: V27.35.10. Instalación incremental con verificación SHA-256; no modifica facturación, créditos, Llamadas IA, Chat Web ni Agentes IA.

## Interfaz
- El área central del chat tiene altura acotada, scroll independiente y conserva el cuadro de envío visible cuando hay centenares de mensajes.
- Lista de conversaciones, mensajes y panel de gestión: scroll independiente. Diseño responsive para PC y móvil.
- Botones «Eliminar» y «Historial» para roles superadmin, owner y admin.
- Menú «Eliminadas · Auditoría» para recuperar conversaciones retiradas.
- Eliminación con confirmación y motivo. No borra mensajes, contactos o notas.
- Historial de cambios de estado, modo IA/humano, prioridad, responsable, departamento, etiquetas, SLA, renombre de contacto y notas agregadas.

## Servidor y base de datos
Dos tablas creadas de forma aditiva al arrancar:
- `inbox_removed_v273511`: registro de eliminación reversible por tenant y chat.
- `inbox_audit_v273511`: registro histórico inmutable desde la API (actor, momento, acción, antes, después y motivo).

No se alteran las tablas antiguas. Los dos actos de eliminación+auditoría o edición+auditoría se guardan dentro de una misma transacción.

Endpoints autenticados:
- `POST /api/inbox/remove` body `{ "chat": "...", "reason": "..." }`
- `POST /api/inbox/restore` body `{ "chat": "...", "reason": "..." }`
- `GET /api/inbox/removed`
- `GET /api/inbox/audit?chat=...`

Solo superadmin, owner o admin pueden eliminar, restaurar, consultar eliminadas y leer auditoría; la pertenencia de la conversación a la empresa es obligatoria. Las conversaciones eliminadas no figuran en el listado ni la consulta de mensajes y el motor que respeta `inboxAIAllowed` no responde automáticamente a ellas. Se mantienen los mensajes en la base para revisión administrativa; restaurar permite visualizar el hilo original.

## Límites y advertencias
- «Eliminar» significa quitar de la bandeja de forma reversible. No es borrado irreversible de datos ni aplicación de una política de retención legal.
- Se registran los cambios de gestión realizados a través de estas rutas. Las operaciones de otros módulos no se auditan automáticamente en esta tabla.
- El historial consulta los últimos 200 eventos por conversación; la lista de eliminadas muestra los últimos 100.
- El backend muestra los últimos 500 mensajes por chat (los más recientes) para evitar cargar sin límite el navegador. No elimina mensajes antiguos. La paginación histórica completa sigue como mejora futura.
- La recepción de webhooks externos puede seguir almacenando mensajes en hilos ocultos. Esto es intencional para conservar evidencia. Las respuestas automáticas de rutas que no usan `inboxAIAllowed` deben revisarse antes de una eliminación con efectos jurídicos.
- Es imprescindible probar SQLite/PostgreSQL y datos existentes en staging, con respaldo, antes de activar en producción.

## Checklist de pruebas locales
1. `node --check static/inbox-v21.js`
2. `go test -count=1 ./...`
3. `go build ./...`
4. `git diff --check`
5. Probar empresa A/B con el mismo ID de conversación: eliminar en A no altera B.
6. Probar rol `agent`: acción debe devolver 403.
7. Eliminar una conversación de prueba y confirmar motivo en auditoría, nombre de actor, hora y número de mensajes preservados.
8. Restaurar y comprobar el hilo original.
9. Abrir una conversación con más de 500 mensajes: la caja de escritura no se desplaza fuera del panel.
10. Verificar cambio de responsable/estado y su evento de auditoría.

## Publicación
No ejecutar si el instalador detecta diferencias respecto a V27.35.10. No forzar el parche. Requiere Go 1.26 para validación completa.
