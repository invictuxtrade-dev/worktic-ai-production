# Worktic AI — Messenger Production Hardening

Base: `Worktic_AI_Messenger_V7_Webhook_Mas_Sincronizacion_API.zip`
Fecha: 2026-07-28

## Qué quedó implementado

### Recepción rápida con respaldo

- Webhook de Meta como vía principal.
- Si Meta envía un POST auxiliar que no contiene el mensaje, Worktic ejecuta inmediatamente la sincronización de Conversations API.
- Sincronización adaptativa:
  - 10 segundos cuando existe actividad reciente.
  - 60 segundos cuando el canal está inactivo.
- Deduplicación real por `tenant_id + conexión + canal + message_id`.
- La misma entrada no puede generar dos respuestas aunque llegue por webhook y por API.

### Envíos duraderos

- Cola SQLite `messenger_outbox`.
- Intento inmediato.
- Reintentos automáticos: 10 s, 30 s, 2 min y 10 min.
- Recuperación de trabajos interrumpidos después de reiniciar Render.
- Estados visibles en la conversación: `queued`, `pending_retry`, `sent`, `ai_sent` o `failed`.
- Botón **Reintentar cola** en el diagnóstico.

### Seguridad

- Credenciales cifradas mediante `CHANNEL_ENCRYPTION_KEY`.
- Validación `debug_token` cuando existe App Secret.
- Verificación de firma `X-Hub-Signature-256` para los POST del webhook cuando el App Secret está configurado.
- El User Access Token de larga duración nunca se devuelve al navegador ni se escribe en logs.
- Page ID, App ID, permisos y vencimientos quedan monitorizados.

### Token estable

El modal de Messenger incluye **Generar y guardar token estable desde Worktic**.

El backend:

1. Recibe un User Access Token temporal.
2. Lo intercambia por un User Access Token de larga duración.
3. Obtiene el Page Access Token de la página seleccionada.
4. Valida página, app y permisos.
5. Guarda ambos tokens cifrados.
6. Monitoriza vencimiento y acceso a datos.
7. Puede regenerar el Page Access Token mientras el User Access Token guardado siga vigente.

Meta puede exigir una nueva autorización del usuario cuando venza el User Access Token o el acceso a datos. Worktic avisa con anticipación; no promete una renovación indefinida sin intervención del administrador.

## Variables de entorno recomendadas en Render

```env
META_GRAPH_VERSION=v25.0
META_APP_ID=2895087757509133
META_APP_SECRET=TU_APP_SECRET_REAL
CHANNEL_ENCRYPTION_KEY=UNA_CLAVE_ALEATORIA_DE_64_CARACTERES_O_MAS
MESSENGER_SYNC_ACTIVE_SECONDS=10
MESSENGER_SYNC_IDLE_SECONDS=60
MESSENGER_TOKEN_CHECK_HOURS=6
MESSENGER_OUTBOX_MAX_ATTEMPTS=5
```

No cambies `CHANNEL_ENCRYPTION_KEY` después de guardar credenciales. Si la cambias, los tokens cifrados existentes dejarán de poder leerse.

## Generar el token estable: paso a paso exacto

### 1. Generar el User Access Token temporal

En Meta Developers:

1. Abre **Herramientas → Explorador de la API Graph**.
2. Selecciona la app **Worktic AI**.
3. En **Usuario o página**, selecciona **Token del usuario**.
4. Agrega estos permisos:

```text
pages_show_list
pages_messaging
pages_read_engagement
pages_manage_metadata
```

5. Pulsa **Generate Access Token**.
6. Selecciona únicamente la página **Worktic AI — 1266783359846425**.
7. Acepta todos los permisos.
8. Copia el token que aparece en el panel derecho. Este es el **User Access Token temporal**.

### 2. Convertirlo dentro de Worktic

1. En Worktic abre **Canales**.
2. En **Messenger Ventas**, pulsa **Reconfigurar**.
3. Confirma:

```text
Page ID: 1266783359846425
Meta App ID: 2895087757509133
```

4. Coloca el **App Secret** o configura `META_APP_SECRET` en Render.
5. Abre **Generar y guardar token estable desde Worktic**.
6. Pega el User Access Token temporal.
7. Pulsa **Convertir y guardar token estable**.
8. Espera el mensaje `Token estable generado y guardado`.

No pegues en ese campo el Page Access Token. Debe ser el token de usuario recién generado en Graph API Explorer.

### 3. Confirmar en el diagnóstico

Abre **Probar conexión**. Deben quedar activos:

- Page Access Token válido.
- Permiso `pages_messaging`.
- Page ID coincide.
- Firma del webhook activa.
- Token estable/monitorizado.
- Webhook listo.
- Respaldo API activo.
- Cola de salida saludable.

El diagnóstico muestra:

- Fecha de vencimiento del Page token cuando Meta la informa.
- Fecha límite del User token de larga duración.
- Fecha de acceso a datos.
- Última validación.
- Último mensaje recibido.
- Última respuesta enviada.
- Mensajes pendientes o fallidos.

## Configuración de Meta que debe conservarse

Webhook actual de la conexión y el token `wtm_...` generado por Worktic.

Campos suscritos recomendados:

```text
messages
messaging_postbacks
message_reads
message_deliveries
messaging_referrals
standby
messaging_handovers
```

La app **Worktic AI** debe continuar como app de enrutamiento predeterminada de Messenger.

## Prueba final

1. Envía desde el evaluador un mensaje nuevo.
2. El POST del webhook debe activar la sincronización inmediata si no trae el contenido completo.
3. La conversación debe aparecer normalmente en menos de 10–15 segundos.
4. La IA debe crear una respuesta y colocarla en la cola.
5. La cola intenta enviarla de inmediato; ante una falla temporal la reintenta.
6. Verifica en el diagnóstico:
   - Mensaje real recibido.
   - Última respuesta enviada.
   - Cola sin fallidos.

## Migraciones automáticas

Al iniciar, Worktic crea:

- `messenger_outbox`.
- Índices para reintentos y consultas.
- Índice único de mensajes de proveedor.

La migración elimina únicamente duplicados exactos del mismo `message_id` dentro del mismo tenant, conexión y canal. No elimina conversaciones diferentes.

## Recuperación ante incidentes

- Token vencido: Worktic intenta regenerar el Page token usando el User token cifrado.
- User token vencido: el diagnóstico solicita una nueva autorización.
- Meta temporalmente no disponible: el envío queda en cola.
- Reinicio de Render: la cola pendiente se recupera automáticamente.
- Webhook parcial o retrasado: Conversations API recupera el mensaje.
- Doble entrega: el índice de idempotencia evita duplicados y respuestas dobles.
