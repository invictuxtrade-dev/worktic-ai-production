# V27.15 Partner production checklist

## Before deployment
- Verify branch is `main` and repository is clean.
- Keep `BASE_URL=https://workticai.com`.
- Do not change `CHANNEL_ENCRYPTION_KEY`.
- Add `PARTNER_ACADEMY_WEBHOOK_SECRET` only when Academy bridge is ready, using the same value in both systems.
- Run Go 1.26 formatting/tests locally.
- Run JavaScript syntax checks.
- Run `git diff --check`.

## After Render is Live
- Login with a paid account and activate Partner Center.
- Verify referral URL `/r/WT...` and registration attribution.
- Verify network list/tree and Momentum.
- Create a Co-Sell and accept from both Partner accounts.
- Approve a test billing payment and verify direct/Co-Sell/network ledger rows.
- Trigger a refund in test data and verify reversal/idempotency.
- Verify payout request → approve → processing → paid with reference.
- Verify Academy webhook using a test certificate.
- Publish a Human Sales job, approve it, apply with certified Partner, accept, report sale, confirm and fund/approve.
- Generate Partner Autopilot plan and create Growth/Landing drafts.
- Confirm public `/` and `/partners.html` remain unchanged.

## Never run for this release
`/app/worktic-migrate`
