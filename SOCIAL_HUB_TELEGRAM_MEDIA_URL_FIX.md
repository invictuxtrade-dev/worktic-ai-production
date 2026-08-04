# Corrección Telegram: recursos multimedia

- Convierte rutas internas `/uploads/social/...` en URL pública usando `BASE_URL`.
- Acepta URLs HTTPS/HTTP completas y URLs pegadas sin esquema.
- Publicaciones sin recurso usan `sendMessage`.
- Imágenes usan `sendPhoto` y videos `sendVideo` únicamente con URL pública válida.
- La misma lógica se usa para publicar ahora, publicaciones programadas y reintentos.
- Evita el error `Bad Request: invalid file HTTP URL specified: URL host is empty`.

## Render
Verifica que exista:

```env
BASE_URL=https://workticai.com
```

Debe corresponder al dominio público que sirve `/uploads/social/`.
