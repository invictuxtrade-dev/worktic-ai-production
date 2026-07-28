# Messenger Webhook Receiver V6 — Standby + Takeover

Esta versión corrige el caso donde Meta entrega una conversación existente dentro de `entry[].standby[]` porque otra aplicación o Meta Inbox todavía conserva el control del hilo.

Cambios:

- Parser explícito para `entry[].messaging[]`.
- Parser explícito para `entry[].standby[]`.
- Identificación de mensajes recibidos en estado standby.
- Registro de auditoría `messenger_standby_message`.
- Intento automático de `take_thread_control` para la conversación.
- Respaldo con `request_thread_control` cuando Meta no permite la toma directa.
- El mensaje entrante se almacena aunque Meta rechace temporalmente la toma de control.
- Auditoría del resultado en `messenger_thread_control`.
- No cambia URL, token, Page ID, suscripciones ni otros módulos.

Suscripciones recomendadas en Meta:

- messages
- messaging_postbacks
- message_reads
- message_deliveries
- standby
- messaging_handovers

Después del despliegue no es necesario reconfigurar Meta. Se debe enviar un mensaje nuevo desde el evaluador y revisar el diagnóstico.
