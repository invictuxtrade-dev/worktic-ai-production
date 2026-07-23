# Worktic AI V13 — Multi-Tenant Channels

## Objetivo

Esta versión introduce una capa de conexiones por tenant y por canal. Cada conexión cuenta con un identificador propio, agente asignado, estado, auditoría y almacenamiento independiente de sesión para WhatsApp QR.

## Modelo de aislamiento

- `tenants`: espacio personal o empresarial.
- `tenant_users`: relación entre usuarios y tenant.
- `channel_connections`: una fila por número, bot o página conectada.
- Toda conexión se consulta por `tenant_id + id`.
- Los mensajes nuevos incluyen `tenant_id` y `channel_connection_id`.
- Las sesiones de WhatsApp se almacenan en:
  `data/wa_sessions/tenant_<tenant_id>/channel_<connection_id>.db`

## Estados

`draft`, `waiting_qr`, `connecting`, `connected`, `reconnecting`, `paused`, `disconnected`, `error`, `revoked`.

## Límites comerciales

| Plan | Total | WhatsApp | Telegram | Messenger | Agentes |
|---|---:|---:|---:|---:|---:|
| Free | 1 | 1 | 1 | 0 | 1 |
| Personal | 2 | 1 | 2 | 1 | 2 |
| Negocio | 5 | 3 | 5 | 3 | 5 |
| Empresa | 15 | 10 | 15 | 10 | 15 |

El máximo por tipo no reemplaza el máximo total. Por ejemplo, Personal solo puede tener dos conexiones activas en total.

## Migración

Al iniciar, la aplicación crea tenants a partir de las cuentas existentes y asigna los usuarios de una misma empresa al mismo espacio. Los datos heredados sin tenant se asignan una sola vez al primer espacio para revisión; no se duplican entre clientes.

## Seguridad

Los tokens nuevos se sellan con AES-GCM antes de almacenarse. En producción, `APP_NAME` no debe utilizarse como secreto definitivo: configura una clave dedicada mediante un gestor de secretos y rota las credenciales existentes. Los tokens no se devuelven a la interfaz.

## Meta y Telegram

La separación de credenciales y conexiones está implementada. La recepción y publicación real dependen de configurar webhooks oficiales, permisos de Meta, dominios HTTPS y bots válidos. No se simula una publicación exitosa cuando el proveedor no la confirmó.

## Pruebas obligatorias antes de producción

1. Crear dos tenants independientes.
2. Conectar dos números de WhatsApp y verificar que cada uno conserva su archivo de sesión.
3. Intentar consultar una conexión ajena cambiando el ID en la URL; debe responder 404.
4. Reiniciar el servidor y comprobar reconexión progresiva.
5. Verificar que el mensaje entrante conserva tenant y conexión.
6. Probar límites de todos los planes.
7. Validar webhooks de Telegram y Meta en HTTPS.
8. Ejecutar auditoría, pruebas de carga y restauración de backups.
