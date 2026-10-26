# WorkticAI V27.34.1 — Website Premium Polish

Base verificada: `WorkticAI_V27.34_COMPLETO_CORREGIDO_01.zip` (web V27.34 + hotfix de compilación anterior). Actualización visual **incremental**, no una nueva configuración comercial.

## Cambios específicos

1. `static/index.html`: header con el ancho necesario para marca/nav y nueva sección Worktic Studio + tarjeta de Creative Credits. Conserva landing, secciones, SEO, planes, checkout y enlaces anteriores.
2. `static/marketing-v2735.css`: diseño premium responsive, sombras, fondos, mockup con animación suave y tarjetas de beneficios. Breakpoint de menú móvil también entre 860 y 1220 px para evitar traslapes. Sin servicios externos nuevos.
3. `static/marketing-v2735.js`: selector interactivo Video/Image en la **vista conceptual** y traducciones ES/EN de nuevos textos, sin llamadas a backend.
4. `static/commercial-offer-v2734.js`: elimina el aviso suelto y muestra estado de activación de forma integrada en las tarjetas; las ofertas y precios siguen consultando `/api/public/offer/v2734`. El checkout continúa informando que los créditos no están incluidos cuando la activación comercial está deshabilitada.
5. `VERSION.txt`, `RELEASE_MANIFEST_V2734.json` y `tools/v2734/verify_manifest.py` actualizados para comprobar la integridad de V27.34.1 con SHA256.

## Restricciones de la actualización

- No se modifica el motor de planes, precios ni suscripciones.
- No se activan generaciones IA ni recargas, y no se promete IA ilimitada.
- El editor ilustrado en la landing es una **vista conceptual**, no un editor independiente. Las herramientas reales están dentro de WorkticAI.
- No se modifica `main.go`, otros archivos Go, PostgreSQL, Redis, Worktic Partners, CRM, canales, pagos, ni Worktic Studio interno.
- No se ejecuta despliegue ni se cambian variables de Render.

## Instalación sobre la carpeta de VS Code V27.34 actual

1. Descargar `WorkticAI_V27.34.1_WEB_PREMIUM_PATCH.zip` en Descargas.
2. Abrir PowerShell en `C:\Users\DELL\Desktop\Worktic_V27.34_Deploy_20261026-103800`.
3. Descomprimir el parche a una carpeta temporal y copiar las rutas respetando la jerarquía.
4. Ejecutar `python tools/v2734/verify_manifest.py`, `node --check static/commercial-offer-v2734.js`, `node --check static/marketing-v2735.js`, `git diff --check`, `go test ./...`, `go build ./...`.
5. Revisar los archivos `git status --short` y publicar con commit/fast-forward normal, solo después de superar las pruebas y disponer de un respaldo recuperable.

## Verificación visual

- PC 1366 px: logo y primer ítem de menú separados; Studio ocupa dos columnas y el editor conceptual está visible; créditos en dos columnas.
- 1280 px: menú pasa a hamburguesa cuando falta espacio.
- Móvil 390 px: diseño en una columna, editor sin overflow horizontal, menú lateral funcional.
- ES/EN: textos del nuevo bloque traducidos.
- Simulación con la API de oferta marcada como activa/inactiva: estado coherente de créditos, sin precio inventado si está inactiva.

La validación de navegador fue aislada, con datos simulados para la API comercial; no son pruebas sobre Render ni los pagos reales.
