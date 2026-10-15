# WorkticAI V27.15 — Partner Business Backoffice

## Architecture
Partner Center is an independent business domain inside the same WorkticAI account and session. It reuses CRM, Billing, Growth, Landing Pages, OpenAI/Copilot, Analytics and Academy signals without turning the CRM into an MLM/backoffice engine and without creating a second CRM.

The public Worktic Partners landing remains informational. The authenticated Partner Center is the operational backoffice.

## Partner backoffice
The Partner user can access:

- **Inicio**: Partner code, smart attributed link, current Momentum rank, certification, earnings, balances, active clients and community metrics.
- **Mi Red**: list/tree view through three levels, rank, status, adoption and Activate status.
- **Mis Clientes**: attributed customers, plan, activity/adoption and shortcut to the existing CRM.
- **Comisiones**: auditable ledger of direct, Co-Sell, N2, N3, adoption, renewal, mentorship, Human Sales and reversals.
- **Momentum**: Spark → Pulse → Orbit → Apex → Infinity, progress and next-rank requirements.
- **Co-Sell**: proposal, split, bilateral acceptance/rejection/cancellation and final commission split.
- **Activate Certified**: Academy certification state and credential history.
- **Oportunidades**: Human Sales marketplace for certified specialists and businesses.
- **Payouts**: payment profile, withdrawal requests, status and reference/hash history.
- **Partner Autopilot**: AI commercial plan, attributed landing and Growth campaign drafts.
- **Configuración**: Partner identity and commercial information.

## Momentum rules
| Rank | Direct active clients | Adopted clients | Direct | N2 | N3 |
|---|---:|---:|---:|---:|---:|
| Spark | 0 | 0 | 25% | 0% | 0% |
| Pulse | 3 | 2 | 28% | 0% | 0% |
| Orbit | 8 | 6 | 30% | 3% | 0% |
| Apex | 18 | 14 | 32% | 4% | 2% |
| Infinity | 35 | 28 | 34% | 5% | 3% |

Rank downgrade protection is 90 days by default.

## Commission engine
The ledger does not store only a balance. Each commission is an immutable/auditable movement with partner, customer/payment/reference, type, level, eligible base, percentage, amount, currency, status, availability date, dedupe key and metadata.

Default bonuses:
- Adoption: 4% after approved adoption and Impact Score >= 70.
- Renewal: 2% only after a prior approved payment for the same plan.
- Mentorship: 2% for the direct sponsor when the mentor meets the configured minimum rank (Apex by default).

Annual WorkticAI subscriptions are recognized in 12 monthly commission installments rather than releasing the full annual commission on day one.

## Co-Sell
A Partner can propose a Co-Sell to another active paid Partner as **referidor** or **closer**. The initiator accepts their own side when creating the agreement; the other party must explicitly accept. Only when both parties accept does the agreement become active.

The direct commission earned by the origin Partner is divided using the agreed percentages. The default is 70/30 and the allowed range protects against invalid splits.

## Activate Certified / Academy
WorkticAI exposes:

`POST /webhooks/academy/partner-certification`

The Academy event is signed with HMAC SHA-256 using `PARTNER_ACADEMY_WEBHOOK_SECRET`. WorkticAI matches the Academy student by email, validates the passing status/score, records the certificate and automatically sets `activate_status=certified` for eligible Partners.

A companion Academy V8.1 bridge package is produced together with this release. The bridge has a persistent retry queue so a temporary API outage does not lose a certificate event.

## Human Sales Network
Businesses can publish commercial opportunities. Flow:

1. Business creates an opportunity and defines the commercial commission percentage on generated sales.
2. Superadmin reviews/opens the opportunity.
3. Activate Certified specialists apply.
4. The business accepts candidates up to the number of available slots.
5. Specialist reports an actual sale.
6. Business confirms or rejects the reported sale.
7. Superadmin registers the settlement/funding reference and approves it.
8. WorkticAI creates the specialist commission after the configured hold.

The default split is **80% specialist / 20% WorkticAI of the commercial commission pool**, never of the gross sale.

Example: $10,000 sale × 10% business commission = $1,000 pool → $800 specialist + $200 WorkticAI.

## Payouts
Partner defines a payout destination/method and requests an amount from available balance. Workflow:

`requested → approved → processing → paid`

Also supported: `cancelled`, `rejected`, `needs_review`.

A payout can only be marked paid after the administrator records a hash/reference/comprobante. WorkticAI then records a negative ledger debit so the accounting remains auditable.

No wallet private keys or bank credentials are stored by the Partner engine. The external transfer is executed by the administrator/payment provider and the platform records/reconciles it.

## Refunds and chargebacks
Approved billing payments can be marked as `refunded` or `chargeback` by Superadmin. The Partner reversal engine is idempotent and re-processable. Original positive commission rows are reversed; if payouts already reduced the account, the remaining balance can become negative debt, which prevents new payouts until recovered.

On startup WorkticAI reconciles both approved sales and refund/chargeback reversals, so transient failures can recover automatically.

## Partner Autopilot
Uses the existing OpenAI key and WorkticAI modules. The Partner gives niche, goal, target clients, offer and channel. Autopilot creates a structured commercial plan and can activate it into:

- Growth campaign draft,
- up to 5 content drafts,
- attributed Partner landing page,
- referral CTA using the Partner smart link.

It does not auto-publish ads/messages or bypass consent/governance.

## Admin
`Administración Worktic → Worktic Partners` includes:

- global KPIs,
- Partner profiles/status/ranks,
- adoption/Impact Score,
- Co-Sell,
- Human Sales jobs/sales,
- payout workflow,
- certifications,
- configurable Partner settings,
- rank rules.

## Database safety
All Partner V27.14/V27.15 tables and additive columns initialize from the application startup using `CREATE TABLE IF NOT EXISTS` / additive `ALTER TABLE` compatibility. Do not run the historical SQLite→PostgreSQL migrator.
