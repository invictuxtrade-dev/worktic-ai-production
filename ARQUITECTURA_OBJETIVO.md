# Arquitectura objetivo SaaS

- API Go modular por dominios: identidad, tenants, canales, conversaciones, CRM, agentes, agenda, catálogo, automatizaciones, billing y administración.
- PostgreSQL con `tenant_id` obligatorio y políticas de acceso por fila.
- Redis para sesiones, rate limits, colas y coordinación.
- Workers independientes para canales, IA, seguimientos, pagos y notificaciones.
- S3/MinIO para medios y fuentes de conocimiento.
- Credenciales cifradas mediante envelope encryption.
- Una instancia lógica de canal por tenant y conexión.
- Auditoría inmutable de operaciones administrativas y financieras.
