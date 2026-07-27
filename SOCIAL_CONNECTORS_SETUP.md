# Worktic AI Social Hub — Conectores oficiales

## Variables requeridas

Configura en Render o en `.env`:

- `BASE_URL=https://tu-dominio-publico.com`
- `CHANNEL_ENCRYPTION_KEY=` una clave aleatoria larga y estable
- `META_APP_ID`, `META_APP_SECRET`
- `LINKEDIN_CLIENT_ID`, `LINKEDIN_CLIENT_SECRET`
- `TIKTOK_CLIENT_KEY`, `TIKTOK_CLIENT_SECRET`
- `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`

## URL de retorno OAuth

Registra exactamente estas URL en cada consola:

- Meta: `${BASE_URL}/api/social/oauth/callback/facebook`
- LinkedIn: `${BASE_URL}/api/social/oauth/callback/linkedin`
- TikTok: `${BASE_URL}/api/social/oauth/callback/tiktok`
- Google/YouTube: `${BASE_URL}/api/social/oauth/callback/youtube`

Instagram se descubre automáticamente desde las páginas de Facebook vinculadas y utiliza la conexión Meta.

## Telegram

1. Crea el bot con BotFather.
2. Añádelo como administrador del canal.
3. En Social Hub selecciona Telegram → API oficial.
4. Introduce el token y el identificador del canal (`@canal` o `-100...`).

## Capacidades incluidas

- Facebook Pages: texto, enlaces e imágenes por URL pública.
- Instagram profesional: imagen y Reel/video mediante URL pública.
- LinkedIn: publicaciones de texto y artículo/enlace.
- Telegram: texto e imagen por URL.
- TikTok: video directo y fotografía desde dominio verificado.
- YouTube: carga resumible de video desde URL pública.
- Cola persistente, reintentos, estados, aislamiento por tenant y bitácora de intentos.

## Requisitos externos

Las credenciales no sustituyen las aprobaciones de cada proveedor. Meta, LinkedIn y TikTok pueden exigir revisión de aplicación o productos adicionales. TikTok exige verificar los dominios usados para archivos remotos. YouTube aplica cuotas y políticas de auditoría. Los archivos multimedia deben estar disponibles mediante HTTPS público.

## Prueba segura

Antes de usar cuentas reales, crea conexiones en modo Sandbox. Después conecta una cuenta oficial y prueba una publicación privada o de bajo impacto.
