# WorkticAI V27.5 → V27.6 Premium Interface

V27.6 parte **exclusivamente** de la V27.5 Stability + Streaming Copilot suministrada por el usuario.

## Alcance

Esta actualización es de interfaz. No cambia APIs, PostgreSQL, Redis, Social Hub ni la lógica estable de V27.5.

### Dashboard
- Hero premium Growth Command Center.
- 4 KPIs principales con sparklines.
- Captación por canal en barras usando fuentes reales de leads.
- Conversaciones atendidas/pendientes usando conversaciones + no leídas reales.
- Actividad reciente con búsqueda y paginación existente.
- Insights de IA.
- El markup premium ya existe en `app.html` desde el primer render; los antiguos cards del Dashboard quedan ocultos como holders de compatibilidad para `app.js`. Así no aparecen y desaparecen durante el arranque.

### Barra superior global
- Visible en escritorio en toda la aplicación.
- Búsqueda de módulos con `Ctrl+K`.
- Enter abre el primer resultado.
- Acceso rápido a Inbox, Guías y Mi perfil.
- Chip de usuario sincronizado con la sesión.

### Worktic Copilot
- Mantiene exactamente el streaming y fallback de V27.5.
- Nuevo diseño inspirado en la referencia.
- Tabs Asistente / Guías / Automatizaciones / Análisis.
- Acciones rápidas.
- Diagnóstico compacto.
- Conversación con área propia.
- Composer fijo de 96 px, sin sacrificar legibilidad.

## Base de datos
No ejecutar `/app/worktic-migrate`. No hay tablas nuevas.

## Prueba recomendada
```powershell
go test ./...
git diff --check
```
Después comprobar Dashboard, Ctrl+K, Copilot, Social Hub, Mi perfil e Inbox.
