# WorkticAI V27.35.20 · Identidad y paginación

**Base requerida:** V27.35.19 corregida y validada en Windows.

## Alcance

- Nuevo logo proporcionado por el cliente instalado en `/static/assets/` como marca, lockup y favicons para las rutas que usan las referencias compartidas.
- Ajustes visuales de identidad en acceso, backoffice, portada y Partners, sin reemplazar la distribución existente ni modificar textos de precios.
- Nombre visible **Agenda IA** en menú y módulo; se actualiza también la traducción.
- Paginador dinámico de 10, 20, 50 o 100 registros en las tablas y cuadrículas de gestión cargadas en el navegador, incluidas oportunidades en vista Lista.
- Se conservan los paginadores backend de Administración Worktic y las vistas de flujo continuo (conversaciones en vivo, calendario, Kanban, timeline) para evitar ocultar entradas importantes.

## Límites de esta versión

La paginación de vistas cargadas no sustituye la paginación SQL real de colecciones masivas. Los módulos con carga parcial del servidor muestran «X de X cargados» y requieren una actualización backend específica para consultar todas las páginas. La validación visual en producción y las pruebas de carga de volumen quedan pendientes.

**Sin cambios** en tablas, permisos, agentes, stock, pagos o APIs de backend.

## Validación

1. `node --check static/pagination-premium-v273520.js`.
2. Prueba de humo: Catálogo y Contactos con 12 o más entradas, elegir 10/20 y comprobar controles.
3. Verificar `/`, `/login.html`, `/app.html`, `/partners.html` y favicon.
4. Abrir Agenda IA y comprobar calendario y nueva cita.
5. Comprobar que conversaciones, Kanban y administración conservan su carga y paginación anterior.
6. Validar `go test ./...` y `go build ./...` en Windows con Go 1.26 antes de publicar.
