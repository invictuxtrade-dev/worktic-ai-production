# WorkticAI — Despliegue final V26 en Render

Esta guía asume que la versión actualmente pública sigue siendo la versión anterior a V15 y que **ninguna V15–V25 fue desplegada**, tal como se acordó.

La actualización final hace un único salto hasta V26.

---

# Principio de seguridad del corte

El SQLite actual permanece intacto durante toda la migración. V26 se despliega primero en **modo mantenimiento**, prepara PostgreSQL y solo después copia los datos.

Mientras `MAINTENANCE_MODE=true`, no deben entrar escrituras de usuarios.

---

# Fase 0 — Antes de tocar producción

1. Guarda/exporta todas las variables actuales de Render.
2. Identifica y conserva **exactamente** el valor actual de:

```text
CHANNEL_ENCRYPTION_KEY
```

3. No borres el Persistent Disk actual.
4. No borres `/var/data/worktic.db`.
5. No borres `/var/data/wa_sessions/`.
6. Conserva el último deploy estable en Render para rollback.
7. Ten preparado el ZIP/repo V26 y no actives auto-deploy durante la migración.

---

# Fase 1 — Crear infraestructura administrada

Crear en la misma región de Render. El Blueprint final usa como base de producción:

```text
Web Worktic AI: 1c-2g
PostgreSQL:      0.5c-1g
Key Value:       256mb
```

Estos tamaños son un punto de partida; escala según concurrencia, número de canales y carga de workers.

## PostgreSQL

Recomendación inicial incluida en `render.yaml`:

```text
name: worktic-db
plan: 0.5c-1g
database: worktic
private network only
```

## Render Key Value

```text
name: worktic-cache
plan: 256mb
private network only
maxmemoryPolicy: noeviction
persistenceMode: journal-snapshot
```

Conectar al web service:

```env
DATABASE_DRIVER=postgres
DATABASE_URL=<connectionString interna de worktic-db>
REDIS_URL=<connectionString interna de worktic-cache>
REDIS_REQUIRED=true
```

---

# Fase 2 — Crear/actualizar variables

Usa `VARIABLES_FINALES_V26.md` como lista maestra.

Antes del primer deploy V26 establece temporalmente:

```env
MAINTENANCE_MODE=true
```

Y verifica especialmente:

```env
APP_ENV=production
BASE_URL=https://TU-DOMINIO
CHANNEL_ENCRYPTION_KEY=<MISMO VALOR ACTUAL>
DATABASE_DRIVER=postgres
REDIS_REQUIRED=true
TRUST_PROXY=true
BACKGROUND_WORKERS_ENABLED=true
CHANNEL_RUNTIMES_ENABLED=true
LEGACY_WHATSAPP_DRIVER=sqlite
LEGACY_WHATSAPP_DSN=file:/var/data/worktic.db?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)
```

No uses `BOOTSTRAP_ADMIN_*` en esta migración porque los usuarios actuales se copiarán desde SQLite.

---

# Fase 3 — Desplegar V26 en mantenimiento

Despliega V26 con:

```env
MAINTENANCE_MODE=true
```

Comprobar:

```text
GET /healthz -> 200 + maintenance=true
GET /readyz  -> 200 + database=ready
```

La interfaz normal debe devolver HTTP 503 con la pantalla de actualización.

Si `/readyz` falla, NO continúes con la migración.

---

# Fase 4 — Backup consistente del Persistent Disk

Abre Render Shell del web service y ejecuta:

```bash
/app/scripts/pre_cutover_backup.sh
```

Debe producir algo como:

```text
/var/data/backups/worktic_pre_v26_YYYYMMDDTHHMMSSZ.tar.gz
/var/data/backups/worktic_pre_v26_YYYYMMDDTHHMMSSZ.tar.gz.sha256
```

Conserva el nombre y checksum en tu registro de cambio.

---

# Fase 5 — Inspección de migración

Primero:

```bash
/app/worktic-migrate --dry-run
```

El dry-run valida que cada columna histórica tenga destino en PostgreSQL y mostrará `OK <tabla>` sin modificar datos. También lista las tablas que existen solo en SQLite; normalmente corresponden al store WhatsApp/Whatsmeow y permanecen deliberadamente en el disco legacy.

---

# Fase 6 — Migración SQLite → PostgreSQL

Ejecuta:

```bash
/app/worktic-migrate
```

Por defecto toma:

```text
source = /var/data/worktic.db
target = $DATABASE_URL
```

El migrador:

1. comprueba origen y destino;
2. detecta tablas compatibles;
3. detiene la migración si una columna histórica no existe en el esquema nuevo;
4. trunca las tablas comunes del PostgreSQL de destino;
5. copia filas preservando IDs;
6. restablece secuencias;
7. compara conteos SQLite/PostgreSQL;
8. registra la migración V26.

Debe terminar con mensajes equivalentes a:

```text
verificación de conteos OK
migración completada
```

`app_sessions` no se copia intencionalmente.

Si el migrador falla, **no desactives mantenimiento**. Corrige el problema y vuelve a ejecutar; está diseñado para repetirse mientras el destino siga siendo el nuevo PostgreSQL.

---

# Fase 7 — Abrir V26

Cambia:

```env
MAINTENANCE_MODE=false
```

Haz deploy/restart.

Comprobar:

```bash
/app/scripts/post_cutover_check.sh
```

O manualmente:

```text
GET /healthz
GET /readyz
/login.html
```

---

# Fase 8 — Login y migración automática de contraseñas

Todos los usuarios deberán iniciar sesión otra vez porque las sesiones antiguas no se migran.

Un usuario con contraseña histórica válida podrá entrar normalmente. Después del primer login correcto, su hash queda actualizado a Argon2id automáticamente.

No es necesario pedir un reset masivo de contraseñas.

---

# Fase 9 — Validar datos críticos antes de habilitar integraciones nuevas

En este orden:

1. Superadmin.
2. Tenants/empresas.
3. Usuarios y roles.
4. CRM y contactos.
5. Oportunidades/pipeline.
6. Catálogo.
7. Agenda.
8. Planes/facturación.
9. Agentes IA.
10. Automatizaciones.
11. Inbox.
12. Social Hub/calendario.
13. Analytics.
14. Ads Center.
15. Reinicio del servicio y segunda verificación de persistencia.

---

# Fase 10 — Validar canales actuales

Primero los canales que ya existían antes del upgrade:

1. WhatsApp QR principal.
2. WhatsApp QR multicanal en `/var/data/wa_sessions/`.
3. Telegram.
4. Messenger.
5. Facebook/Instagram Social Hub.
6. LinkedIn.
7. TikTok.
8. YouTube.

**No borres `worktic.db` después de migrar**: V26 todavía lo usa como store heredado para WhatsApp QR principal.

---

# Fase 11 — Reautorizar OAuth por los nuevos scopes

Después de confirmar que la plataforma base está estable, reconecta las cuentas necesarias para aceptar scopes añadidos desde V15–V25.

Revisar:

- Facebook/Instagram Social;
- Meta Lead Ads;
- YouTube comentarios;
- Meta Ads;
- TikTok Ads;
- Google Ads.

Las URLs exactas están en `VARIABLES_FINALES_V26.md`.

---

# Fase 12 — Meta oficial

Configurar/revisar en Meta Developers:

```text
Messenger webhook      {BASE_URL}/webhooks/meta/messenger
WhatsApp webhook       {BASE_URL}/webhooks/meta/whatsapp
Lead Ads webhook       {BASE_URL}/webhooks/meta/leads
```

Y sus Verify Tokens correspondientes.

Configurar Embedded Signup y `META_WHATSAPP_CONFIG_ID`.

Después probar:

1. Conectar WhatsApp Business mediante Embedded Signup.
2. WABA/Phone Number ID.
3. Webhook messages.
4. Texto desde Inbox.
5. Template aprobado.
6. Estado sent/delivered/read.
7. Lead Ad de prueba.
8. Lead → CRM → oportunidad → Automation Engine.

---

# Fase 13 — Social Hub / Autopilot

1. Confirmar modo Governance deseado: Autopilot / Híbrido / Aprobación.
2. Probar borrador.
3. Probar publicación inmediata.
4. Probar programación.
5. Probar pausa global.
6. Probar carrusel/multiasset donde el proveedor lo soporte.
7. Probar Community comments.
8. Probar reutilización IA.

---

# Fase 14 — Analytics y Ads

1. Definir moneda del tenant.
2. Probar First Touch.
3. Probar Last Touch.
4. Probar `/t/{tenant}/{post}`.
5. Completar una landing y confirmar journey.
6. Marcar una oportunidad como Ganado.
7. Confirmar revenue real.
8. Conectar Meta Ads/TikTok Ads/Google Ads.
9. Confirmar spend oficial.
10. Confirmar que monedas diferentes no se mezclan.
11. Comparar Platform ROAS vs Worktic ROAS.

---

# Fase 15 — Automatizaciones

Probar como mínimo:

```text
lead.created
message.received
conversation.assigned
conversation.mode_changed
conversation.closed
social.comment
```

Y acciones:

```text
condición
etiqueta
pipeline
owner
WhatsApp Cloud
IA
webhook HTTPS seguro
```

Para webhooks externos, si deseas una política estricta configura:

```env
AUTOMATION_WEBHOOK_ALLOWED_HOSTS=api.empresa.com,hooks.partner.com
```

---

# Fase 16 — Backup PostgreSQL después del corte

Cuando todo esté validado:

1. crea un backup/export lógico de PostgreSQL;
2. conserva una copia fuera de Render;
3. mantén PITR habilitado mediante un plan PostgreSQL pagado;
4. conserva además el backup `worktic_pre_v26_*.tar.gz`;
5. no elimines la base SQLite activa del store QR.

---

# Rollback

## Antes de abrir V26 al público

Si falla algo durante mantenimiento:

1. deja `MAINTENANCE_MODE=true`;
2. no modifiques el SQLite original;
3. revierte el deploy al último release estable;
4. restaura las variables anteriores si las habías cambiado;
5. vuelve a usar el Persistent Disk original.

El SQLite original y las sesiones QR no han sido destruidos.

## Después de abrir V26 y recibir escrituras

No vuelvas simplemente al SQLite antiguo: los datos nuevos ya estarán en PostgreSQL y podrían perderse.

En ese caso:

1. activa mantenimiento;
2. toma backup PostgreSQL;
3. determina qué datos nuevos deben reconciliarse;
4. solo después realiza rollback o restauración.

---

# Criterio de éxito final

La migración se considera terminada cuando:

- `/readyz` está verde;
- Redis está disponible;
- PostgreSQL contiene todos los datos históricos esperados;
- usuarios pueden iniciar sesión;
- tenants están aislados;
- CRM/agenda/catálogo funcionan;
- WhatsApp QR conserva sesiones;
- WhatsApp Cloud y Meta webhooks funcionan;
- Social Hub publica/programa;
- Automation Engine ejecuta;
- Inbox IA↔humano funciona;
- Analytics atribuye sin doble conteo;
- Ads Center sincroniza gasto;
- existe backup post-corte;
- `MAINTENANCE_MODE=false`;
- `ALLOW_SQLITE_PRODUCTION` no está habilitado.
