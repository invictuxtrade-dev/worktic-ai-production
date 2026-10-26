# WorkticAI V27.34.2 — Landing FAQ & Partners Navbar

Base de código: **V27.34.1**, no la versión V27.34 sin el parche Premium Web.

## Alcance
- Eliminación completa de la sección promocional `#worktic-credits` de la landing (sin cambiar la billetera, recargas, base de datos ni checkout).
- 31 preguntas frecuentes con 6 categorías y buscador; HTML presente para indexación, alternancia ES/EN y accesibilidad mediante `<details>`.
- Ajuste exclusivo de estilos del encabezado de `partners.html` con mismos espaciados, anchos, color y CTA de la web principal; enlaces y contenido Partner sin cambios.
- Se conserva el bloque de Worktic Studio y sus controles visuales de la V27.34.1.

## Archivos del parche
- `VERSION.txt`
- `RELEASE_MANIFEST_V2734.json`
- `RELEASE_V27_34_2_WEB_FAQ_PARTNERS.md`
- `static/index.html`
- `static/partners.html`
- `static/marketing-v2736.css` (nuevo)
- `static/marketing-faq-v2736.js` (nuevo)
- `static/partners-v2736.css` (nuevo)

## Validación previa a producción
1. Verificar que el repositorio está en V27.34.1 o que sus archivos se encuentran ya instalados.
2. Aplicar estos ocho archivos sin borrar ni reemplazar otros.
3. Ejecutar `node --check static/marketing-faq-v2736.js`, `python tools/v2734/verify_manifest.py`, `go test ./...` y `go build ./...`.
4. Revisar la landing en 1366, 1024 y 390 px, FAQ (buscador, categorías, respuestas ES/EN), menú de Partners en escritorio y móvil.
5. Respaldo de producción y despliegue seguro con `git push` sin force.

No cambia permisos, precios, pagos, reglas, módulos de Partners ni variables de entorno. No habilita créditos comerciales.
