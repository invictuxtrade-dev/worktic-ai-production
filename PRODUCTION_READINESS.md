# Preparación para producción — V13

## Incorporado

- Tenant persistente por cuenta.
- Usuarios relacionados con tenant.
- CRUD de conexiones filtrado por tenant.
- Límites total y por tipo según plan.
- WhatsApp QR con cliente y base de sesión independiente por conexión.
- Reconexión progresiva después del arranque.
- Identificación de mensajes con tenant y conexión.
- Credenciales nuevas selladas con AES-GCM.
- Auditoría básica de conexiones.
- Panel de salud de canales.
- Web y facturación actualizadas con características detalladas.

## Requiere infraestructura o validación externa

- App Review y permisos oficiales de Meta.
- Webhooks HTTPS de Messenger/Instagram y Telegram.
- Sustituir la clave derivada local por KMS/Vault/Secret Manager.
- PostgreSQL efectivo como base principal; la demo todavía conserva SQLite.
- Redis efectivo para colas, rate limits y locks distribuidos.
- S3/MinIO efectivo para archivos y URLs firmadas.
- Verificación automática blockchain de pagos.
- 2FA, correo transaccional y recuperación de contraseña.
- Backups probados mediante restauración.
- Pruebas automatizadas de aislamiento, carga y seguridad.

No se debe describir esta versión como certificada para producción hasta completar y documentar esas pruebas en la infraestructura definitiva.
