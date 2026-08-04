# Worktic AI — Guía exacta de conexiones oficiales de Social Hub

## 1. Configuración obligatoria en Render

En **Render > servicio Worktic > Environment**, agrega estas variables:

```env
BASE_URL=https://workticai.com
CHANNEL_ENCRYPTION_KEY=UNA_CLAVE_ALEATORIA_LARGA_Y_ESTABLE
META_GRAPH_VERSION=v25.0
META_APP_ID=
META_APP_SECRET=
LINKEDIN_CLIENT_ID=
LINKEDIN_CLIENT_SECRET=
TIKTOK_CLIENT_KEY=
TIKTOK_CLIENT_SECRET=
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
```

Reglas importantes:

- `BASE_URL` no debe terminar en `/`.
- Debe coincidir exactamente con el dominio desde el cual abre Worktic.
- `CHANNEL_ENCRYPTION_KEY` debe tener al menos 32 caracteres y no debe cambiarse después de conectar cuentas. Si se cambia, los tokens guardados dejarán de poder descifrarse.
- Después de modificar variables, realiza **Manual Deploy > Deploy latest commit**.

## 2. URLs de retorno OAuth

Registra exactamente estas URLs, respetando HTTPS, dominio y ruta:

```text
https://workticai.com/api/social/oauth/callback/facebook
https://workticai.com/api/social/oauth/callback/instagram
https://workticai.com/api/social/oauth/callback/linkedin
https://workticai.com/api/social/oauth/callback/tiktok
https://workticai.com/api/social/oauth/callback/youtube
```

No uses `app.html` en las URLs de retorno.

---

## 3. Facebook e Instagram

### Requisitos de la cuenta

- La persona que conecta debe administrar una Página de Facebook.
- La cuenta de Instagram debe ser profesional: Empresa o Creador.
- Instagram debe estar vinculada a la Página de Facebook correspondiente.

### Configuración en Meta for Developers

1. Crea o abre la aplicación de Worktic.
2. Configura el dominio `workticai.com` en **App domains**.
3. Agrega la URL de política de privacidad y la URL de eliminación de datos.
4. Agrega el producto de inicio de sesión de Facebook/Meta requerido por la aplicación.
5. En las URI válidas de OAuth agrega:

```text
https://workticai.com/api/social/oauth/callback/facebook
https://workticai.com/api/social/oauth/callback/instagram
```

6. Solicita o habilita los permisos:

```text
pages_show_list
pages_read_engagement
pages_manage_posts
instagram_basic
instagram_content_publish
business_management
```

7. Mientras la app esté en modo desarrollo, únicamente podrán conectarse administradores, desarrolladores o testers de la app.
8. Para usuarios externos, la app debe estar en modo Live y los permisos avanzados deben haber sido aprobados cuando Meta lo requiera.

### En Worktic

1. Entra a **Social Hub > Conexiones**.
2. Pulsa **Conectar red**.
3. Selecciona Facebook o Instagram.
4. Selecciona **API oficial**.
5. Autoriza todas las páginas solicitadas.
6. Worktic guardará automáticamente la Página y la cuenta profesional de Instagram vinculada.
7. Pulsa **Probar**. Debe mostrar “Conexión oficial verificada correctamente”.

### Fallos típicos

- “No se encontraron páginas administradas”: el usuario no administra una Página o no concedió acceso a ninguna.
- Instagram no aparece: la cuenta no es profesional o no está vinculada a la Página.
- Error de redirect URI: la URL registrada en Meta no coincide exactamente con `BASE_URL`.
- Funciona para ti pero no para terceros: la aplicación sigue en modo desarrollo o faltan permisos aprobados.

---

## 4. Telegram

1. Abre Telegram y conversa con `@BotFather`.
2. Ejecuta `/newbot` y copia el token.
3. Añade el bot como administrador del canal o grupo.
4. Dale permiso para publicar mensajes.
5. En Worktic selecciona **Telegram > API oficial**.
6. Introduce:
   - Nombre visible.
   - Token del bot.
   - `@usuario_del_canal` para canales públicos, o el ID `-100...` para canales privados.
7. Pulsa **Probar**.

Para obtener el ID de un canal privado, publica un mensaje y consulta `getUpdates` del bot, o usa una herramienta segura propia. El bot debe estar dentro del canal antes de realizar la consulta.

Worktic ahora prueba realmente `getChat` y publica texto, imagen o video según el recurso seleccionado.

---

## 5. LinkedIn

### En LinkedIn Developers

1. Crea una aplicación y asóciala a una Página de LinkedIn.
2. En **Auth**, agrega:

```text
https://workticai.com/api/social/oauth/callback/linkedin
```

3. Activa los productos:
   - **Sign In with LinkedIn using OpenID Connect**.
   - **Share on LinkedIn**.
4. Verifica que estén disponibles los scopes:

```text
openid
profile
w_member_social
```

5. Copia Client ID y Client Secret a Render.

### En Worktic

Selecciona **LinkedIn > API oficial**, autoriza la cuenta y después pulsa **Probar**.

La conexión actual publica como el miembro autenticado. Publicar como Página empresarial requiere permisos de organización y aprobación adicional de LinkedIn.

---

## 6. TikTok

### En TikTok for Developers

1. Crea una aplicación web.
2. Agrega Login Kit y Content Posting API.
3. Registra:

```text
https://workticai.com/api/social/oauth/callback/tiktok
```

4. Solicita los scopes:

```text
user.info.basic
video.upload
video.publish
```

5. Verifica el dominio desde el cual TikTok descargará videos o imágenes.
6. Solicita revisión para Direct Post. Sin aprobación, TikTok puede limitar publicaciones a cuentas de prueba o privacidad restringida.
7. Copia Client Key y Client Secret a Render.

Worktic conserva y renueva el refresh token de TikTok automáticamente.

Los recursos enviados por URL deben ser HTTPS públicos y estar alojados en un dominio verificado por TikTok.

---

## 7. YouTube

### En Google Cloud Console

1. Crea o selecciona un proyecto.
2. Habilita **YouTube Data API v3**.
3. Configura la pantalla de consentimiento OAuth.
4. Crea credenciales **OAuth Client ID > Web application**.
5. Agrega como Authorized redirect URI:

```text
https://workticai.com/api/social/oauth/callback/youtube
```

6. Agrega los scopes de YouTube solicitados por Worktic.
7. Si la aplicación está en modo Testing, agrega las cuentas como Test users.
8. Copia Client ID y Client Secret a Render.

Worktic solicita acceso offline, guarda el refresh token y lo renueva automáticamente antes de probar o publicar.

La cuenta autorizada debe tener un canal de YouTube creado.

---

## 8. Orden correcto de prueba

Prueba una red a la vez:

1. Conectar.
2. Pulsar **Probar**.
3. Crear una publicación de prueba.
4. Guardarla como borrador.
5. Publicarla manualmente.
6. Confirmar el enlace o ID externo.
7. Programar una segunda publicación para 5 minutos después.
8. Revisar que cambie de `scheduled` a `published`.

Orden recomendado:

```text
Telegram → Facebook → Instagram → LinkedIn → YouTube → TikTok
```

## 9. Qué corrige esta versión

- La prueba de conexión ya no revisa únicamente el estado guardado: consulta realmente la API del proveedor.
- Registra el error real en la tarjeta de conexión.
- Cambia una conexión fallida a estado `attention`.
- Intercambia el token corto de Meta por un token de larga duración.
- Renueva automáticamente los tokens de Google/YouTube y TikTok.
- Facebook distingue entre publicación de imagen y video.
- Telegram distingue entre `sendPhoto` y `sendVideo`.
- Mantiene aislamiento por empresa y cifrado de credenciales.
