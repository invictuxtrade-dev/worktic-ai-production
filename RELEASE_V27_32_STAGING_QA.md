# WorkticAI V27.32 — Fase 4/5, QA de seguridad, integración y capacidad

**Base exacta:** ZIP V27.31. **Ningún servicio GitHub/Render modificado.** Esta entrega incorpora scripts de validación y documentación de staging, no activa banderas ni modifica los módulos de negocio.

## Qué entrega realmente

- `tools/v2732/qa_runner.py`: ejecuta validación offline de sintaxis Go AST, JavaScript, pruebas de configuración/comercio, y modelos concurrentes SQLite; genera `audits/QA_REPORT_V2732.json` y `audits/QA_SUMMARY_V2732.md`.
- `tools/v2732/staging_load.py`: ensayo de carga **solo GET** sobre `/healthz` o `/readyz` en staging, con autorización expresa, tope 300 peticiones y concurrencia máxima 10; rechaza `workticai.com` y hostnames productivos. No simula clientes reales.
- `tools/v2732/test_staging_load.py`: pruebas de autocontrol del script para que no toque hosts de producción sin querer.
- `tools/v2732/STAGING_ACCEPTANCE.md`: 16 pruebas de seguridad, facturación, módulos, permisos, multiempresa, PostgreSQL y carga con datos ficticios. Estado pendiente hasta obtener evidencia.
- `VERSION.txt`: marca la base V27.32 QA.

**Comprobaciones que NO hemos podido ejecutar aquí:** `go test ./...`, `go build ./...` por requerir Go 1.26; PostgreSQL/Redis aislados, 2+ instancias de app, flujos UI autenticados, pasarelas y proveedores de pago. Esta limitación no se disfraza como `PASS`.

## Comandos en la terminal PowerShell de VS Code (sobre copia aislada de V27.32)

```powershell
python tools/v2732/qa_runner.py
python -m unittest discover -s tools/v2732 -p 'test_*.py' -v
go version
go test ./...
go build ./...
```

No activar banderas ni ejecutar carga contra producción. Requiere entorno de staging aislado antes de V27.33. La V27.33 estará destinada a **corregir** hallazgos y preparar recuperación y despliegue sin destruir la versión vigente; no es todavía aprobación comercial automática.
