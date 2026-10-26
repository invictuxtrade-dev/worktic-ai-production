# WorkticAI V27.20 — Creative Cost Guard & Cost Intelligence

## Provenance
Base is the *uploaded V27.19 ZIP* (`WorkticAI_V27.19_Studio_Video_Creator_Base_V27.18.zip`), not a legacy ChatGPT build. **Production commit parity has NOT been independently verified.** Inspect Render's active commit and `VERSION.txt` before deploying.

## Implemented, scope
- PostgreSQL/SQLite additive tables `creative_cost_budgets_v2720`, `creative_cost_ledger_v2720` (automatic at startup; NO SQLite→Postgres remigration).
- Per-tenant and global **monthly USD-cent budgets**; atomic reservation *before* Studio calls to OpenAI images, OpenAI text/copy, OpenAI speech and Luma video.
- Fail closed if budget absent/zero or DB error; protect against simultaneous requests and multiple app instances using SQL conditional updates.
- Conservative provider-cost estimate ledger with `held`, `estimated`, `released` statuses; authorized request amounts counted against the cap. External calls with ambiguous outcomes remain estimated, not refunded automatically; reconcile against provider invoices.
- Authenticated tenant-only budget/status and quote endpoints; superadmin-only Cost Intelligence panel in Administration Worktic showing monthly budget, holds and estimates by tenant/provider/action.
- Confirmation of *estimated* costs before each paid Studio action. Editing, timeline, local animation, local music and WebM export remain free of these new external-vendor reservations.
- Defaults are **zero** for all plan/global budgets: this deliberately blocks external Studio IA until an administrator sets safe limits in Render. Existing paid Studio features will stop generating while budgets remain unconfigured. This is an explicit launch safety choice.

## What is NOT yet implemented
- Commercial credit wallets, purchases, balances, refunds, real revenue/margins, accounting invoices, provider-invoice reconciliation automation, or spending control for other AI modules (Copilot, agents, Social Hub AI etc.). No database billing data or live OpenAI/Luma costs were accessible in the test environment.
- The numbers below are **conservative sample hypotheses**, not provider pricing guarantees. Markup, taxes, fees and Partner commissions are NOT accounted for in these budget-only tables.
- Historic V27.19 generations are not backfilled.

## Routes / API
- `GET /api/studio/cost/status` — authenticated tenant budget, remaining cents and price estimates; never a customer charge.
- `GET /api/studio/cost/quote?action=image|video|narration|copy|storyboard&duration=5|10&chars=123` — conservative estimate in cents.
- `GET /api/admin/cost-intelligence` — superadmin only, monthly budget rows + provider/action breakdown.

## Optional Render Environment (budget example)
Copy keys from `CREATIVE_COST_GUARD_V2720.env.example` into Render service Environment **after verifying model prices**. The amounts are whole **USD CENTS**. Example global 3000 = $30 cap per UTC calendar month. One company has its separate plan cap, not multiplied by number of users. A 720p 10-second video example reserves 150 cents. No value implies 0 and blocks vendor generation. Do NOT commit actual API keys.

## Safe deployment plan
1. Confirm current `main` and Render LIVE commit is the V27.19 source baseline, and backups exist for PostgreSQL and `/var/data`.
2. Deploy V27.20 to **staging with separate Postgres** and **no vendor API keys**. Verify automatic additive schemas and all existing views.
3. Run `gofmt -w creative_cost_guard_v2720.go studio_v2711.go studio_v2718.go studio_video_v2719.go main.go`, `go test ./...`, `go build ./...`, `node --check` modified JS and `git diff --check` using Go 1.26.
4. Verify absence of budget blocks a paid action with HTTP 429; no provider request is sent; local Image/Video Studio editing and export still work.
5. Set SMALL pilot caps in staging (e.g., $3 tenant, $5 global) and use a vendor with a limit **smaller than your account's hard spend limit**. Generate one controlled example and verify `held → estimated`, concurrency and ledger.
6. Confirm `/api/admin/cost-intelligence` is inaccessible to regular tenants. Verify a tenant cannot read another tenant's status.
7. Reconcile pilot estimates with real provider invoices and adjust quotes. Only then deploy to Render LIVE and enable explicit monthly caps. A code rollback does not roll back newly created database tables.
8. Before selling commercial credits: implement commercial balance ledger, payment confirmation, refund rules, real vendor-cost reconciliation and guard for the rest of the AI modules.

## Verification completed in generation environment
- `gofmt` and Go 1.23 **isolated compile/unit tests** of new guard logic: 3 groups PASS.
- `scripts/test_creative_guard_sqlite_v2720.py`: 12 simulated concurrent reservations, 4 permitted by cap PASS (SQLite; NOT a PostgreSQL/Go integration test).
- JavaScript syntax tests and static duplicate HTML-ID scan PASS.
- **Full Go 1.26 `go test ./...` NOT RUN:** sandbox only contains Go 1.23 and cannot download Go 1.26. Real provider calls, Render deployment and PostgreSQL not tested.

## Files changed
`VERSION.txt`, `main.go`, `studio_v2711.go`, `studio_v2718.go`, `studio_video_v2719.go`, `static/app.html`, `static/worktic-studio-v2711.js`, `static/worktic-studio-v2718.js`, `static/worktic-studio-v2719.js`. New: `creative_cost_guard_v2720.go`, `creative_cost_guard_v2720_test.go`, `static/creative-cost-v2720.js`, `static/creative-cost-v2720.css`, `scripts/test_creative_guard_sqlite_v2720.py`, this release doc, example env.
