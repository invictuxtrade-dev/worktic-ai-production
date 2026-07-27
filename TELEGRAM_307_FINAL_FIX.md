# Corrección final Telegram — HTTP 307

Esta versión elimina el bloqueo `Wrong response from the webhook: 307 Temporary Redirect`.

## Funcionamiento

- Antes de registrar el webhook, Worktic prueba la URL pública sin seguir redirecciones de manera invisible.
- Si el dominio redirige hacia otro host o ruta canónica, sigue la cadena de forma controlada.
- Telegram recibe únicamente la URL final HTTPS, que responde directamente sin códigos 301, 302, 307 o 308.
- El botón **Probar conexión** detecta conexiones antiguas con redirección y vuelve a registrar automáticamente el webhook correcto.
- Conserva el secreto del webhook y la separación por empresa/conexión.

## Después de desplegar

1. Abra **Canales**.
2. Pulse **Probar conexión** en Telegram.
3. El diagnóstico debe indicar que el webhook fue reparado automáticamente o que está operativo.
4. Envíe `/start` y luego `Hola` al bot.

No es necesario eliminar la conexión ni volver a pegar el token cuando el botón puede reparar la URL existente.
