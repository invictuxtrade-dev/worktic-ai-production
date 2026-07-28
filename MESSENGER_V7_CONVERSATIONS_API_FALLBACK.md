# Messenger V7 — Webhook + Conversations API fallback

Esta versión conserva el webhook en tiempo real y añade una segunda vía oficial de recuperación mediante la Conversations API de Meta.

## Motivo

Meta confirma la conectividad del webhook, pero en algunas configuraciones el evento de mensaje real puede retrasarse u omitirse aunque la conversación exista en la bandeja de la página. Worktic ya no depende de una sola vía.

## Cambios

- Sincronización automática de conexiones Messenger activas cada 30 segundos.
- Intervalo configurable mediante `MESSENGER_SYNC_INTERVAL_SECONDS` (mínimo 15, máximo 300).
- Consulta de conversaciones y mensajes usando el Page Access Token y `pages_messaging`.
- Importación deduplicada por Message ID.
- Creación/actualización de conversación, contacto CRM y oportunidad.
- Respuesta IA únicamente a mensajes nuevos y recientes; el historial antiguo se importa sin responder automáticamente.
- Acción manual `sync` en `/api/channels/action`.
- Botón «Sincronizar Messenger ahora» en el diagnóstico.
- Estado de la última sincronización, conversaciones revisadas y mensajes importados.

## Archivos modificados

- `main.go`
- `channels_v13.go`
- `messenger_channels.go`
- `static/app.js`

## Archivo nuevo

- `messenger_sync_fallback.go`
