# Messenger Real Fix

- Reconfigurar usa `action: details` y nunca consulta Meta Graph API.
- Probar conexión nunca devuelve HTTP 502 por permisos de Meta; conserva y muestra webhook y verify token.
- Se eliminan falsas alarmas causadas por `/subscribed_apps` o por consultas bloqueadas por revisión.
- Se añadió cache-busting a `app.js` para evitar que el navegador mantenga la versión anterior.
