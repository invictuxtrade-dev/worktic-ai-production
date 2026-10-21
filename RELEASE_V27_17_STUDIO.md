# WorkticAI V27.17 — Worktic Studio (integración sobre respaldo V27.16.1)

**Fuente exclusiva de la integración:** `worktic-ai-production.zip` proporcionado por el propietario, cuyo `VERSION.txt` dice `WorkticAI V27.16.1 — Partner Admin Loader Hotfix`, commit `09e37d4` en `main` y árbol Git limpio.

## Qué se incorporó

- **Image Studio**: lienzo editable por capas, textos, elementos, plantillas, controles de posición/tamaño/opacidad/color, deshacer/rehacer, exportación PNG a la Galería multimedia.
- **Video Studio**: composición secuencial de clips e imágenes, textos sobre pista, música, previsualización y exportación WebM en navegador (máximo 60 s y 50 MB).
- **Biblioteca compartida:** consulta y escritura en `social_media_assets`, con archivos persistentes en `DATA_DIR/social_uploads/<tenant_id>` y nueva carga de audio de 20 MB como complemento aislado. Social Hub conserva sus endpoints originales.
- **Persistencia por empresa:** proyectos editables en `studio_projects_v2711`, uso de generación IA en `studio_ai_usage_v2711`, creadas con `CREATE TABLE IF NOT EXISTS`.
- **IA:** solicitudes del servidor a OpenAI para imágenes (modelo configurable con `WORKTIC_STUDIO_IMAGE_MODEL`) y textos/guiones. Usa `OPENAI_API_KEY` existente. La generación de guion no crea automáticamente clips de video con IA.
- **Navegación:** opción Worktic Studio independiente y carga inicial desde la vista correspondiente.
- **Audio:** `/api/studio/upload` para MP3/WAV/OGG/M4A, sin modificar el cargador existente `/api/social/upload`.
- **PostgreSQL:** adaptador de inserciones con `RETURNING id` para las dos tablas nuevas. Sin migración SQLite → PostgreSQL.

## Archivos modificados respecto del respaldo V27.16.1

- `VERSION.txt`
- `main.go`: arranque aditivo y tres rutas del Studio
- `db_compat.go`: dos entradas de `postgresSerialTables`
- `static/app.html`: opción del menú, vista Studio, CSS y JS
- `static/app.js`: inicialización al abrir Studio
- `static/i18n.js`: títulos del nuevo módulo

## Nuevos archivos

- `studio_v2711.go` (código aditivo original, ampliado con audio)
- `static/worktic-studio-v2711.js`
- `static/worktic-studio-v2711.css`
- Este documento `RELEASE_V27_17_STUDIO.md`

No se editaron `partners_v2714.go`, `partners_v2715.go`, `partners_v2716.go`, `static/partners-v2715.js`, ni módulos de WhatsApp, CRM, Copilot, Social Hub o los endpoints originales de la Galería.

## VALIDACIONES REALIZADAS EN CONTENEDOR

- `gofmt` sobre Go modificado: correcto.
- `node --check` de los JS modificados: correcto.
- `git diff --check`: correcto.
- HTML: etiquetas de sección balanceadas e IDs estáticos únicos.
- Navegador Chromium con APIs simuladas: cargar Image Studio, añadir texto, guardar un proyecto, exportar PNG, cambiar a Video Studio, importar audio mediante ruta independiente, generar texto IA simulado y exportar WebM corto: correcto.
- Diseño móvil de módulo aislado a 390 px: sin desbordamiento horizontal.

**Pendientes obligatorios antes de publicar:** `go test ./...` y `go build ./...` con Go 1.26 (el entorno de generación solo tiene Go 1.23 y no puede descargar 1.26), arranque real del backend, pruebas autenticadas multi-tenant y de autorizaciones sobre PostgreSQL, OPENAI_API_KEY real, archivos de disco persistente en Render, carga de medios y regresiones de todas las vistas. Estas validaciones pendientes impiden afirmar certificación integral al 100%.

## Límites expresos

Este **no es un clon completo de CapCut**. No incluye exportación MP4 avanzada, subtitulación de voz automática, tracking, máscaras inteligentes, edición de audio profesional ni generación nativa de clips de video con IA. El formato de exportación video es WebM por MediaRecorder en navegadores compatibles; puede haber diferencias de audio y codecs entre navegadores y redes sociales. Audio y video deben validarse con archivos reales antes de publicar.

## INSTALACIÓN SIN PERDER MÓDULOS

1. Respalda el repositorio local, el despliegue Render, PostgreSQL y `/var/data`.
2. Verifica que el repositorio actual que estás por actualizar corresponde a **V27.16.1** y su estado Git esté limpio. Si tiene cambios posteriores, **no sobrescribas**: primero integra estos cambios por diferencias.
3. Descomprime el ZIP en una carpeta nueva. El ZIP entregado está **sin `.git`** ni secretos de producción.
4. Compara la carpeta nueva contra el repo actual (Windows PowerShell): `git diff --no-index -- <carpeta_actual> <carpeta_nueva>` (ignorando `.git`) o usa un comparador visual. Verifica que la lista de modificaciones sea exactamente la descrita arriba.
5. Incorpora los archivos en el repositorio real, sin eliminar sus archivos adicionales. Comprueba `git status --short` y `git diff --check`.
6. Con Go 1.26 disponible, ejecuta `go test ./...` y `go build ./...`. Con Node instalado, valida `node --check static/app.js` y `node --check static/worktic-studio-v2711.js`.
7. Prueba primero en un servicio Render *staging* aislado, con BD y datos copiados/anonimizados. Comprueba que las tablas nuevas aparezcan sin alterar las existentes.
8. Verifica inicio de sesión, Studio, Galería, Social Hub, Copilot, CRM, WhatsApp, Partners, administración, mensualidad/anualidad, `healthz` y `readyz`.
9. Solo después, realiza `git add` de los archivos revisados, `git commit` y `git push` a la rama de despliegue real. Despliega desde Render y vigila logs/rollback.

**Nunca ejecutes la migración SQLite a PostgreSQL de nuevo.**
