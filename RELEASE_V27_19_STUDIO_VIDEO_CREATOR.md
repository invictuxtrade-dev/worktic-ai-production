# WorkticAI V27.19 — Worktic Studio Video Creator

**Base única:** WorkticAI V27.18 Studio Pro (la que procede del despliegue V27.17). No se incorporan archivos de versiones históricas.

## Cambios

1. Video Studio ocupa más espacio de previsualización en pantalla, reduce la altura del timeline, organiza las herramientas en una barra horizontal desplazable y permite ocultar/mostrar el timeline. Barra de zoom de previsualización y modo enfoque ya existente.
2. Pestaña **Video IA**: creación de un video animado editable a partir de texto y/o imágenes existentes en la Galería, con fondos, zoom de cámara, pistas de textos y opción de exportación WebM a la Galería. No consume IA externa.
3. Generación **nativa de clips IA** a partir de texto o imagen mediante **Luma Agents ray-3.2**, si se configura `LUMA_AGENTS_API_KEY` en Render. Clips de 5 o 10 segundos, 720p, formato social escogido. El proyecto guarda la solicitud, consulta el resultado, descarga el MP4 al disco persistente del tenant e incorpora el recurso en la Galería y en Video Studio.
4. El control de generación muestra el estado, limitaciones y requerimiento de credencial en lugar de afirmar que se generó un video cuando el proveedor no está conectado.

## Archivos nuevos

- `studio_video_v2719.go` (API y trabajos IA, autenticación, tenant, protección de descargas)
- `static/worktic-studio-v2719.js` (interfaz y generación local/IA)
- `static/worktic-studio-v2719.css` (previsualización y responsive)
- `RELEASE_V27_19_STUDIO_VIDEO_CREATOR.md`

## Archivos modificados

- `VERSION.txt`
- `main.go` (inicialización de esquema aditivo y ruta API)
- `static/app.html` (UI y carga de recursos V27.19)
- `.env.render.example` (clave opcional sin valor)

No se modifica Worktic Partners, Social Hub, CRM, WhatsApp, las tablas existentes ni el Dockerfile Go 1.26.

## Configuración para videos generativos reales

**IMPORTANTE:** `OPENAI_API_KEY` permite funcionalidades OpenAI existentes, pero NO habilita video generativo nativo en esta versión. La antigua Videos API de OpenAI no está disponible; la ruta IA utiliza un proveedor de video independiente.

En Render → `worktic-ai` → Environment agrega la clave de tu cuenta de **Luma Agents**:

```
LUMA_AGENTS_API_KEY=tu_clave_luma_agents
```

Obtén acceso comercial y saldo desde la plataforma del proveedor. **No introduzcas la clave en el navegador, GitHub ni `.env.render.example`.** Se usan llamadas autenticadas desde Go a `https://agents.lumalabs.ai/v1/generations` con modelo `ray-3.2`. Su disponibilidad, tarifas, acceso a 720p/5s/10s y límites dependen del proveedor. Es recomendable probar con créditos y una sola generación en staging primero.

Para controlar el gasto, hay límites iniciales **por usuario y día**: Personal 1, Negocio 2, Empresa 5, Free 0. Estos límites son provisionales y requieren aprobación comercial. Se registra una generación solicitada aunque el proveedor pueda fallar, para evitar uso no controlado. Las generaciones fallidas deben revisarse manualmente para posibles créditos compensatorios del proveedor.

## API privada

- `GET /api/studio/video/generate?status=config` — proveedor configurado (sin exponer la clave).
- `POST /api/studio/video/generate` — inicia generación con `{prompt,ratio,duration,image_url?}`. `image_url` debe pertenecer a la Galería del tenant.
- `GET /api/studio/video/generate?id=<uuid>` — consulta el trabajo y descarga el resultado MP4 cuando esté disponible.

El nuevo esquema `studio_video_jobs_v2719` se crea de forma **aditiva** con el arranque. No ejecutar SQLite → PostgreSQL. Las tablas anteriores no se borran.

El máximo de 50 MB por archivo se mantiene; pueden existir videos del proveedor que lo excedan y no puedan guardarse. El frontend permite recuperar una generación pendiente durante la misma sesión del navegador.

## Pruebas realizadas fuera de producción

- `node --check` de los JS — pasó.
- Parser sintáctico de 46 Go files — pasó.
- Compilación aislada del handler Go V27.19 con tipos stub — pasó.
- HTML: 461 identificadores, sin duplicados.
- Chromium escritorio: Video Studio muestra lienzo de unos **475 px de alto** en ventana 1366×768, comparado con la previsualización pequeña de V27.18; ocultar timeline aumenta el espacio hasta ~660 px.
- Generador local creó 3 clips, textos editables y exportó un WebM de ~1.1 MB con navegador simulado y recurso guardado en biblioteca simulada.
- Simulación de API de video externo: solicitar, consultar, recibir MP4 y añadir a la línea de tiempo — pasó, **no es una prueba con Luma real**.
- Responsive 390px, sin desbordamiento horizontal; sin errores JavaScript en la prueba.

**Pendientes:** `go test ./...` y `go build ./...` con Go 1.26 y dependencias descargadas, prueba con PostgreSQL real, prueba real de la API de Luma, almacenamiento persistente, permisos y regresión de módulos en staging. Este entorno no tiene acceso a descargar Go 1.26 ni dependencias del proyecto. No desplegar directo en producción sin validar estas pruebas.

## Instalación recomendada

1. Confirmar `origin/main` y Render `Live` en V27.18; si son distintos, no mezclar versiones.
2. Usar un Git worktree nuevo desde el commit actual de `main` y copiar solo los ocho archivos nuevos/modificados indicados arriba.
3. Ejecutar `node --check` sobre nuevos JS, `gofmt -l main.go studio_video_v2719.go`, `git diff --check`, `go test ./...`, `go build ./...`, y preferiblemente `docker build -t workticai:v27.19 .` con Go 1.26.
4. Validar en staging con almacenamiento y base de datos separados y una generación real autorizada.
5. Crear commit, enviar rama, fusionar a `main` y desplegar el último commit desde Render.
6. Probar video local WebM, la generación IA MP4, regreso a Galería, videos por formato y módulos CRM/Partners/Social Hub.

Para revertir el código, usar el último despliegue V27.18 Live en Render. **El rollback no borra automáticamente nuevas filas de los trabajos de generación ni los archivos persistentes.**
