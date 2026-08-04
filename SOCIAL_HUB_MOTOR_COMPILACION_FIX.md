# Social Hub - corrección de compilación del motor

Se corrigió el fallo de despliegue de Render:

- `social_providers.go: undefined: log`

La causa era que el worker de publicaciones utilizaba `log.Printf` sin importar el paquete estándar `log`.

## Conservado sin cambios

- Compositor IA y modo manual.
- Vista previa por red.
- Edición completa de publicaciones.
- Calendario y programación.
- Conexión oficial de Telegram.
- Códigos promocionales.
- Cola, reintentos y recuperación del scheduler.

## Comportamiento del worker

- Arranca junto con la aplicación.
- Ejecuta una revisión inmediata.
- Revisa publicaciones cada cinco segundos.
- Evita dobles publicaciones con reclamo atómico.
- Recupera trabajos interrumpidos.
- Registra errores y reintenta automáticamente.
