# WorkticAI V27.2 — Corrección UX del Copilot y aislamiento de WhatsApp Marketing

Actualización incremental sobre V27.1. No agrega tablas, no requiere migración de PostgreSQL y no añade variables de entorno.

## Correcciones

1. **WhatsApp Marketing solo aparece en su módulo**
   - Se corrige el conflicto de CSS entre `.view` y `.wa-mkt-v27`.
   - La vista queda oculta en Dashboard, CRM, Agenda, Social Hub y cualquier otro módulo.
   - Solo se muestra al seleccionar `Marketing por WhatsApp` en el menú.

2. **Worktic Copilot rediseñado**
   - El panel ya no desborda horizontalmente.
   - Controles apilados y responsivos.
   - Mensajes y botones hacen wrap correctamente.
   - El panel adapta alto y ancho al viewport.
   - Mejor distribución entre cabecera, selector, conversación y caja de escritura.

3. **Soluciones inmediatas**
   - El catálogo principal se carga localmente al abrir el Copilot.
   - Ya no aparece `Cargando soluciones…` mientras se consulta el contexto del tenant.
   - El diagnóstico de configuración se actualiza en segundo plano.
   - Incluye buscador y categorías: Inicio/soporte, Ventas/atención, Canales/automatización, Marketing/crecimiento y Cuenta/administración.

4. **Cobertura más amplia**
   - Configuración inicial
   - Diagnóstico general
   - Errores y guías
   - Inbox
   - Contactos y oportunidades
   - Catálogo y Agenda
   - Agentes IA
   - Canales
   - WhatsApp Business y WhatsApp Marketing
   - Automatizaciones
   - Social Hub
   - Growth
   - Landing Pages
   - Grupos y comunidades
   - Ads Center
   - Analytics
   - Equipo, perfil, membresía y administración

## Despliegue

1. Copiar V27.2 sobre el repo conservando `.git`.
2. Ejecutar `go test ./...` con Go 1.26.x.
3. Ejecutar `git diff --check`.
4. Commit y push.
5. Desplegar el último commit en Render.

No ejecutar `/app/worktic-migrate`.
