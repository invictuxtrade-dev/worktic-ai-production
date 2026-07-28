# Messenger Token Validation V2

Corrección aplicada sobre la versión Administración Full.

## Problema corregido

La validación alternativa consultaba `/{page-id}/messenger_profile`. En algunas versiones actuales de Graph API Meta responde código 100 indicando que `messenger_profile` no existe como campo del nodo Page, aunque el Page Access Token sea válido y tenga `pages_messaging`.

## Nuevo flujo

1. Si existe Meta App ID + App Secret, Worktic valida con `/debug_token`.
2. Sin App Secret, intenta consultar únicamente `/me?fields=id` para confirmar la identidad de la página sin solicitar nombre, engagement ni contenido.
3. Los errores OAuth 190/102 o HTTP 401 sí bloquean por token inválido o vencido.
4. Errores de permisos, cambios de versión o campos no disponibles ya no producen falsos rechazos.
5. Si Meta no permite una verificación adicional sin App Secret, el token se guarda de forma provisional para completar el webhook y se comprueba funcionalmente al recibir o enviar el primer mensaje.
6. Se eliminó el uso de `messenger_profile` y `conversations` como validadores del token.
7. El conector legado también usa la validación nueva y ya no consulta `fields=id,name` de forma obligatoria.

## Recomendación de producción

Configurar `META_APP_ID` y `META_APP_SECRET` en Render permite realizar la validación estricta mediante `/debug_token` y validar la firma `X-Hub-Signature-256` de los webhooks.
