# Production change plan — HUMAN APPROVAL REQUIRED

This is a *runbook*, not an instruction to deploy now. Do not promote V27.33 without evidence.

## Baseline and non-destructive staging

1. In VS Code, verify `git remote -v`, `git fetch origin`, `git show origin/main:VERSION.txt` and the currently Live SHA shown by Render. **Never replace an unknown/newer source tree.** Confirm no uncommitted changes in the source of truth.
2. Create a separate `git worktree` from a confirmed production commit, apply V27.33 only after comparing each changed file and the source base, then run release manifest and full Go tests. Do not use `git push --force` or any database downgrade.
3. Provision standalone staging: new app service, new PostgreSQL and Redis, empty test data and fake integrations. No production credentials, no links to the production webhook callback or production WhatsApp numbers.
4. Use fake tenants/users A/B, all four plans, credits/wallets, channels and simulated payments. For security, test both expected access and denied access, including API-level bypass attempts.
5. Execute the 16 scenarios with a controlled load plan; publish evidence files and 2 independent reviews. The evidence gate only checks files and statuses; **it does not prove claims automatically.**

## Pre-deploy snapshot (do not overwrite)

- Preserve the exact running Git SHA and Render deployment ID, restore instructions, existing service environment-variable names (NOT secret values), current custom domains, webhooks and feature flag states.
- Create and verify restoration of a PostgreSQL backup on a separate test database. Do not assume clicking Rollback in Render reverses schema changes.
- Snapshot `/var/data`: uploads, WhatsApp sessions and application files. Do not delete or reinitialize current sessions.
- Compare schema changes for backwards compatibility; additive migrations may still break if new code changes behavior. If backwards compatibility is not verified, defer deployment.
- Temporarily disable only disruptive external actions during cutover **after** mapping which jobs can be retried idempotently, rather than stopping all workers blindly.

## Controlled cutover, only after explicit approval

- Keep candidate commercial flags OFF for the first code deploy; validate health, ready, authentication, CRM, WhatsApp, Partners, billing and Studio.
- Compare DB migration outcome with pre-deploy schema plan and actual customer data.
- Run 1-tenant internal canary, then staged activation by function only after cost and contract checks; never use public real customers for destructive tests.
- Watch 5xx, 429, latency, queue growth, duplicate messages, credits ledger, billing approvals and database locks; stop activation at unexpected increase or loss of authorization.

## Rollback triggers and procedure

**Triggers:** tenant data leak/cross-access, duplicate payout or message, lost channel credentials/sessions, failed subscription/credit entitlement, unexpected vendor spending, migration failure or data corruption.

1. Turn OFF new feature flags and suspend *new* cost-generating actions; reconcile in-flight jobs before retrying.
2. Revert **application code** using the recorded former Render deploy/commit. Never `git push --force`; use a safe revert/forward fix workflow.
3. **Do not automatically restore an older DB snapshot onto a live DB**, because transactions that occurred since backup would be lost. Prefer a compatible code rollback if migrations are additive and verified; otherwise activate a documented downtime/reconciliation recovery plan reviewed by DB and billing owners.
4. Restore `/var/data` only when the recovery procedure preserves subsequently received files/messages, and never duplicate webhook deliveries or paid media jobs.
5. Verify auth, inbox, plan rights, payments, Partners, CRM, Studio and credits after recovery; keep public monetization disabled until cause is resolved.

## Approval signoffs

Release engineer: ______  Date (UTC): ______
DB/operations owner: ______  Date (UTC): ______
Finance and Partners owner: ______  Date (UTC): ______
Security and tenant isolation owner: ______  Date (UTC): ______
Business owner: ______  Date (UTC): ______

**Status as distributed: BLOCKED — NO PRODUCTION APPROVAL.**
