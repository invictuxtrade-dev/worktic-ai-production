# Messenger — diagnóstico y reconfiguración corregidos

## Correcciones

- El botón **Probar conexión** ya no intenta ejecutar `/{page-id}/subscribed_apps` automáticamente.
- Se evita el falso error de Meta `(#100) Object does not exist / missing permission` cuando la app aún no tiene permisos avanzados.
- El diagnóstico valida token, página, webhook, token de verificación y actividad real.
- La suscripción a eventos se presenta como un paso manual en Meta Developers.
- **Reconfigurar** consulta primero los datos actuales del backend y siempre muestra:
  - URL de devolución de llamada.
  - Token de verificación.
  - Page ID detectado.
- En conexiones existentes, el Page Access Token puede dejarse vacío para conservar el actual.
- El App Secret puede dejarse vacío para conservar el actual.
- El token de verificación existente se conserva y no cambia al reconfigurar.

## Flujo de Meta

1. Copiar URL de devolución y token de verificación desde Worktic.
2. Pegarlos en Meta Developers > Messenger > Configurar webhooks.
3. Pulsar Verificar y guardar.
4. En la página, pulsar Agregar suscripciones.
5. Seleccionar `messages`, `messaging_postbacks`, `message_reads` y `message_deliveries`.
