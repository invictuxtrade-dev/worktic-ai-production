# WorkticAI V26 — Production / Enterprise

Rama acumulativa final construida sobre V15→V25. V26 prepara WorkticAI para el corte de producción con PostgreSQL, Render Key Value/Redis, endurecimiento de autenticación/sesiones, rate limiting, seguridad HTTP, observabilidad, backup y migración controlada desde el SQLite actualmente desplegado.

## Documentos principales

Lee en este orden:

1. `PRODUCTION_ENTERPRISE_V26.md` — qué cambia técnicamente en V26.
2. `VARIABLES_FINALES_V26.md` — inventario final de variables, callbacks y webhooks.
3. `DESPLIEGUE_FINAL_V26_RENDER.md` — procedimiento exacto de actualización en Render.
4. `ACTUALIZACION_FINAL_ACUMULATIVA.md` — historial V15→V26.

## Desarrollo local

```text
1. Copia .env.example a .env
2. Configura las credenciales que vayas a probar
3. Ejecuta el proyecto con Go 1.24+ (Docker usa Go 1.25)
4. Abre http://localhost:8080
```

En desarrollo puede mantenerse SQLite. Producción V26 exige PostgreSQL salvo una excepción explícita de emergencia.

## Producción

La instalación actual **no debe sustituirse directamente sin migración**. Sigue `DESPLIEGUE_FINAL_V26_RENDER.md`.

V26 no contiene credenciales administrativas hardcodeadas. En una instalación completamente vacía se utilizan temporalmente `BOOTSTRAP_ADMIN_EMAIL` y `BOOTSTRAP_ADMIN_PASSWORD`; en la actualización real desde la instalación existente esos valores no son necesarios porque los usuarios se migran.

## Binarios Docker

```text
/app/worktic-ai
/app/worktic-migrate
```

## Utilidades de corte

```text
/app/scripts/pre_cutover_backup.sh
/app/scripts/post_cutover_check.sh
```

## Datos

- Base SaaS principal: PostgreSQL.
- Rate limiting/coordinación: Render Key Value/Redis.
- Persistent Disk: `/var/data`.
- WhatsApp QR legado: conserva SQLite/store local durante V26.
- Uploads y sesiones QR: no borrar durante el corte.

## Módulos acumulados

WorkticAI V26 conserva e integra CRM, Inbox omnicanal, WhatsApp Business, Social Hub, Autopilot/Governance, Growth/Lead Ads, Automation Engine, Agentes IA 2.0, Agenda, Catálogo, Analytics/Attribution y Ads Center.
