# WorkticAI — Actualización V27.4 → V27.5

## Objetivo

V27.5 es una actualización de estabilidad y experiencia del Copilot. Corrige los errores JavaScript que estaban afectando Agentes IA, Grupos & Comunidades e Inbox, y convierte el Copilot a respuesta progresiva en tiempo real.

## Correcciones de estabilidad

### App principal

El HTML actual ya no contiene `#metaConnectInfo`, pero `app.js` seguía intentando asignar `onclick` directamente a ese elemento. Esa excepción detenía la ejecución del archivo antes de inicializar estados posteriores y generaba errores secundarios como:

- `Cannot access 'aiAgents' before initialization`
- `Cannot access 'allLandings' before initialization`
- `Cannot access 'groupLimits' before initialization`
- errores posteriores en `agents-v22.js`

V27.5:

- hace opcional ese binding;
- inicializa los estados compartidos críticos antes del bootstrap;
- mantiene compatibilidad con los módulos V20–V27.

### Inbox V21

Se corrige un problema de Automatic Semicolon Insertion (ASI) antes de los filtros de conversación que podía producir:

`Cannot read properties of undefined (reading 'forEach')`

Además, las respuestas de equipo, conversaciones, mensajes y notas se normalizan defensivamente a arrays.

## Worktic Copilot Streaming

Se añade:

`POST /api/copilot/v27/stream`

El backend consume OpenAI Responses API en modo streaming y reenvía los deltas al navegador mediante NDJSON.

En frontend:

- la respuesta comienza a aparecer apenas llegan los primeros fragmentos;
- el texto crece dentro de la misma burbuja;
- el scroll acompaña gradualmente mientras el usuario está cerca del final;
- si el usuario desplaza manualmente hacia arriba, no se le devuelve forzosamente al final;
- existe fallback al endpoint tradicional `/api/copilot/v27` si el streaming no puede iniciarse;
- se mantiene timeout y protección de doble envío.

## Base de datos

No hay tablas ni migraciones nuevas.

**No ejecutar** `/app/worktic-migrate`.

## Validación local antes del push

```powershell
Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
go test ./...
git diff --check
git status
```
