# Messenger Webhook Receiver V4 — registro duradero

Esta versión corrige el caso en el que Meta confirmaba la prueba del webhook, pero el diagnóstico de Worktic permanecía en gris.

## Cambio principal

El diagnóstico ya no depende únicamente de campos dentro de `channel_connections.config_json`. Cada solicitud y cada etapa del procesamiento se guarda en la tabla existente `channel_events`:

- `messenger_webhook_http`: el POST llegó físicamente a Worktic.
- `messenger_webhook_processed`: el JSON fue leído y clasificado.
- `messenger_webhook_sample`: Meta envió una prueba sintética.
- `messenger_message_received`: llegó un mensaje real de un usuario.
- `messenger_webhook_rejected`: se rechazó por firma o configuración.
- `messenger_processing_error`: ocurrió un error guardando contacto, conversación u oportunidad.

El diagnóstico consulta esta tabla como fuente de verdad. Así, guardar nuevamente una configuración del canal ya no puede borrar el estado de recepción.

## Registro temprano

La llegada se registra inmediatamente después de localizar la conexión y leer el cuerpo HTTP, antes de analizar el formato del evento. Esto permite distinguir entre:

1. Meta no llamó la URL.
2. Meta sí llamó, pero el evento fue rechazado.
3. Meta llamó y el evento fue procesado.
4. Llegó un mensaje real.

## Compatibilidad

Se conservan los marcadores anteriores en `config_json` para instalaciones existentes, pero ya no son la fuente principal del diagnóstico.
