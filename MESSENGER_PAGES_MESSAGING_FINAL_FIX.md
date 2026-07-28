# Messenger Final Fix — pages_messaging

Esta versión corrige la conexión multi-tenant de Messenger sin exigir `pages_read_engagement`.

## Cambios principales

- Se eliminó la validación bloqueante mediante `/me?fields=id,name`.
- El Page Access Token se valida con `/debug_token` cuando existen App ID y App Secret.
- Si no hay App Secret, se valida directamente mediante endpoints de Messenger Platform que requieren `pages_messaging`.
- Se comprueba que el token sea válido, tenga `pages_messaging` y corresponda al Page ID indicado.
- La consulta del nombre de la página es opcional y nunca bloquea la conexión.
- El token de verificación se genera automáticamente, se guarda y permanece estable.
- El webhook puede ser verificado por Meta aun antes de recibir el primer mensaje.
- Se conservan el Page Access Token y el App Secret al reconfigurar dejando los campos vacíos.
- Se retiró una función JavaScript heredada que sobrescribía el conector nuevo.
- El diagnóstico muestra el método de validación, permiso `pages_messaging`, coincidencia de Page ID, URL y token de verificación.
- Los mensajes manuales desde la bandeja usan la conexión Messenger correcta del tenant.
- El webhook procesa texto, respuestas rápidas y postbacks.

## Configuración después del despliegue

1. Abre **Canales → Messenger → Reconfigurar**.
2. Ingresa el **Page ID** y el **Page Access Token** actual.
3. El Meta App ID es recomendado. El App Secret es opcional, pero permite usar `debug_token` y validar la firma de los webhooks.
4. Guarda la conexión.
5. Abre **Diagnóstico** y copia la URL de devolución y el token de verificación.
6. En Meta Developers pulsa **Verificar y guardar**.
7. En la página pulsa **Agregar suscripciones** y marca `messages`, `messaging_postbacks`, `message_reads` y `message_deliveries`.
8. Envía un mensaje desde otra cuenta y comprueba **Recepción reciente**.

`pages_read_engagement` ya no es requisito para conectar ni responder por Messenger.
