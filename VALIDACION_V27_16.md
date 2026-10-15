# Validación WorkticAI V27.16 — Partner Certified Services & UX Polish

Base real: **WorkticAI V27.15.1 — Partner Workspace UX**.

## Validaciones realizadas en el paquete

- `gofmt` aplicado a `agents.go`, `db_compat.go`, `main.go`, `partners_v2715.go` y `partners_v2716.go`.
- `node --check static/app.js`: OK.
- `node --check static/agents-v22.js`: OK.
- `node --check static/partners-v2715.js`: OK.
- `node --check static/admin-full.js`: OK.
- `node --check static/i18n.js`: OK.
- `static/app.html`: 0 IDs duplicados.
- `static/index.html`: 0 IDs duplicados.
- `static/partners.html`: 0 IDs duplicados.
- Esquema `partner_service_assignments_v2716`: ejecución SQLite de validación OK.
- No quedan botones HTML/JS con acción visible `Eliminar`/`Olvidar` usando estilo secundario: las acciones destructivas quedan en estilo danger con texto blanco.
- Los archivos públicos aprobados permanecen idénticos a V27.15.1: `static/index.html`, `static/partners.html`, `static/marketing-v2713.css`, `static/marketing-v2713.js`.

## Validación obligatoria en el PC antes de push

El entorno de generación tiene Go 1.23.2 y el proyecto exige Go 1.26. El intento de `go test ./...` no puede descargar el toolchain por bloqueo de red. Ejecutar localmente con Go 1.26.x:

```powershell
Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
go test ./...
node --check .\static\app.js
node --check .\static\agents-v22.js
node --check .\static\partners-v2715.js
node --check .\static\admin-full.js
node --check .\static\i18n.js
git diff --check
git status
```

No hacer commit/push si `go test ./...` falla.

## Pruebas funcionales posteriores al deploy

1. **Agentes IA**: todos muestran `Eliminar`; si se elimina el principal, otro agente disponible pasa a principal y las conexiones se reasignan al reemplazo. El historial de auditoría se conserva.
2. **Botones destructivos**: Catálogo, Canales, Agentes, Automatizaciones, conocimiento/memoria, Admin y Partner muestran fondo rojo + texto blanco.
3. **Partner > Inicio**: Share, Sell, Activate Certified y Lead se explican como cuatro formas diferentes de generar ingresos.
4. **Partner > Activate Certified**: se explica la certificación profesional en implementación, CRM, canales, IA, automatización, Growth/marketing y Analytics, y qué beneficios desbloquea.
5. **Admin > Worktic Partners > Asignaciones**: asignar un proyecto únicamente a un especialista activo, validado, Activate Certified y con licencia elegible.
6. **Partner > Oportunidades**: visualizar empresa, alcance, entregables, fecha, prioridad y pago antes de aceptar; aceptar inicia el trabajo; rechazar cierra la propuesta; entregar pasa a revisión.
7. **Admin**: completar un proyecto de pago fijo y confirmar crédito `service_assignment` en el ledger Partner.
8. **Human Sales 80/20**: permanece como flujo separado de los proyectos de implementación/asistencia asignados por WorkticAI.

## Base de datos

V27.16 crea su tabla de forma aditiva al iniciar WorkticAI. **No ejecutar `/app/worktic-migrate`.**
