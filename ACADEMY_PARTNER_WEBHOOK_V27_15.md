# Academy → WorkticAI Partner Center · Activate Certified

## WorkticAI environment
Add one new secret in Render:

`PARTNER_ACADEMY_WEBHOOK_SECRET=<long-random-secret>`

Use the exact same secret in Academy.

## Endpoint

`POST https://workticai.com/webhooks/academy/partner-certification`

Header:

`X-Worktic-Signature: sha256=<HMAC_SHA256(raw_json, secret)>`

Payload:

```json
{
  "email": "student@example.com",
  "exam_code": "activate-certified",
  "status": "passed",
  "score": 92,
  "passing_score": 85,
  "certificate_id": "WAI-...",
  "certificate_url": "https://academy.workticai.com/?page=verify&code=...",
  "issued_at": "2026-10-15T18:00:00Z"
}
```

## Academy package
The companion `WorkticAI_Academy_V8_1_Partner_Activate_Bridge` package integrates the existing WorkticAI Operator course (`workticai-operador-configuracion-y-dominio`) with Partner Center.

When its final exam is passed and a certificate exists, the bridge:

1. inserts/updates a persistent `partner_sync_queue` event,
2. sends it immediately with HMAC,
3. keeps failed events locally,
4. retries with exponential backoff using `partner_bridge_worker.php`,
5. exposes `partner_bridge_admin.php` for an Academy administrator to inspect/retry the queue.

This avoids silently losing Activate certifications during temporary network outages.
