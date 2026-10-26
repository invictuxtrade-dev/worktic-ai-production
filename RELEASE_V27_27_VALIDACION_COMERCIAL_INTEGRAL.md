# WorkticAI V27.27 — Validación Comercial Integral (STAGING ONLY)

**Base exacta:** WorkticAI V27.26 — Automatic Operation Metering. **Producción/GitHub/Render:** no modificados. **No activar ventas ni límites en producción.** Esta versión añade comprobaciones y una corrección de cuota IA; no certifica rentabilidad general.

## Lo que incorpora esta versión

1. **Corrección efectiva del límite de respuestas IA:** `commercial_core_ai_v2721.go` ahora utiliza el mínimo del cupo piloto y `billing_plans.max_ai_responses`, y considera el contador histórico de `usage_monthly` del mismo mes mediante el máximo (evita dobles conteos). Sin esto, un cliente podía recibir más respuestas que las contratadas.
2. **Preflight administrativo de solo lectura:** `GET /api/admin/commercial/v2727/preflight`, únicamente superadministrador. Verifica planes configurados, matrices Studio, paquetes, reglas heredadas, ciclos de pago sin verificar, operaciones sin precio y flags de staging. Expone `public_launch_approved=false`, aunque todas las consultas resulten correctas: aprobar ventas requiere evidencias reales.
3. **Interfaz integrada en Administración:** `static/commercial-validation-v2727.js/css`, muestra resultados por módulo, separados en comprobaciones, bloqueos y evidencias pendientes. No modifica datos ni llama a proveedores.
4. **Auditoría offline reproducible:** `python tools/v2727/validate.py` genera `audits/VALIDACION_COMERCIAL_V2727.json`, compara el inventario de 198 rutas y 31 grupos de costos, valida componentes críticos y construye escenarios comerciales con precios de referencia, anualidad y Partners 0/10/50%. NO calcula rentabilidad en USD sin tipo de conversión USDT/USD explícito. No conoce el gasto real de Render/Meta.
5. **Matriz de aceptación:** `audits/ACEPTACION_31_GRUPOS_V2727.csv`, un criterio de prueba pendiente por módulo/grupo y columna de evidencia; no cambia los permisos existentes.
6. **Simulación de concurrencia aislada:** `python tools/v2727/concurrency_sim.py` prueba un modelo SQLite independiente con 48 solicitudes sobre dos empresas y saldos de cinco; NO prueba los handlers reales ni PostgreSQL.

## Seguridad de esta fase

- No cambia las banderas por defecto; `WORKTIC_COMMERCIAL_ENABLE`, `WORKTIC_MODULE_GATES_V2722`, `WORKTIC_RESOURCE_GUARD_V2724`, `WORKTIC_CONSUMPTION_GUARD_V2723`, `WORKTIC_METER_V2726`, `WORKTIC_BILLING_V2725_SAFE_APPROVAL` conservan la desactivación por defecto.
- No se acreditan recargas ni ejecutan pagos automáticos nuevos.
- Sin claves ni secretos en los informes.
- No cambia comisiones de Worktic Partners.
- El endpoint de preflight no es un mecanismo que impida realmente que un operador active los toggles en un entorno distinto; funciona como señal y auditoría.

## Instrucciones para validación local (VS Code / PowerShell, no desplegar)

Desde la carpeta que contiene `go.mod`, en Go 1.26:

```powershell
go version
go test ./...
go build ./...
node --check static/commercial-validation-v2727.js
python tools/v2727/validate.py
python -m unittest discover -s tools/v2727 -p "test*.py" -v
python tools/v2727/concurrency_sim.py
```

El primer informe local está en `audits/VALIDACION_COMERCIAL_V2727.json`. Opcionalmente, para un **escenario estrictamente hipotético** con USDT=US$1:

```powershell
python tools/v2727/validate.py --usdt-usd-rate 1.0
```

Esta opción no consulta un tipo de cambio; **no demuestra un margen real**. Los valores de `billing_plans` en PostgreSQL pueden diferir de los precios de referencia codificados en la simulación.

## Aceptación E2E obligatoria antes de vender

1. Crear un **staging aislado**: DB PostgreSQL distinta, Redis diferente y disco separado, con backup verificable. NUNCA usar usuarios reales ni enviar campañas de test a contactos reales.
2. `go test ./...` y `go build ./...` con Go 1.26; resolver advertencias o errores antes de activar banderas. Este entorno de creación solo dispone de Go 1.23, por lo que no ejecutamos la compilación integral.
3. Acceso a 17 módulos bajo Free, Personal, Negocio y Empresa; comprobar servidor 403 y bypass por URL directa.
4. 20+ reservas en paralelo contra el MISMO tenant, y 20+ contra otro, en PostgreSQL multiinstancia. Confirmar ningún cupo negativo, ni saldo duplicado, ni aislamiento cruzado.
5. Probar mes nuevo, upgrades/downgrades, renovaciones anuales, descuentos, caducidad, reembolso, cancelación y usuario removido del equipo; cuotas compradas y del plan separadas.
6. Correlacionar token OpenAI, voz, imagen, video Luma y mensajes Meta con **facturación real**, incluidos timeouts, reintentos, fallos reembolsables y actualizaciones de estado tardías.
7. Meta/WhatsApp por país, categoría y ventana, publicidad Ads pagada por anunciante, y carga/persistencia de archivos: costo externo desconocido NO es costo cero.
8. Worktic Partners: comisión máxima 50%, Co-Sell, referidos, reversos, ajustes y pagos de retiros; no imputar automáticamente 50% de comisión a recargas creativas.
9. Conciliar precios comerciales del sitio, bases `billing_plans`, `commercial_rules_v2721` y los límites efectivos por endpoint; revisar textos SEO existentes sin reemplazarlos.
10. Medir CPU/RAM/DB/Redis/transferencia en staging con cargas de 10/100/500/1000 empresas simuladas sin proveedores pagados; documentar percentiles de latencia y costos atribuibles.
11. Actualizar la matriz de 31 grupos con pruebas, evidencia y persona revisora; hasta que no esté completa, NO marcar el lanzamiento como aprobado.

## Resultado obtenido aquí

- 14 pruebas Python de validación financiera/estructura pasadas.
- 3 pruebas aisladas Go del preflight pasadas con Go 1.23 (stubs, sin DB real).
- Simulación independiente SQLite de 48 solicitudes: 10 autorizadas, 38 rechazadas, sin doble cierre ni saldo negativo.
- Sintaxis de Go y JavaScript inspeccionada; no sustituye pruebas de compilación ni E2E.
- 198 rutas históricas identificadas, incluida 1 entrada dinámica que no es una ruta literal, y 31 grupos cost drivers inventariados. Inventario no implica cobertura real certificada.

**Estado:** CÓDIGO PILOTO / STAGING, NO APTO PARA VENTA PÚBLICA. La matriz financiera aún tiene rubros sin costos observados. Ningún informe de costes debe llamarse «factura real» salvo conciliación con evidencia externa.
