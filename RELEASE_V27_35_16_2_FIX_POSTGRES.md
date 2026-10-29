# WorkticAI V27.35.16.2 — Hotfix PostgreSQL (BYTEA)

## Causa confirmada

Render detuvo el proceso con `catalog commerce schema: pq: type "blob" does not exist at column 145 (42704)` durante el despliegue de `V27.35.16.1` (`6edc4ba`). La sentencia `CREATE TABLE IF NOT EXISTS catalog_sale_evidence_v2735161` usaba `content BLOB NOT NULL`, válido en SQLite pero no en PostgreSQL.

## Corrección

La función `catalogSaleEvidenceDDLV2735162(postgres bool)` genera `content BYTEA NOT NULL` para PostgreSQL y `content BLOB NOT NULL` para SQLite. No modifica tablas existentes, columnas de ventas, precios, comisiones, ni control de stock. La migración sigue usando `CREATE TABLE IF NOT EXISTS`; no incluye operaciones destructivas.

Incluye `catalog_commerce_pg_v2735162_test.go` con verificaciones del DDL de ambos motores.

## Alcance y validación

El hotfix modifica únicamente `catalog_commerce_v273516.go` y `VERSION.txt`, más un test nuevo. El instalador verifica la huella SHA-256 del archivo Go correspondiente al parche V27.35.16.1, crea respaldo y ejecuta `go test -count=1 ./...`, `go build ./...`, y `git diff --check`. Ante error revierte sus cambios.

La compilación integral se debe ejecutar localmente con Go 1.26. Tras el despliegue, comprobar en Render que no aparezca `catalog commerce schema` y que el servicio marque **Live**, además de verificar la carga/descarga privada de un comprobante de prueba y el aislamiento entre empresas.

La prueba del tipo SQL no reemplaza una prueba real contra PostgreSQL. Mantén una copia de seguridad de la base persistente antes de desplegar.

## Archivos publicados

- VERSION.txt
- catalog_commerce_v273516.go
- catalog_commerce_pg_v2735162_test.go
- RELEASE_V27_35_16_2_FIX_POSTGRES.md
