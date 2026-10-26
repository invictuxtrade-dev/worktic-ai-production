# WorkticAI V27.33 — Candidate release / mandatory staging sign-off

**Source of truth:** WorkticAI V27.32 staging QA ZIP. No GitHub, Render, production database, provider API or live payment modified.

## Delivered in this release

- Full cumulative project and release source-integrity manifest (`RELEASE_MANIFEST_V2733.json`).
- Hardened read-only staging loader, including blocking any `*.workticai.com` host and preventing HTTP redirects from reaching production; added corresponding regression tests.
- Fail-closed acceptance gate (`tools/v2733/release_gate.py`) with 16 mandatory test records QA-01 to QA-16, per-test evidence files and SHA256 validation, review dated within 30 days and independent technical/commercial approval. **The tool never grants production approval**, even if all evidence is present; final go/no-go belongs to a human.
- Manifest checker (`tools/v2733/verify_manifest.py`) and PowerShell local preflight (`tools/v2733/preflight.ps1`) for VS Code. The script does not push or deploy.
- Commercial switches explicitly disabled in `.env.render.example` for safe defaults. It is an example file only, not the Render runtime environment.
- Staging verification and rollback runbook in `tools/v2733/GO_NO_GO_AND_ROLLBACK.md`.

**This is a candidate only. It is NOT certified for production.** V27.32 reported 15 passed offline checks and 8 blocked staging verification categories. There was no PostgreSQL/Redis multi-instance testing, end-to-end payment processing, Luma/OpenAI/Meta billing reconciliation, 10-1000 tenant simulation or Render deployment in this environment.

## Validate in Visual Studio Code, PowerShell (local copy)

```powershell
Set-Location 'C:\YOUR\STAGING\WORKTIC_V27.33'
powershell -ExecutionPolicy Bypass -File .\tools\v2733\preflight.ps1
```

The command must show Go 1.26.x, run static checks plus `go test ./...` and `go build ./...`, verify the manifest and report any missing staging evidence. The script never creates a production-ready marker.

To run only the offline checks on any platform:

```text
python tools/v2732/qa_runner.py
python -m unittest discover -s tools/v2732 -p 'test_*.py' -v
python -m unittest discover -s tools/v2733 -p 'test_*.py' -v
python tools/v2733/verify_manifest.py
python tools/v2733/release_gate.py
```

Expected **before actual staging**: `release_gate.py` exits nonzero and says evidence missing. This is correct.

## Staging evidence required

1. Deploy to an **isolated** staging service and new PostgreSQL/Redis resources; no production secrets/data; mock external providers for volume testing.
2. Run QA-01 through QA-16 from `tools/v2732/STAGING_ACCEPTANCE.md` and capture redacted logs/screenshots/report files.
3. Place them under `audits/evidence_v2733/`, duplicate `tools/v2733/STAGING_EVIDENCE_TEMPLATE.json` as `audits/STAGING_EVIDENCE_V2733.json`, mark each PASS only after observing passing behavior, record path and actual SHA256, UTC date, the SHA256 of `RELEASE_MANIFEST_V2733.json` in `source_manifest_sha256`, and two reviewers.
4. Execute the release gate, review any blockers. Even a green gate is only **ready for human go/no-go**.
5. Obtain backups for PostgreSQL and `/var/data`, compare running production commit/source-of-truth with candidate lineage and plan cutover/rollback. Never re-run old SQLite-to-PostgreSQL migration.

## Critical known blockers (not corrected by documentation)

- Real multi-tenant E2E authorization and legacy rule ownership migration.
- Confirmed test transactions/renewals/refunds/reversal and Worktic Partners compensation.
- Token/operation costs reconciled to provider billing; infrastructure cost per tenant and margin approval.
- Go 1.26 compilation and integration tests under PostgreSQL/Redis with multiple workers.
- Channel webhook/inbox delivery preservation, Meta/WhatsApp billing per destination/category and dynamic tariff checks.
- Load/availability of real workflows under 10/100/500/1000 fictitious tenants: health checks alone do NOT prove this.

**Commercial activation:** all flags remain OFF. No new paid generation or commercial offer is approved by this release. Never copy/example environment values over secrets already set in Render.
