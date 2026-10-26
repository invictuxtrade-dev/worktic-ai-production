# V27.32 — Pruebas de aceptación requeridas antes del despliegue

**Estado:** todas las pruebas de esta sección requieren un entorno de staging totalmente separado, PostgreSQL/Redis de prueba, datos ficticios y proveedores simulados. **Nunca contra WorkticAI producción.**

## Dependencias reales

- Go 1.26 (`go version`), `go test ./...`, `go build ./...` en el código V27.32.
- PostgreSQL propio de QA con migraciones iniciales verificadas, Redis propio y al menos dos procesos app/worker con configuración **no productiva**.
- Datos ficticios: tenants A y B (usuarios, roles, contactos, productos, eventos, canales, planes Free/Personal/Negocio/Empresa); comparar con la matriz de derechos del servidor.
- Webhooks/API mocks (no usar Meta/WhatsApp real, Luma ni OpenAI en pruebas masivas). Probar un conjunto pequeño de solicitudes pagadas **solo** con presupuesto y aprobación.
- Capturas de logs, latencias p50/p95/p99, CPU/RAM, DB connections, facturas de sandbox, huellas de fixtures (no claves).

## Matriz de escenarios obligatorios

| ID | Prueba de aplicación real | Resultado para aprobar | Estado inicial |
|---|---|---|---|
| QA-01 | Login y roles admin/vendedor/tenant A/B | tenant A no lee ni escribe B; admin respeta privilegios | PENDIENTE |
| QA-02 | Free, Personal, Negocio, Empresa: cada módulo de matriz de derechos | GET/POST autorizados solo para módulos contratados | PENDIENTE |
| QA-03 | Alta concurrente de contactos/productos/agenda/canales | ninguna empresa supera cupo bajo 2 workers | PENDIENTE |
| QA-04 | Importaciones CRM, leads Meta y reactivación | cuota y aislamiento también por API/importaciones | PENDIENTE |
| QA-05 | Crédito mensual vs comprado, reserva, liquidación, reverso | saldo nunca negativo, sin dobles reservas | PENDIENTE |
| QA-06 | Luma/OpenAI (mock): éxito, fallo, timeout, callback repetido | gasto protegido, cargo idempotente y errores reconciliables | PENDIENTE |
| QA-07 | Copilot/Agentes/WhatsApp: IA y canales externos | precio/no precio y límites sin dobles cargos | PENDIENTE |
| QA-08 | Pago único hash usado en recarga + suscripción | sólo una aprobación, hasta con 2 workers | PENDIENTE |
| QA-09 | Mensualidad, anualidad, upgrades/downgrades, cortesías | no pierde vigencia, política contractual explícita | PENDIENTE |
| QA-10 | Worktic Partners: venta, reverso y compensación | comisiones y reversos reproducibles sin alterar red | PENDIENTE |
| QA-11 | Automatizaciones/WhatsApp/Inbox legado | no cruza tenants; webhooks íntegros y colas sin pérdida | PENDIENTE |
| QA-12 | Analytics y Ads: sincronización, reintentos, duplicados | atribución y costes separados del gasto publicitario | PENDIENTE |
| QA-13 | Pruebas de carga: 10/100/500/1000 tenants FICTICIOS | estable o capacidad/limitaciones documentadas; no simular solo HTTP GET | PENDIENTE |
| QA-14 | PostgreSQL/Redis resiliencia; reinicios y recuperación | reservas/pagos consistentes; sin pérdida/dobles envíos | PENDIENTE |
| QA-15 | Costeo por proveedor y tenant, FX USD/USDT y presupuesto Partners | conciliación verificable; costos desconocidos no cuentan como cero | PENDIENTE |
| QA-16 | Revisión UI escritorio/móvil y accesibilidad | funciones compradas utilizables; bloqueos entendibles | PENDIENTE |

## Prueba muy limitada de HTTP read-only en staging

Con una URL real **exclusivamente de staging**, nunca la de producción:

```powershell
$env:WORKTIC_QA_STAGING_ONLY = 'I_CONFIRM_ISOLATED_STAGING'
python tools/v2732/staging_load.py --base 'https://worktic-staging.onrender.com' --requests 50 --concurrency 5 --path /healthz --acknowledge
Remove-Item Env:WORKTIC_QA_STAGING_ONLY
```

Sustituye el hostname por el real. Esta prueba solo mide `/healthz`; **no prueba cantidad de clientes ni transacciones**. El ejecutable rechaza un dominio productivo y solicitudes de escritura.

## Criterio de salida V27.32 → V27.33

Los 16 escenarios con evidencia **real de staging** deben estar PASS. Cualquier fallo de tenant isolation, pago, credenciales, reserva o duplicación de envíos es crítico y **bloquea despliegue**. Los servicios externos con tarifa no verificada no se comercializan. La autorización final sigue siendo humana tras revisar todas las pruebas.
