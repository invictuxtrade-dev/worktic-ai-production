# Worktic AI V13.1 — Team WhatsApp Premium

Versión multitenant con canales aislados, sistema multiagente, facturación por planes, invitaciones de equipo por WhatsApp, CRUD profesional y diseño premium responsive.

Consulta `V13_1_TEAM_WHATSAPP_PREMIUM.md` para conocer las mejoras y el flujo de invitación.

# Worktic AI V13 Multi-Tenant Channels

Plataforma omnicanal con CRM, sistema multiagente, campañas, landing pages, grupos, facturación y conexiones físicamente separadas por tenant.

## Arranque rápido

1. Copia `.env.example` como `.env`.
2. Configura OpenAI, direcciones USDT y secretos.
3. Ejecuta `INSTALAR_DEPENDENCIAS_WINDOWS.bat`.
4. Ejecuta `INICIAR_WINDOWS.bat`.
5. Abre `http://localhost:8080`.

Acceso inicial local:

- Correo: `admin@worktic.local`
- Contraseña: `Admin123!`

Cámbiala antes de exponer el servidor.

## Principales módulos

- Registro personal y empresarial.
- Planes Free, Personal, Negocio y Empresa.
- Pagos USDT BEP20/TRC20.
- CRM, pipeline, catálogo y agenda.
- Agentes IA múltiples con enrutamiento y métricas.
- Canales por tenant y conexión.
- WhatsApp QR con sesión independiente por conexión.
- Telegram y Messenger con credenciales separadas.
- Growth, campañas y atribución.
- Landing pages premium públicas.
- Grupos y comunidades.
- Administración y auditoría.
- Diseño móvil.

## Documentación

Lee `V13_MULTI_TENANT_CHANNELS.md` y `PRODUCTION_READINESS.md` antes del despliegue.

## Estado

V13 es una base de migración y pruebas preproducción. Debe compilarse y probarse en el servidor de destino, validar los permisos reales de Meta, configurar HTTPS, backups, correo, claves seguras y observabilidad antes de abrir el registro público.

## Corrección V13.0.3

Se separó la experiencia de cuenta personal, cuenta empresarial y superadministración. Consulta `V13_0_3_ROLES_PROFILE_BILLING.md`.

## Inicio en Windows V13.0.4

Ejecuta primero `DESBLOQUEAR_WINDOWS.bat`, después `INSTALAR_DEPENDENCIAS_WINDOWS.bat` y finalmente `INICIAR_WINDOWS.bat`. El iniciador usa una ruta absoluta para el ejecutable y espera antes de abrir el navegador.
