#!/usr/bin/env python3
"""V27.27 offline commercial QA. NO network, writes, payments or vendor calls.

Usage: python tools/v2727/validate.py --output audits/V2727_VALIDATION_REPORT.json
Optional --usdt-usd-rate 1.00 for an explicitly HYPOTHETICAL currency scenario.
"""
import argparse,csv,io,json,pathlib,re,sys
from decimal import Decimal, InvalidOperation
from datetime import datetime,timezone
ROOT=pathlib.Path(__file__).resolve().parents[2]
D=Decimal
PLANS={
 'free':{'usdt':D('0'),'studio':D('0'),'core':D('0'),'users':1},
 'personal':{'usdt':D('25'),'studio':D('.75'),'core':D('1.50'),'users':1},
 'business':{'usdt':D('75'),'studio':D('3'),'core':D('5'),'users':5},
 'enterprise':{'usdt':D('150'),'studio':D('7.50'),'core':D('15'),'users':15},
}
FEATURES={
 'free':{'crm','catalog','agenda','inbox','channels','marketing','landings','social','studio'},
 'personal':{'crm','catalog','agenda','inbox','channels','marketing','landings','social','studio','agents','automations','analytics'},
 'business':{'crm','catalog','agenda','inbox','channels','marketing','landings','social','studio','agents','automations','analytics','social_governance','ads','meta_leads','groups','whatsapp_marketing'},
 'enterprise':{'crm','catalog','agenda','inbox','channels','marketing','landings','social','studio','agents','automations','analytics','social_governance','ads','meta_leads','groups','whatsapp_marketing'},
}
PROVIDER_ACTIONS=[('image',4,8),('copy',2,5),('video5',15,36),('video10',45,99),('voice1000',5,10)]
def money(x):return str(x.quantize(D('.01')))
def scenario(plan, annual_pct, partners_pct, rate):
 p=PLANS[plan]; annual=D(str(annual_pct));partners=D(str(partners_pct))
 if not (D('0')<=annual<=D('80') and D('0')<=partners<=D('50')):raise ValueError('discount/partner percent out of bounds')
 income=p['usdt']*(1-annual/100)
 data={'plan':plan,'gross_monthly_equivalent_usdt':money(income), 'partners_usdt':money(income*partners/100),'studio_budget_usd':money(p['studio']),'core_ai_budget_usd':money(p['core']), 'fx_rate_usd_per_usdt':str(rate) if rate is not None else None}
 if rate is None:
  data.update({'remaining_usd':None,'contribution_usd':None,'status':'INCOMPLETE: FX, processing, infrastructure, messages, support and tax unverified'})
 else:
  if rate<=0:raise ValueError('positive FX rate required')
  after=(income*(1-partners/100)*rate)-(p['studio']+p['core'])
  data.update({'remaining_usd_before_other_costs':money(after),'status':'SIMULATION: not verified margin; excludes other expenses'})
 return data

def audit(project,rate=None):
 issues=[];checks=[]
 def check(key, condition, detail, blocker=True):
  status='pass' if condition else ('block' if blocker else 'manual')
  checks.append({'key':key,'status':status,'detail':detail})
  if status=='block':issues.append(key)
 # Structural/source tests, not runtime E2E certifications.
 source=(project/'commercial_core_ai_v2721.go').read_text()
 check('core_reserve_db_cap', 'commercialCoreCapForPlan(plan, billingCap)' in source and 'SELECT max_ai_responses FROM billing_plans' in source,'Reservation checks both plan billing cap and pilot cap')
 source=(project/'commercial_validation_v2727.go').read_text()
 check('read_only_admin_preflight', 'requireSuperadmin(w, r)' in source and 'public_launch_approved' in source and 'false' in source,'Server exposes superadmin-only read-only preflight, public sale never asserted')
 source=(project/'main.go').read_text()
 check('preflight_route', '"/api/admin/commercial/v2727/preflight"' in source,'Endpoint registered')
 check('finance_meter_route', '"/api/admin/finance/v2726/metering"' in source,'V27.26 meter route preserved')
 check('partners_engine_preserved',(project/'partners_v2715.go').exists() and (project/'partners_v2716.go').exists(),'Worktic Partners code retained')
 check('commercial_wallet',(project/'commercial_credits_v2721.go').exists(),'Credit ledger retained')
 check('guards_exist',all((project/x).exists() for x in ('consumption_guard_v2723.go','resource_guard_v2724.go','creative_cost_guard_v2720.go')),'Guards retained')
 modules=list(csv.DictReader(io.StringIO((project/'audits/RUTAS_COMPLETAS_V2722.csv').read_text(encoding='utf-8-sig'))))
 costgroups=list(csv.DictReader(io.StringIO((project/'audits/MATRIZ_COSTOS_TODOS_MODULOS_V2722.csv').read_text(encoding='utf-8-sig'))))
 check('routes_inventory',len(modules)>=198,'Inventory from V27.22 includes at least 198 routes')
 check('cost_groups',len(costgroups)>=31,'31+ cost drivers inventoried, NOT fully verified')
 current_routes=set(re.findall(r'mux\.HandleFunc\("([^"]+)"',source))
 original_routes=set(r['path'] for r in modules if r.get('path','').startswith('/')) # dynamic constant entries are not literal routes
 missing=sorted(original_routes-current_routes)
 check('routing_regression',not missing,'Historic registered routes still present in main.go; missing: '+', '.join(missing[:12]))
 # Public promises remain blocked pending provider/DB verification.
 checks.extend([
 {'key':'postgresql_e2e','status':'manual','detail':'Test actual PG isolation, concurrent quotas, migration, repayment and cancellation'},
 {'key':'provider_invoices','status':'manual','detail':'OpenAI/Luma/Meta/Render invoice and actual cost alignment'},
 {'key':'payments_partners','status':'manual','detail':'USDT chain verification, coupons, renewals, Partner payouts/reversals'},
 {'key':'legacy_global_rules','status':'manual','detail':'Audit remaining global worktic_auto_rules routes without guessing tenant owners'},
 {'key':'public_sale','status':'block','detail':'No approval for public commercialization without staging evidence and measured operational costs'},
 ])
 issues.append('public_sale')
 prices=[]
 for action,credits,cents in PROVIDER_ACTIONS:
  max_safe=2*int(cents)<=5*int(credits)
  check('credit_ceiling_'+action,max_safe,action+' estimated provider cents '+str(cents)+' / '+str(credits)+' credits respects <= US$0.025/credit')
  prices.append({'action':action,'credits':credits,'reserved_cost_cents':cents,'pass':max_safe})
 scenarios=[scenario(plan,annual,commission,rate) for plan in PLANS for annual in (0,20) for commission in (0,10,50)]
 for plan,p in PLANS.items():
  if plan=='free' and (p['core']>0 or p['studio']>0):check('free_zero',False,'Free must have no external included budget')
  check('included_cost_'+plan,p['studio']+p['core']<p['usdt'] or plan=='free','Budget Studio+Core does not exceed monthly price in USDT under numeric comparison only; FX/other costs unknown')
 return {'version':'V27.27','generated_utc':datetime.now(timezone.utc).isoformat(),'data_classification':'Offline code inspection and scenarios; not measured bills or actual customer data','routes_in_prior_inventory':len(modules),'cost_groups_in_prior_inventory':len(costgroups),'newly_missing_routes':missing,'checks':checks,'hard_blockers':sorted(set(issues)), 'credit_quotes':prices, 'scenarios':scenarios,'public_launch_approved':False,'network_access':False}

def main():
 ap=argparse.ArgumentParser();ap.add_argument('--output',default='audits/VALIDACION_COMERCIAL_V2727.json');ap.add_argument('--usdt-usd-rate',type=str,help='Hypothetical and manual. Omit for no cross-currency comparison.');a=ap.parse_args()
 try: rate=D(a.usdt_usd_rate) if a.usdt_usd_rate is not None else None
 except InvalidOperation:ap.error('FX rate invalid')
 result=audit(ROOT,rate)
 output=ROOT/a.output
 if not output.resolve().is_relative_to(ROOT.resolve()):ap.error('output must be inside project')
 output.parent.mkdir(parents=True,exist_ok=True)
 output.write_text(json.dumps(result,ensure_ascii=False,indent=2),encoding='utf-8')
 print(json.dumps({'report':str(output),'checks':len(result['checks']),'failed_code_checks':[x['key'] for x in result['checks'] if x['status']=='block' and x['key']!='public_sale'],'public_launch_approved':False},ensure_ascii=False))
 return 1 if any(x['status']=='block' and x['key']!='public_sale' for x in result['checks']) else 0
if __name__=='__main__':sys.exit(main())
