# WorkticAI V27.35.1 — Chat Web + Llamadas IA (STAGING)

Base: `WorkticAI V27.35.0 STAGING` subida anteriormente al repositorio `invictuxtrade-dev/worktic-ai-production`. Esta actualización conserva el código de voz V27.35 y añade las rutas, el widget y la interfaz de Chat Web de V27.34.10 mediante una fusión de cambios. No reinstalar V27.34.10 sobre esta base.

## Integración
- `Canales`: mantiene la tarjeta de Llamadas IA e incorpora acceso a Chat Web.
- `Chat Web`: permite crear, configurar y desactivar canales de chat web, dominio autorizado HTTPS, agente asignado y copiar widget.
- `Bandeja`: filtro Chat Web y respuestas humanas por la API existente.
- `CRM`: conversaciones vinculadas con contactos según rutas y permisos del backend.
- `Llamadas IA`: conserva las rutas, JS, CSS y esquema de V27.35.
- Flags iniciales en Render: `WORKTIC_WEBCHAT_ENABLED=false`, `WORKTIC_WEBCHAT_AUTO_AI=false`; flags de voz mantener el estado anterior.

## Instalación
El `INSTALAR_CHATWEB_SOBRE_V2735.ps1` se ejecuta desde PowerShell, tras descomprimir este parche. Solo copia archivos después de comprobar: (1) repositorio limpio; (2) HEAD == origin/main tras git fetch; (3) SHA-256 de archivos modificados coincide con el ZIP V27.35.0 suministrado. Si algo difiere, no sobrescribe. No use `git reset --hard` ni `git push --force`.

## Pruebas locales obligatorias
```
node --check static/app.js
node --check static/inbox-v21.js
node --check static/i18n.js
node --check static/webchat-v2736.js
node --check static/worktic-webchat-v1.js
go test ./...
go build ./...
git diff --check
```
Si pasa, confirmar en navegador que login, Studio, créditos y Llamadas IA siguen presentes; luego revisar Chat Web desactivado antes del despliegue.

## Estado
Revisión estática de Go con `gofmt`, JavaScript con Node, HTML con parser, sin IDs repetidos. No se ejecutó compilación Go integral: este entorno tiene Go 1.23 y proyecto requiere Go 1.26. Tampoco hay pruebas contra Render ni de telefonía/Chat Web extremo a extremo. No activar atención IA hasta validar consumo, límites y permisos.
