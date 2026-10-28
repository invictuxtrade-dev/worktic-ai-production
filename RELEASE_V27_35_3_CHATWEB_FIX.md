# WorkticAI V27.35.3 — Corrección «Continuar a configuración» Chat Web

## Error corregido
En V27.35.2 el selector cerraba el diálogo antes de consultar `/api/webchat/channels`. Si `WORKTIC_WEBCHAT_ENABLED=false` o faltaban agentes activos, el aviso se escribía dentro de una vista oculta y parecía que el botón no hacía nada.

## Correcciones
- El selector **permanece abierto** cuando el servidor no habilita Chat Web o no hay agentes IA activos.
- Muestra en el mismo formulario una explicación visible y un aviso al usuario.
- Solo abre el formulario de Chat Web tras verificar disponibilidad y agentes activos.
- Cambia el estado del botón durante la consulta para evitar dobles clics.
- Mantiene el acceso unificado sin banners y conserva los formularios específicos para Web y Voz.
- También corrige que la selección de Llamadas IA no cierre el formulario al fallar la comprobación de disponibilidad.
- Fuerza actualización de caché de los dos JavaScript afectados mediante `?v=27353`.

## Importante
**Para crear un Chat Web hay que poner `WORKTIC_WEBCHAT_ENABLED=true` en Render.** Mantener `WORKTIC_WEBCHAT_AUTO_AI=false` para pruebas manuales. Debes tener al menos un agente IA activo.

## Cambios
`static/app.html`, `static/channels-unified-v27352.js`, `static/webchat-v2736.js`, `VERSION.txt`.
No se modifican archivos Go, base de datos, cuotas de créditos ni otros canales.

## Pruebas necesarias
1. `node --check static/channels-unified-v27352.js` y `node --check static/webchat-v2736.js`.
2. `go test ./...`, `go build ./...`, `git diff --check` en la máquina local.
3. Con variable `WORKTIC_WEBCHAT_ENABLED=false`, el selector muestra aviso visible, sin cerrar el diálogo.
4. Con variable en `true` y agente activo, abre configuración Chat Web (dominio HTTPS, nombre, agente).
5. Guardar y obtener código de instalación, comprobar que aparece la conexión.
6. Confirmar WhatsApp, Telegram, Messenger, Llamadas IA y la Bandeja sin regresiones.

La compilación y pruebas E2E de Render deben confirmarse antes de comercializar el canal.
