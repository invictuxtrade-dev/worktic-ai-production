# Worktic AI V27.35.25 — Login Premium V10 (integración)

## Alcance

- Vista de escritorio aprobada: escenario continuo, escena del teléfono, tarjeta de login y efectos sutiles.
- Vista móvil aprobada: **solo formulario**, sin banner ni textos comerciales extensos.
- Conserva **los archivos de logo originales ya presentes en producción**: `/assets/logo-mark.png` y `/assets/logo-full.png`. No incluye ni modifica logos.
- No modifica `static/login.js`, `static/i18n.js`, el backend Go, la base de datos ni otros módulos.
- Reutiliza los mismos IDs `loginTab`, `registerTab`, `loginForm`, `registerForm`, `authError`, `accountType`, `companyLabel` y los mismos `name=` de los campos, preservando el contrato esperado por el login real.
- Los controles de mostrar contraseña quedan a cargo del JS visual previo (`login-fidelity-v273524.js`).
- Recordarme guarda **solo el correo** en el navegador, nunca contraseñas.
- **Recuperación de contraseña:** la base V27.35.24 no proporcionaba un flujo propio verificable. El enlace informa que se debe contactar al administrador. NO se promete restablecimiento automático.

## Seguridad

- El instalador exige la base exacta V27.35.24 (normalizando LF/CRLF) y comprueba los SHA-256 de los archivos de este parche.
- Rechaza modificaciones previas en `login.html` o `VERSION.txt` y archivos destino nuevos existentes.
- Hace respaldo antes de escribir; intenta revertir si algo falla.
- No hace commit, push ni despliega Render.

## Advertencia de verificación

Se validó la apariencia en Chromium con recursos locales del paquete, las transiciones de pestañas, el registro visual y el dimensionamiento en 1366×768, 1536×864, 390×844 y 320×568. **La autenticación real depende del `login.js` y backend de producción, no incluidos aquí**: deben comprobarse ingreso, registro, alternancia de idioma y control de errores con una cuenta de prueba antes de publicar.
