# WorkticAI V27.2 → V27.3 Premium Experience

## Cambios principales

- Worktic Copilot rediseñado con prioridad a la conversación.
- Área de escritura amplia (96–156 px), legible y responsive.
- Panel organizado en Asistente, Guías, Automatizaciones y Análisis.
- Diagnóstico rápido compacto y carga en segundo plano.
- Social Hub: estado social inicializado antes del bootstrap para eliminar el error `Cannot access 'socialOverview' before initialization`.
- Perfil CRM Premium: identidad, métricas, licencia, límites, seguridad, preferencias, canales, equipo y actividad reciente.
- Edición funcional de nombre, empresa, idioma y zona horaria.
- Preferencias persistentes en PostgreSQL mediante tabla creada automáticamente.
- Cache busting V27.3 para evitar cargar JavaScript/CSS anterior.

## Base de datos

No ejecutar `/app/worktic-migrate`.

V27.3 crea automáticamente `user_profile_preferences_v273` al abrir/guardar el perfil. La tabla es aditiva y no modifica datos existentes.

## Validación local recomendada

```powershell
Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
go test ./...
git diff --check
```

Después:

```powershell
git add -A
git commit -m "WorkticAI V27.3 Premium Experience"
git push origin main
```
