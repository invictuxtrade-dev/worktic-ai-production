#!/usr/bin/env python3
"""WorkticAI V27.32 -- deterministic offline quality gate, no external calls.
Creates evidence JSON/Markdown. Passing offline checks is NOT approval for launch.
Usage: python tools/v2732/qa_runner.py
"""
from __future__ import annotations
import csv, datetime, hashlib, json, os, pathlib, re, shutil, subprocess, sys, time
from collections import Counter

ROOT=pathlib.Path(__file__).resolve().parents[2]
DEST=ROOT/'audits'/'QA_REPORT_V2732.json'
ISO=datetime.datetime.now(datetime.timezone.utc).isoformat()
TESTS=[]


def record(key, result, detail, *, scope='offline', runtime_s=0.0):
    TESTS.append(dict(id=key, status=result, scope=scope, detail=detail[:950], elapsed_seconds=round(runtime_s,3)))


def run(key, cmd, timeout=60, *, optional=False):
    started=time.monotonic()
    if shutil.which(cmd[0]) is None:
        record(key,'BLOCKED' if not optional else 'SKIPPED', f'Tool not installed: {cmd[0]}',runtime_s=time.monotonic()-started)
        return
    try:
        p=subprocess.run(cmd,cwd=ROOT,capture_output=True,text=True,timeout=timeout, env={**os.environ,'GO111MODULE':'off'} if key=='go_ast' else None)
        output=' '.join((p.stdout+' '+p.stderr).strip().split())
        record(key,'PASS' if p.returncode==0 else 'FAIL',f'exit={p.returncode}; {output[-650:]}',runtime_s=time.monotonic()-started)
    except subprocess.TimeoutExpired:
        record(key,'FAIL',f'Timed out after {timeout}s',runtime_s=time.monotonic()-started)


def source_check(key, conditions):
    missing=[]
    for path,needles in conditions.items():
        p=ROOT/path
        if not p.is_file():missing.append(f'{path}: file missing');continue
        content=p.read_text(encoding='utf-8')
        for needle in needles:
            if needle not in content:missing.append(f'{path}: missing {needle}')
    record(key,'FAIL' if missing else 'PASS','; '.join(missing[:8]) if missing else f'{len(conditions)} source files verified. This is a structural check, not runtime authorization.')


def main():
    # Runtime module checks using existing test infrastructure, not mocks presented as E2E.
    run('go_ast',['go','run','tools/v2732/go_syntax_scan.go','--',str(ROOT)],timeout=90)
    js=sorted((ROOT/'static').rglob('*.js'))
    if shutil.which('node'):
        errors=[]
        for p in js:
            proc=subprocess.run(['node','--check',str(p)],capture_output=True,text=True,timeout=15)
            if proc.returncode:errors.append(f'{p.relative_to(ROOT)}: {proc.stderr[:140]}')
        record('javascript_syntax','PASS' if not errors and len(js)>25 else 'FAIL',f'{len(js)} scripts checked; failures: {errors[:5]}')
    else:record('javascript_syntax','BLOCKED','Node.js required to check JS syntax')

    run('commercial_validation_tests',[sys.executable,'-m','unittest','discover','-s','tools/v2727','-p','test_validate.py','-v'],timeout=90)
    run('finance_preflight_tests',[sys.executable,'-m','unittest','discover','-s','tools/v2731','-p','test_financial_preflight.py','-v'],timeout=60)
    run('go_preflight_isolated',['bash','tools/v2727/run_go_isolated.sh'],timeout=90,optional=True)
    run('sql_sim_credit_reserve',[sys.executable,'tools/v2727/concurrency_sim.py'],timeout=60)
    run('sql_sim_unique_payment',[sys.executable,'tools/v2728/claim_concurrency_sim.py'],timeout=60)
    run('sql_sim_channel_quota',[sys.executable,'audits/test_v2730_quota_sim.py'],timeout=60)
    run('source_billing_static',[sys.executable,'tools/v2728/validate_offline.py'],timeout=25)
    run('cost_catalog_preflight',[sys.executable,'tools/v2731/financial_preflight.py'],timeout=25)

    source_check('critical_components_preserved',{
        'main.go':['/api/admin/finance/v2731/unit-economics','/api/admin/commercial/v2727/preflight', '/api/consumption/v2724/resources','/api/studio/video/generate'],
        'commercial_credits_v2721.go':['commercial','creative'],
        'creative_cost_guard_v2720.go':['WORKTIC_COST_GLOBAL_MONTHLY_CENTS','creative_cost_ledger_v2720'],
        'billing_lifecycle_v2728.go':['billing_payment_claims_v2728','PRIMARY KEY(network,tx_hash)'],
        'contract_entitlements_v2729.go':['WORKTIC_CONTRACT_ENTITLEMENTS_V2729'],
        'tenant_hardening_v2730.go':['tenant'],
        'finance_unit_economics_v2731.go':['public_launch_approved'],
    })
    main_src=(ROOT/'main.go').read_text(encoding='utf-8')
    paths=re.findall(r'mux\.HandleFunc\("([^\"]+)"',main_src)
    duplicates=[p for p,n in Counter(paths).items() if n>1]
    record('routes_unique','FAIL' if duplicates or len(paths)<198 else 'PASS', f'{len(paths)} literal routes, duplicate declarations={duplicates[:10]}. Does not prove missing dynamic handlers are safe.')

    # Confirm historical paths preserved (read only).
    prior=ROOT/'audits'/'RUTAS_COMPLETAS_V2722.csv'
    if prior.exists():
        with prior.open(encoding='utf-8-sig',newline='') as f:hist={r['path'] for r in csv.DictReader(f) if r.get('path','').startswith('/')}
        missing=sorted(hist-set(paths))
        record('legacy_routes_preserved','FAIL' if missing else 'PASS',f'{len(hist)} historic route declarations; missing={missing[:12]}')
    else:record('legacy_routes_preserved','FAIL','Inventory missing')

    # Check snapshot is not carrying active credentials or private .env. Still not a complete secret scan.
    private=[str(p.relative_to(ROOT)) for p in ROOT.rglob('.env')]+[str(p.relative_to(ROOT)) for p in ROOT.rglob('key.properties')]
    record('private_env_absent','FAIL' if private else 'PASS', f'Known private configuration files: {private[:5]}; example files allowed')
    flag_names=['WORKTIC_MODULE_GATES_V2722','WORKTIC_CONTRACT_ENTITLEMENTS_V2729']
    sample=(ROOT/'.env.render.example').read_text(encoding='utf-8')
    flags={k:re.search(r'^'+re.escape(k)+r'=([^\r\n]+)$',sample,re.M) for k in flag_names}
    flag_status={k:(m.group(1).strip() if m else 'missing') for k,m in flags.items()}
    record('pilot_flags_off','PASS' if all(v=='0' for v in flag_status.values()) else 'FAIL',str(flag_status))

    # Never claim tests not run. These require an isolated staging service & real business flows.
    for key,reason in [
        ('go126_full_build','Requires Go >= 1.26 + project dependencies, `go test ./...` and `go build ./...` on developer machine / CI.'),
        ('postgresql_multiinstance','Requires real independent PostgreSQL, multiple app instances and tenant A/B session credentials.'),
        ('redis_workers','Requires staging Redis and background workers with external calls mocked.'),
        ('payment_chain_and_refunds','Requires test transactions with real chain proofs, yearly renewals, refunds and Partners reversals.'),
        ('ai_provider_billing','Requires controlled OpenAI/Luma/Meta cost reconciliation with verified source documents.'),
        ('infrastructure_capacity','Requires independent staging hosting and 10/100/500/1000 tenant DATA scenarios; HTTP GET load != tenants.'),
        ('legacy_rules_tenant_migration','Requires verified ownership of historical global rules and E2E inbox/webhook tests.'),
        ('customer_acceptance_ui','Requires full browser login and authorization journey for all plan roles and tenants.'),
    ]:record(key,'BLOCKED',reason,scope='staging-required')

    failed=[x['id'] for x in TESTS if x['status']=='FAIL']
    blockers=[x['id'] for x in TESTS if x['status']=='BLOCKED']
    report={
        'version':'V27.32','generated_utc':ISO,'base':'V27.31','environment':'OFFLINE; no GitHub/Render or vendor calls',
        'tests':TESTS,'counts':dict(Counter(x['status'] for x in TESTS)),
        'offline_pass':not failed and all(x['status']=='PASS' for x in TESTS if x['scope']=='offline'),
        'unresolved_failures':failed, 'mandatory_unverified':blockers,
        'production_approved':False,
        'assertion':'Passing SQLite/unit/source tests does NOT establish Go/PostgreSQL E2E functionality or real profitability.',
    }
    DEST.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    md=ROOT/'audits'/'QA_SUMMARY_V2732.md'
    lines=['# WorkticAI V27.32 — informe de QA offline', '',
           f'Generado: {ISO}', '**APROBACIÓN DE PRODUCCIÓN: NO.**','','| Comprobación | Estado | Evidencia |','|---|---|---|']
    for x in TESTS: lines.append(f"| {x['id']} | {x['status']} | {x['detail'].replace('|','/')[:120]} |")
    lines += ['', f"**Fallos ejecutados:** {', '.join(failed) if failed else 'ninguno'}.",
              f"**Validaciones de staging pendientes:** {', '.join(blockers) if blockers else 'ninguna'}.",
              '', 'Los ensayos SQL SQLite son modelos independientes; no ejecutan handlers Go ni sustituyen PostgreSQL.',
              'No hay pruebas de carga reales ni facturas verificadas en esta sesión.']
    md.write_text('\n'.join(lines)+'\n',encoding='utf-8')
    print(json.dumps({'report':str(DEST),'counts':report['counts'],'production_approved':False,'failed':failed},ensure_ascii=False))
    return 1 if failed else 0

if __name__=='__main__':sys.exit(main())
