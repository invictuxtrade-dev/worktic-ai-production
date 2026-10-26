# Hoja de ruta cerrada para candidato a producción WorkticAI

**No equivale a una fecha comprometida:** las fases son hitos de calidad y podrían requerir revisiones si las pruebas fallan. Actual: V27.29 (fase 1 de 5).

| Fase | Release | Criterio verificable de salida |
|---|---|---|
| 1 | V27.29 — Contratos y plan versionado | Snapshots de nuevos contratos, bloqueo de renovaciones con derechos cambiados, auditoría de clientes antiguos. Primera implementación preparada; pendiente probar en PostgreSQL. |
| 2 | V27.30 — Restricciones y rutas heredadas | 100% de rutas facturables identificadas con permisos server-side; migración segura de reglas WhatsApp históricas, importaciones e integraciones; cuotas por tenant sin bypass. |
| 3 | V27.31 — Costos, pagos y Partners | Consumo de servicios conciliado o riesgo presupuestado, validación de pasarelas y facturas, plan de comisiones/reversos, política de upgrades/prorrateo aprobada, margen mínimo comprobable por plan. |
| 4 | V27.32 — Pruebas E2E, carga y seguridad | Matrículas de test con empresas ficticias: Free/Personal/Negocio/Empresa, mensual/anual, pago, recarga, vencimiento, bloqueo, rollback, 10/100/1000 tenants simulados, estrés sobre staging y monitoreo. |
| 5 | V27.33 — Release Candidate / Staging | Go 1.26 `go test ./...` y `go build ./...` pasan; BD y servicios de staging equivalentes a Render; migraciones y rollback documentados, sin pérdida de datos, pruebas de regresión Partners, CRM, WhatsApp, Studio, Ads, Analytics. |

**Despliegue controlado a Render:** solo después del quinto hito y de la aprobación explícita del usuario. Se realiza en ventana controlada con copia de PostgreSQL y `/var/data`, plan de reversión, monitoreo y validación live. GitHub/Render permanecen intactos durante esta fase.

**Bloqueos que existen hoy:** compatibilidad de contratos históricos, fuentes heredadas globales, derechos extendidos no congelados (WhatsApp, Telegram, Messenger, agentes), pagos/reversiones mixtos, facturas reales de proveedores y pruebas multiinstancia con PostgreSQL. El primer hito no implica certificación de producción.
