# Social Hub — motor de publicación estable

## Correcciones

- El worker procesa la cola inmediatamente al arrancar y luego cada 5 segundos.
- Las fechas RFC3339 se convierten a UTC y se comparan como fechas reales, no como texto SQL.
- Reclamo atómico de cada trabajo para evitar publicaciones duplicadas con múltiples instancias.
- Recuperación de publicaciones que queden en `publishing` por reinicios del servidor.
- Tiempo máximo de 90 segundos por intento.
- Registro visible de errores y hasta 4 intentos automáticos.
- Los errores previos al proveedor (conexión o credenciales) ya no dejan el contenido eternamente en cola.
- Logs de Render con prefijo `social publisher:` para diagnóstico.

## Flujo

`scheduled/queued` → `publishing` → `published`

En caso de error:

`publishing` → `retrying` → `failed`

## Prueba recomendada

1. Desplegar esta versión.
2. Confirmar en Render: `social publisher: worker iniciado`.
3. Crear una publicación de Telegram para 2 minutos después.
4. Al llegar la hora debe procesarse en un máximo aproximado de 5 segundos.
5. Si Telegram devuelve un error, se verá en la publicación y en los logs de Render.
