# WorkticAI V26 — Production / Enterprise Hardening

V26 convierte la rama acumulativa V15→V25 en una base de producción comercial. No elimina los módulos construidos; endurece su infraestructura, autenticación, observabilidad y proceso de migración.

## 1. Base de datos principal: PostgreSQL

La base transaccional del SaaS pasa de SQLite a PostgreSQL mediante `DATABASE_URL`.

Migran a PostgreSQL, entre otros:

- usuarios, tenants, equipo y facturación;
- CRM, oportunidades, catálogo y agenda;
- conexiones/canales y eventos de aplicación;
- Social Hub, Governance, Community y Analytics;
- WhatsApp Business Cloud y templates;
- Meta Lead Ads;
- Automation Engine;
- Inbox;
- Agentes IA 2.0 y memoria;
- Ads Center.

El helper `DB` mantiene compatibilidad de consultas existentes y adapta placeholders, autoincrementos y algunas expresiones SQLite a PostgreSQL.

### WhatsApp QR heredado

V26 conserva el Persistent Disk. El almacenamiento de dispositivo de WhatsApp QR heredado continúa en SQLite (`LEGACY_WHATSAPP_DSN`) y los stores multicanal permanecen en `/var/data/wa_sessions/`.

**No borrar `/var/data/worktic.db` ni `/var/data/wa_sessions/` mientras existan sesiones QR heredadas.** La información comercial ya no usa SQLite como base principal después del corte, pero WhatsApp QR sí puede seguir usando sus stores locales.

## 2. Migrador incluido

Se compila un segundo binario:

```text
/app/worktic-migrate
```

Su función es copiar la base actual `/var/data/worktic.db` al PostgreSQL nuevo.

Características:

- preserva IDs;
- solo copia tablas compatibles;
- verifica que ninguna columna del origen quede sin destino;
- limpia PostgreSQL antes de una ejecución real para que sea repetible durante mantenimiento;
- restablece secuencias PostgreSQL;
- compara conteos de filas al terminar;
- no migra `app_sessions`;
- no toca los stores WhatsApp QR;
- registra `sqlite_to_postgres_v26` en `production_migrations_v26`.

Comandos:

```bash
/app/worktic-migrate --dry-run
/app/worktic-migrate
```

## 3. Modo mantenimiento

Nueva variable temporal:

```env
MAINTENANCE_MODE=true
```

Con ella Worktic crea/actualiza el esquema PostgreSQL y mantiene únicamente:

```text
/healthz
/readyz
```

disponibles. El resto devuelve una página 503 de actualización.

Esto permite detener escrituras durante el backup y la migración sin perder el health check de Render.

Después del corte:

```env
MAINTENANCE_MODE=false
```

## 4. Contraseñas

V26 elimina las credenciales administrativas iniciales hardcodeadas de versiones antiguas.

Nuevas contraseñas se guardan usando Argon2id con:

```text
memory = 19 MiB
iterations = 2
parallelism = 1
salt aleatorio por contraseña
```

Se mantiene lectura temporal de:

- hashes legacy de Worktic;
- bcrypt si existiese.

Cuando un usuario inicia sesión correctamente con un hash antiguo, Worktic lo reescribe automáticamente en Argon2id.

Nuevas contraseñas/invitaciones/cambios administrativos exigen mínimo 12 caracteres.

## 5. Sesiones

El navegador recibe un token aleatorio de sesión, pero PostgreSQL guarda únicamente SHA-256 del token.

Las sesiones anteriores **no se migran**, por lo que después de V26 todos los usuarios inician sesión nuevamente. Esto también dispara progresivamente el rehash Argon2id de cuentas legacy.

## 6. Redis / Render Key Value

V26 soporta `REDIS_URL` y lo requiere en producción cuando:

```env
REDIS_REQUIRED=true
```

En esta fase Redis se usa para rate limiting coordinado entre procesos y queda preparado como infraestructura compartida para evolución de workers/colas.

Si Redis no es obligatorio (desarrollo), existe fallback de rate limiting en memoria.

## 7. Procesos y workers

Se añaden dos controles:

```env
BACKGROUND_WORKERS_ENABLED=true
CHANNEL_RUNTIMES_ENABLED=true
```

`BACKGROUND_WORKERS_ENABLED` controla:

- Social Publisher;
- Automation Engine worker;
- Analytics sync;
- Ads Center sync;
- Messenger sync/outbox/token monitor.

`CHANNEL_RUNTIMES_ENABLED` controla runtimes sessionful:

- restauración de canales activos;
- WhatsApp QR;
- Telegram loop;
- runtime de Channel Manager.

En el despliegue Render actual ambos se dejan en `true`. La separación permite mover jobs a procesos dedicados posteriormente sin duplicarlos.

## 8. Rate limiting y protección HTTP

V26 aplica límites por IP/ruta, con límites reforzados para login y registro.

También incorpora:

- límite de body HTTP;
- protección de Origin para mutaciones `/api/*`;
- `X-Request-ID`;
- logs por request con método, path, status, bytes y latencia;
- `X-Content-Type-Options`;
- `X-Frame-Options`;
- `Referrer-Policy`;
- `Permissions-Policy`;
- COOP/CORP;
- Content Security Policy;
- HSTS en producción;
- `Cache-Control: no-store` para APIs.

## 9. Auditoría de seguridad

Nueva tabla:

```text
security_audit_v26
```

Inicialmente registra eventos como login correcto/fallido y queda como base para más eventos de seguridad.

## 10. Automation Webhooks seguros

El nodo `webhook` de V20 deja de ser placeholder.

V26 permite POST HTTPS real, pero bloquea:

- HTTP sin TLS;
- localhost;
- IP privadas;
- link-local;
- multicast;
- rangos reservados/documentación;
- credenciales embebidas en URL;
- redirects inseguros;
- respuestas excesivamente grandes.

Timeout total: 10 segundos.

Allowlist opcional:

```env
AUTOMATION_WEBHOOK_ALLOWED_HOSTS=api.midominio.com,hooks.proveedor.com
```

Si se deja vacía, siguen permitiéndose únicamente hosts HTTPS públicos.

## 11. Health / readiness

```text
GET /healthz
GET /readyz
```

`/readyz` comprueba PostgreSQL y Redis cuando `REDIS_REQUIRED=true`.

## 12. Backup de pre-corte

Incluido:

```text
/app/scripts/pre_cutover_backup.sh
```

Crea dentro de `/var/data/backups/` un `.tar.gz` y un `.sha256` que incluyen, si existen:

- `worktic.db`;
- `wa_sessions/`;
- `uploads/`.

Ejecutarlo únicamente con `MAINTENANCE_MODE=true`.

## 13. Post-cutover smoke check

Incluido:

```text
/app/scripts/post_cutover_check.sh
```

Valida `healthz`, `readyz` y acceso a la página de login.

## 14. Nuevas dependencias V26

```text
github.com/lib/pq
github.com/redis/go-redis/v9
golang.org/x/crypto
```

El Dockerfile compila:

```text
/app/worktic-ai
/app/worktic-migrate
```

## 15. Tablas nuevas V26

```text
security_audit_v26
production_migrations_v26
```

No existe SQL manual para la aplicación normal: el esquema es idempotente y se crea al arranque. El migrador se utiliza únicamente para trasladar los datos históricos del SQLite desplegado actualmente.


## Perfil inicial recomendado en Render

El Blueprint final usa `1c-2g` para el servicio web, `0.5c-1g` para PostgreSQL y `256mb` para Key Value. Ajusta hacia arriba según concurrencia, cantidad de canales y workers.
