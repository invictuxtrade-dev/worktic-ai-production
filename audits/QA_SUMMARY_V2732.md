# WorkticAI V27.32 — informe de QA offline

Generado: 2026-10-23T21:16:04.587530+00:00
**APROBACIÓN DE PRODUCCIÓN: NO.**

| Comprobación | Estado | Evidencia |
|---|---|---|
| go_ast | PASS | exit=0; AST_VALID 70 Go files (NOT COMPILED) |
| javascript_syntax | PASS | 37 scripts checked; failures: [] |
| commercial_validation_tests | PASS | exit=0; validate.Tests.test_fx_warnings) ... ok test_historic_routes (test_validate.Tests.test_historic_routes) ... ok t |
| finance_preflight_tests | PASS | exit=0; test_duplicate_cost_group_rejected (test_financial_preflight.FinanceAuditV2731.test_duplicate_cost_group_rejecte |
| go_preflight_isolated | PASS | exit=0; === RUN TestProductionNeverConfusedWithStaging --- PASS: TestProductionNeverConfusedWithStaging (0.00s) === RUN  |
| sql_sim_credit_reserve | PASS | exit=0; { "test": "SQLite isolated transactional wallet model, NOT actual WorkticAI/PostgreSQL", "requested": 48, "allow |
| sql_sim_unique_payment | PASS | exit=0; OK: 48 solicitudes simultáneas, 1 aceptada, 47 rechazadas. Origen ganador: subscription Esta prueba utiliza una  |
| sql_sim_channel_quota | PASS | exit=0; Canales simulated: 48 concurrent attempts, 10 allowed, 38 refused; 5 per tenant |
| source_billing_static | PASS | exit=0; OK integración estática: 7 archivos de código, 18 recorridos auditables. NO certifica producción. |
| cost_catalog_preflight | PASS | exit=0; {"groups": 31, "not_verified": 31, "launch_approved": false, "output": "/mnt/data/worktic_v2733/worktic-ai-produ |
| critical_components_preserved | PASS | 7 source files verified. This is a structural check, not runtime authorization. |
| routes_unique | PASS | 212 literal routes, duplicate declarations=[]. Does not prove missing dynamic handlers are safe. |
| legacy_routes_preserved | PASS | 197 historic route declarations; missing=[] |
| private_env_absent | PASS | Known private configuration files: []; example files allowed |
| pilot_flags_off | PASS | {'WORKTIC_MODULE_GATES_V2722': '0', 'WORKTIC_CONTRACT_ENTITLEMENTS_V2729': '0'} |
| go126_full_build | BLOCKED | Requires Go >= 1.26 + project dependencies, `go test ./...` and `go build ./...` on developer machine / CI. |
| postgresql_multiinstance | BLOCKED | Requires real independent PostgreSQL, multiple app instances and tenant A/B session credentials. |
| redis_workers | BLOCKED | Requires staging Redis and background workers with external calls mocked. |
| payment_chain_and_refunds | BLOCKED | Requires test transactions with real chain proofs, yearly renewals, refunds and Partners reversals. |
| ai_provider_billing | BLOCKED | Requires controlled OpenAI/Luma/Meta cost reconciliation with verified source documents. |
| infrastructure_capacity | BLOCKED | Requires independent staging hosting and 10/100/500/1000 tenant DATA scenarios; HTTP GET load != tenants. |
| legacy_rules_tenant_migration | BLOCKED | Requires verified ownership of historical global rules and E2E inbox/webhook tests. |
| customer_acceptance_ui | BLOCKED | Requires full browser login and authorization journey for all plan roles and tenants. |

**Fallos ejecutados:** ninguno.
**Validaciones de staging pendientes:** go126_full_build, postgresql_multiinstance, redis_workers, payment_chain_and_refunds, ai_provider_billing, infrastructure_capacity, legacy_rules_tenant_migration, customer_acceptance_ui.

Los ensayos SQL SQLite son modelos independientes; no ejecutan handlers Go ni sustituyen PostgreSQL.
No hay pruebas de carga reales ni facturas verificadas en esta sesión.
