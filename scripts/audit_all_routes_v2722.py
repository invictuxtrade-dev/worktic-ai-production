#!/usr/bin/env python3
"""Static, read-only route inventory for staged V27.22. No API calls or writes to DB."""
from pathlib import Path
import csv,re,json,collections
base=Path(__file__).resolve().parent.parent
src=(base/'main.go').read_text(encoding='utf-8')
routes=[]
for m in re.finditer(r'\bmux\.HandleFunc\(\s*("[^"\n]+"|[A-Za-z][A-Za-z0-9_]*)\s*,\s*app\.([A-Za-z0-9_]+)', src):
    literal=m.group(1)
    route=literal.strip('"') if literal.startswith('"') else '$'+literal
    routes.append((route,m.group(2),src[:m.start()].count('\n')+1))

def classify(p):
    if p.startswith('/api/ads/v25/'):return 'ads','plan-gate-opt-in','Ads provider quota + sync costs TBD'
    if p.startswith('/api/analytics/v24/'):return 'analytics','plan-gate-opt-in','Analytics DB/retention costs TBD'
    if p.startswith(('/api/social/governance','/api/social/approvals','/api/social/repurpose','/api/social/community')):return 'social_governance','plan-gate-opt-in','Community calls + concurrency TBD'
    if p.startswith('/api/social/'):return 'social','plan-gate-opt-in','API publish quotas TBD'
    if p.startswith('/api/automations/'):return 'automations','plan-gate-opt-in + workflow create limit','Fanout and concurrency TBD'
    if p.startswith('/api/agents/'):return 'agents','plan-gate-opt-in + core AI quota','Agents cross-route fallback TBD'
    if p.startswith('/api/whatsapp-marketing/'):return 'whatsapp_marketing','plan-gate-opt-in','Meta message fees/country TBD'
    if p.startswith('/api/groups'):return 'groups','plan-gate-opt-in','Group sends must be costed'
    if p.startswith('/api/marketing/meta-leads/'):return 'meta_leads','plan-gate-opt-in','Lead APIs/retention TBD'
    if p.startswith('/api/marketing/landings'):return 'landings','plan-gate-opt-in','Bandwidth/storage TBD'
    if p.startswith('/api/marketing/'):return 'marketing','plan-gate-opt-in + historic limits','Campaign creation quota/AI TBD'
    if p.startswith('/api/crm/'):return 'crm','plan-gate-opt-in + historic count','Concurrent inserts TBD'
    if p in ('/api/products','/api/catalog/upload'):return 'catalog','plan-gate-opt-in + historic count','Uploads/storage TBD'
    if p.startswith('/api/agenda/') or p=='/api/appointments':return 'agenda','plan-gate-opt-in','Appointments rate cap TBD'
    if p.startswith('/api/inbox/'):return 'inbox','plan-gate-opt-in','Message retention TBD'
    if p.startswith('/api/studio/'):return 'studio','plan-gate-opt-in + creative credits','OpenAI/Luma cost reconciliation TBD'
    if p.startswith(('/api/channels/','/api/whatsapp/business/','/api/whatsapp/templates/')):return 'channels','plan-gate-opt-in + historic count','Meta fees/templates TBD'
    if p.startswith('/api/admin/'):return 'admin','role-check-in-handler','Review each admin mutation'
    if p.startswith('/api/partners/'):return 'partners','partner-program-owned','Commission liabilities + payments TBD'
    if p.startswith('/api/billing/') or p in ('/api/plans',):return 'billing','payments/subscription-module','Payment confirmation + discounts TBD'
    if p.startswith('/api/commercial/'):return 'commercial','wallet/entitlements-module','Price + allotment validation TBD'
    if p.startswith('/api/auth/') or p.startswith('/api/account/') or p.startswith('/api/team/'):return 'account','session/roles','Seats + auth cost TBD'
    if p.startswith('/api/whatsapp/'):return 'whatsapp','historic-flow','Legacy tenant+fees TBD'
    if p.startswith('/api/agent'):return 'agent_legacy','historic-flow','Core quota coverage TBD'
    if p in ('/api/connect','/api/disconnect','/api/logout','/api/qr','/api/telegram','/api/messenger'):return 'legacy_channels','historic-flow','Legacy tenant and vendor billable calls require validation'
    if p in ('/api/conversations','/api/messages','/api/read','/api/contact','/api/send'):return 'legacy_inbox','historic-flow','Legacy tenancy, message metering and security review'
    if p == '/api/rules':return 'legacy_rules','GLOBAL legacy table; blocking finding','Not tenant-scoped: do not advertise per-company rule quotas'
    if p.startswith('/api/copilot/v27'):return 'copilot','core AI quota V27.21','Actual token cost/model/retries TBD'
    if p in ('/api/dashboard','/api/dashboard/v27','/api/dashboard/v27/activity'):return 'dashboard','session/tenant','Dashboard response shape & DB cost TBD'
    if p.startswith(('/webhooks/','/f/','/l/','/r/','/t/','/uploads/')) or p.startswith('$'):return 'public_callbacks','public/webhook-specific','Signatures/tenant isolation TBD'
    if p in ('/','/healthz','/readyz','/api/status'):return 'infra','health/static','Render/DB/Redis allocation TBD'
    return 'UNMAPPED','NEEDS MANUAL REVIEW','Unclassified: no automatic guarantee'

rows=[]
for p,h,lineno in routes:
    category,state,note=classify(p)
    rows.append({'path':p,'handler':h,'source':'main.go:'+str(lineno),'module':category,'gate_status':state,'risk_or_cost_unverified':note})
if len(rows)!=len(set(r['path'] for r in rows)):
    raise SystemExit('duplicate path registration detected')
if len(rows)<100: raise SystemExit('route scan unexpectedly incomplete')
audit=base/'audits';audit.mkdir(exist_ok=True)
with (audit/'RUTAS_COMPLETAS_V2722.csv').open('w',newline='',encoding='utf-8-sig') as f:
    w=csv.DictWriter(f,fieldnames=rows[0].keys());w.writeheader();w.writerows(rows)
c=collections.Counter(r['module'] for r in rows)
summary={'source':'main.go','route_count':len(rows),'module_count':len(c),'modules':dict(sorted(c.items())),'unmapped':[r['path'] for r in rows if r['module']=='UNMAPPED'],'scope':'Static endpoint coverage, not security or cost validation','gate_opt_in':'WORKTIC_COMMERCIAL_ENABLE=1 + WORKTIC_MODULE_GATES_V2722=1'}
(audit/'RESUMEN_RUTAS_V2722.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2),encoding='utf-8')
print(json.dumps({'routes':len(rows),'modules':len(c),'unmapped':summary['unmapped']},ensure_ascii=False,indent=2))
