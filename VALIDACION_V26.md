# WorkticAI V26 — Informe de validación

Fecha: 27 de septiembre de 2026.

## Resultado

La rama acumulativa V15→V26 fue validada estáticamente y por componentes antes de empaquetarse. No se modificó el servicio Render en producción.

## Validaciones ejecutadas

- `gofmt` sobre todos los archivos Go.
- `node --check` sobre todos los archivos JavaScript de `static/`.
- Validación YAML de `render.yaml`.
- Verificación de IDs HTML duplicados: 0 encontrados.
- `sh -n` sobre los scripts de backup y post-cutover.
- Auditoría de tablas `AUTOINCREMENT` frente al mapa de secuencias PostgreSQL: 72/72 cubiertas.
- Búsqueda de credenciales legacy hardcodeadas: no presentes.
- Revisión de SQL SQLite específico (`INSERT OR IGNORE`, `datetime`, `instr`) y cobertura en la capa de compatibilidad PostgreSQL.
- Type-check aislado de `db_compat.go` + `production_v26.go` con stubs: PASS.
- Type-check aislado del binario `worktic-migrate`: PASS.
- Dependencias nuevas fijadas en `go.mod`: `github.com/lib/pq v1.12.3`, `github.com/redis/go-redis/v9 v9.22.0`, `golang.org/x/crypto v0.57.0`.

## Limitación de este entorno

No fue posible ejecutar `go test ./...` del proyecto completo porque el entorno de trabajo dispone de Go 1.23.2 y el proyecto exige Go 1.24+, y además no puede descargar el toolchain/módulos externos desde Internet. El Dockerfile final usa Go 1.25 y ejecuta `go mod download`, `go mod verify` y ambos `go build` durante el build de Render.

Por esta razón, el build Docker/Render real es una puerta obligatoria antes del corte de datos. Si el build falla, no debe iniciarse la migración.

## Validaciones obligatorias durante el corte

1. El build Docker debe finalizar correctamente.
2. `/healthz` y `/readyz` deben responder 200 en modo mantenimiento.
3. `worktic-migrate --dry-run` debe completar sin incompatibilidades de columnas.
4. El migrador real debe terminar con verificación exacta de conteos.
5. Tras desactivar mantenimiento, ejecutar `scripts/post_cutover_check.sh`.
6. Validar login, CRM, canales, Inbox, automatizaciones, Social Hub, Analytics y Ads Center antes de considerar cerrado el cambio.

## Política de rollback

Antes de abrir V26 a usuarios, el rollback hacia la versión anterior es seguro porque el SQLite original no se elimina. Después de aceptar nuevas escrituras en PostgreSQL, no se debe volver ciegamente al release anterior: habría que reconciliar los datos generados tras el corte.
