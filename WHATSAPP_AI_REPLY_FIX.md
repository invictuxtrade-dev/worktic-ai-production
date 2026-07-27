# Corrección WhatsApp + IA multitenant

## Problema encontrado
Los mensajes entrantes de las conexiones creadas desde **Canales** sí se almacenaban en la bandeja, pero el manejador multicuenta no llamaba al motor de respuesta automática. Además, el módulo de agentes resolvía el espacio por ID del propietario en vez del `tenant_id` empresarial real.

## Correcciones
- El manejador de WhatsApp multitenant ahora invoca la IA tras guardar cada mensaje entrante.
- La respuesta se envía por la misma conexión de WhatsApp que recibió el mensaje.
- Se conserva el historial por empresa, canal y conversación.
- Se utiliza primero el agente especializado asignado al canal.
- Si no existe un agente especializado asignado, se usa el Asistente Principal clásico cuando está habilitado.
- Los mensajes enviados por IA se guardan en la bandeja como `ai_sent`.
- Se registran errores del proveedor y métricas del agente especializado.
- `agentTenant()` ahora usa el `tenant_id` empresarial real.
- Se añadió una migración para normalizar agentes creados por versiones anteriores.

## Verificación en producción
1. Confirma `OPENAI_API_KEY` y `OPENAI_MODEL` en Render.
2. Activa el Asistente Principal o crea un agente especializado con estado `active`.
3. Asigna el agente al canal de WhatsApp o déjalo como principal.
4. Reinicia el servicio después del despliegue.
5. Envía un mensaje desde un número externo.
