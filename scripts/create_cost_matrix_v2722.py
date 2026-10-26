#!/usr/bin/env python3
"""Document one cost owner and measurement plan for each group of registered endpoints."""
import csv,json
from pathlib import Path
p=Path(__file__).resolve().parent.parent/'audits'
summary=json.loads((p/'RESUMEN_RUTAS_V2722.json').read_text())
manual={
'crm':('DB requests/contact/attachments','Worktic','contacts active/tenant; bytes','no'),
'catalog':('disk+transfer+image storage','Worktic','products/images/tenant','no'),
'agenda':('DB/notification transport','Worktic; external SMS if used','appointments + SMS sent','no'),
'inbox':('DB, messages; per-provider messaging','shared + customer variable','conversation/messages/retention','no'),
'channels':('external provider API, WhatsApp templates','customer pass-through OR Worktic variable','channel counts; provider conversations by geography','no'),
'marketing':('campaign requests, content, AI','Worktic + ads budget customer','campaigns/forms/generations','partial'),
'landings':('hosting, bandwidth and uploads','Worktic','page hits/leads/month and bytes','no'),
'social':('social upload, publish API, retry queue','Worktic + external account','posts/syncs/GB','no'),
'studio':('OpenAI image/text/speech; Luma video','Worktic','job cost estimate/credits + vendor invoice','estimated'),
'agents':('OpenAI token input/output + retrieval','Worktic','tokens/tenant/month','estimated'),
'automations':('jobs, events, external webhook, AI','Worktic + external customer accounts','runs, actions, retries, tokens','partial'),
'analytics':('events, Postgres storage, retention, export','Worktic','events ingested, reads, export bytes','no'),
'social_governance':('moderation/sync jobs/API + AI','Worktic','approvals, comments, AI tokens','no'),
'ads':('external Ads API sync + ad spend','ad spend customer; integration Worktic','API requests, data sync, dollars passed through','no'),
'meta_leads':('Graph API sync, storage, follow-ups','Worktic, outbound messaging variable','leads + API requests + retention','no'),
'groups':('messages/network/AI if used','Worktic or external channels','group sends, members, leads','no'),
'whatsapp_marketing':('Meta message pricing country/template','client pass-through recommended','messages successfully delivered by country/category','no'),
'legacy_channels':('legacy messaging/session maintenance','Worktic','connects, send calls, session GB','no'),
'legacy_inbox':('legacy message DB/network','Worktic','messages + GB + tenant scoping','no'),
'legacy_rules':('global rule table, old background engine','Worktic','rules, fires, recipients; TENANT ISSUE','no'),
'copilot':('OpenAI token billing','Worktic','input/output tokens, retries, cache','estimated'),
'agent_legacy':('OpenAI tokens legacy','Worktic','tokens and legacy tenant isolation','partial'),
'dashboard':('DB analytical queries/cache','Worktic','queries per request, latency and RAM','no'),
'partners':('commission ledger, payouts, autopilot AI','Worktic; commercial commission','approved revenue, partner payouts, AI tokens','partial'),
'admin':('database queries, customer operations','Worktic','requests and audit events','no'),
'billing':('processor fees, FX, refunds, USDT','Worktic; transaction fees','successful charges, refunds, chain fees','no'),
'commercial':('credits, package transactions, wallet ledger','Worktic','topups, holds, expired credits','estimated'),
'account':('password hashing, sessions, team email','Worktic','active seats, login/registration, invites','partial'),
'whatsapp':('WhatsApp Cloud templates/media','customer variable unless Worktic subsidizes','sent/delivered per category/region','no'),
'public_callbacks':('webhook requests, landing visits, bot callbacks','Worktic','requests/tenant, bandwidth, storage','no'),
'infra':('Render Go web, PG, Redis, disk, transfer','Worktic','CPU, RAM, DB size, egress','no'),
}
assert set(summary['modules']) == set(manual), set(summary['modules'])^set(manual)
rows=[]
for group,n in sorted(summary['modules'].items()):
    cost,payer,meter,observed=manual[group]
    rows.append({'group':group,'registered_endpoints':n,'cost_driver':cost,'who_pays_or_invoicing':payer,'measurement':meter,'provider_cost_verified':observed,'commercial_status':'BLOCKED UNTIL MEASURED' if observed=='no' else 'PILOT ESTIMATE ONLY'})
with (p/'MATRIZ_COSTOS_TODOS_MODULOS_V2722.csv').open('w',newline='',encoding='utf-8-sig') as f:
    w=csv.DictWriter(f,rows[0].keys());w.writeheader();w.writerows(rows)
print('Cost rows:',len(rows))
