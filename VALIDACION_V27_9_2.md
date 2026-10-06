# WorkticAI V27.9.2 — Copilot Functional Hotfix

## Fuente
Base: `WorkticAI V27.9.1 — Copilot UI Polish`.

## Error corregido
El frontend llamaba `diagnosticTextV277(c)` desde `loadHome()` y `updateContext()`, pero esa función no existe en la base V27.6/V27.9.1. La excepción ocurría al recibir el evento `start` del streaming, por lo que el chat mostraba `diagnosticTextV277 is not defined` y caía al manejo de error.

Se reemplazó por `copilotContextText(c)`, una función local y defensiva que usa exactamente las claves que devuelve `copilotContextV27`: `connected_channels`, `active_agents`, `active_workflows` y `unread_messages`.

## Funcionalidad preservada
- GET `/api/copilot/v27` para contexto.
- POST `/api/copilot/v27/stream` para streaming NDJSON.
- POST `/api/copilot/v27` como fallback.
- Historial reciente.
- Deep links/acciones contextuales.
- Herramientas y selección de módulo.
- Saludos locales cortos.

## UI
- Dashboard V27.6 sin cambios.
- Estrella del launcher reemplazada por SVG centrado.
- No se agregan selectores que modifiquen Dashboard/Resumen.

## Validaciones
```powershell
node --check .\static\v27-growth-command.js
node --check .\static\app.js
node --check .\static\social-v23.js
go test ./...
git diff --check
```
