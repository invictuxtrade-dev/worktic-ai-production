# Messenger Webhook Receiver V3

Corrección basada en Worktic_AI_Messenger_Token_Validation_V2_Sin_Messenger_Profile.zip.

## Cambios
- Registra cada POST recibido desde Meta aunque sea un evento de prueba.
- Distingue entre "Webhook recibido" y "Mensaje real recibido" en el diagnóstico.
- Acepta los dos formatos de payload observados en Meta:
  - `entry[].messaging[]`
  - `entry[].changes[].value` para el campo `messages`
- Ignora la conversación ficticia del botón **Probar** (`test_message_id`) para no contaminar CRM y bandeja.
- Conserva el evento de prueba como prueba de conectividad.
- Registra en Render el formato detectado y la cantidad de eventos.
- Mantiene el procesamiento de texto, quick replies y postbacks reales.

## Logs esperados
- Evento de prueba:
  `[messenger webhook] ... shape=entry.changes.messages events=1`
- Mensaje real:
  `[messenger webhook] inbound message stored ...`
