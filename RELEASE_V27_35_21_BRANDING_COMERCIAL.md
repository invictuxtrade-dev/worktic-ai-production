# WorkticAI V27.35.21 · Branding Comercial Premium

**Base exigida:** WorkticAI V27.35.20 (Identidad Premium y Paginación).

## Cambios

- Sitio público: imagen del logo de marca de 34 px a 56 px en escritorio y proporción adaptable en móvil. Barra superior de 84 px, menú y enlaces existentes intactos.
- Menú móvil: marca ampliada a 50 px sin sustituir su navegación.
- Inicio de sesión / crear cuenta: logo de marca centrado sobre el nombre `WorkticAI` y el titular comercial **«Automatiza tu marketing y convierte prospectos en ventas»**.
- Subtítulo comercial: «Centraliza tus canales, automatiza seguimientos y convierte más oportunidades desde una sola plataforma».
- Traducciones EN/ES para titular y subtítulo, siguiendo el sistema `i18n.js` existente.
- CSS aislado `static/branding-commercial-v273521.css` cargado después de los estilos de marketing previos para resolver precedencia sin reescribirlos.

## Lo que NO cambia

- No se sustituyen archivos del logo V27.35.20 (se reutiliza el icono aprobado).
- No se modifican las funcionalidades de registro/login, cuentas, permisos, pagos, agentes, Agenda IA, CRM ni catálogos.
- Se mantienen los paginadores de V27.35.20; no se agregan consultas SQL ni nuevas tablas.
- No se modifica código de Go ni migraciones de PostgreSQL.

## Instalación y validación

1. Respaldar Git/repo y extraer ZIP.
2. Ejecutar `INSTALAR_BRANDING_COMERCIAL_V273521.ps1` con `-Repo` y `-PatchFolder`. Verifica SHA-256 de los cuatro archivos base para evitar sobrescribir trabajo distinto de V27.35.20.
3. El instalador valida `node --check static/i18n.js`, `node --check static/pagination-premium-v273520.js`, `go test -count=1 ./...`, `go build ./...` y `git diff --check`; restaura archivos si falla.
4. Revisar visualmente en 1440, 768 y 390 px las rutas `/`, `/login.html`, pestañas Ingresar/Crear cuenta, idioma EN/ES, navegación móvil y otros módulos.
5. Publicar en GitHub y Render **solo tras validar**, si no hay cambios de origen ni errores en producción.

**Advertencia:** Esta entrega no equivale a una auditoría de todos los paginadores de backend; esa tarea requiere revisar los módulos y APIs por separado.
