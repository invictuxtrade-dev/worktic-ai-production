# Messenger Webhook Receiver V5 — Parser universal

Esta versión parte de Messenger Webhook Receiver V4 y corrige el caso donde Meta alcanza el webhook, pero el mensaje real no se clasifica.

## Cambios

- Parser recursivo compatible con:
  - `entry[].messaging[]`
  - `entry[].changes[].value.message`
  - `entry[].changes[].value.messages[]`
  - campos `sender/recipient`, `from/to` y variantes por ID
- Soporte para texto, respuestas rápidas, postbacks, adjuntos y stickers.
- Deduplicación de candidatos extraídos por distintas rutas.
- Registro de la ruta exacta del payload que originó el mensaje.
- Auditoría del hash y una vista previa limitada del POST para diagnosticar cambios futuros de Meta.
- Los eventos sintéticos `test_message` continúan sin crear contactos ni conversaciones.

## Después de desplegar

No es necesario modificar URL, Page Access Token, verify token ni suscripciones en Meta.

1. Envía un mensaje nuevo desde el evaluador.
2. Abre Canales > Messenger > Diagnóstico.
3. Deben quedar verdes `Webhook recibido` y `Mensaje real recibido`.
4. Revisa Conversaciones con el filtro Messenger.
