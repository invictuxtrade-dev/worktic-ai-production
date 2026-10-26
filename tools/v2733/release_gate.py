#!/usr/bin/env python3
"""Fail-closed release evidence gate: validates *presence and integrity* of staging proof.
It does NOT validate that humans described their evidence truthfully, and can NEVER
issue an automatic production approval. Use with human technical and finance review.
Does not access the network or change production.
"""
from __future__ import annotations
import argparse, datetime as dt, hashlib, json, pathlib, sys
ROOT = pathlib.Path(__file__).resolve().parents[2]
QA_IDS = [f'QA-{i:02d}' for i in range(1,17)]
STAGING_IDS = ['go126_full_build','postgresql_multiinstance','redis_workers',
               'payment_chain_and_refunds','ai_provider_billing','infrastructure_capacity',
               'legacy_rules_tenant_migration','customer_acceptance_ui']

def sha(p: pathlib.Path) -> str:
    return hashlib.sha256(p.read_bytes()).hexdigest()

def safe_evidence_path(name):
    if not isinstance(name,str) or not name.startswith('audits/evidence_v2733/'):
        return None
    p = ROOT / name
    # Restrict even symlinks and path traversal to the evidence directory.
    base = (ROOT/'audits'/'evidence_v2733').resolve()
    try:
        p.resolve().relative_to(base)
    except ValueError:
        return None
    return p if p.is_file() and not p.is_symlink() else None

def audit(acceptance_path):
    issues=[]
    offline=ROOT/'audits'/'QA_REPORT_V2732.json'
    if not offline.exists():
        issues.append('Falta reporte QA offline V27.32; ejecutar qa_runner.py')
    else:
        try:
            q=json.loads(offline.read_text(encoding='utf-8'))
            if not q.get('offline_pass') or q.get('unresolved_failures'):
                issues.append('Reporte offline tiene fallos')
            actual={x.get('id'):x.get('status') for x in q.get('tests',[])}
            if any(x.get('status')=='FAIL' for x in q.get('tests',[])):
                issues.append('Hay fallos FAIL en el reporte QA')
            missing=[k for k in STAGING_IDS if actual.get(k)!='BLOCKED']
            if missing:
                issues.append('Reporte QA V27.32 inesperado: '+', '.join(missing))
        except (ValueError,KeyError,TypeError) as ex:
            issues.append('Reporte offline ilegible: '+str(ex))
    if not acceptance_path.exists():
        issues.append('Falta evidencia de staging (copiar plantilla y completar después de probar)')
        return issues
    try:
        a=json.loads(acceptance_path.read_text(encoding='utf-8'))
    except (ValueError, OSError):
        return issues+['Evidencia de staging JSON inválida']
    if a.get('release')!='V27.33':issues.append('Versión de evidencia distinta de V27.33')
    manifest=ROOT/'RELEASE_MANIFEST_V2733.json'
    if not manifest.is_file() or a.get('source_manifest_sha256')!=sha(manifest):
        issues.append('Evidencia no corresponde al manifiesto SHA256 del código V27.33')
    # Verify the actual source against that release manifest, not just the evidence's claim.
    if manifest.is_file():
        try:
            expected=json.loads(manifest.read_text(encoding='utf-8'))['sha256_files']
            for rel,digest in expected.items():
                path=ROOT/rel
                if not path.is_file() or sha(path)!=digest:
                    issues.append('Código modificado desde la creación del manifiesto: '+rel)
                    break
        except (OSError,ValueError,KeyError,TypeError):
            issues.append('Manifiesto ilegible')
    env=a.get('staging_environment') or {}
    if env.get('isolated_db') is not True or env.get('isolated_redis') is not True or env.get('real_customer_data') is not False:
        issues.append('Staging debe usar PostgreSQL y Redis aislados, sin datos reales')
    host=str(env.get('host','')).lower()
    if not host or 'workticai.com' in host or ('staging' not in host and 'qa' not in host and 'localhost' not in host):
        issues.append('Host de staging ausente/inseguro')
    got=a.get('tests')
    if not isinstance(got,list):
        issues.append('Falta matriz de 16 pruebas');return issues
    ids=[x.get('id') for x in got if isinstance(x,dict)]
    if len(ids)!=16 or set(ids)!=set(QA_IDS) or len(set(ids))!=16:
        issues.append('Matriz QA-01..QA-16 incompleta o duplicada')
    for x in got:
        if not isinstance(x,dict):continue
        k=x.get('id','?')
        if x.get('status')!='PASS':issues.append(k+': no acreditada PASS');continue
        p=safe_evidence_path(x.get('evidence_path'))
        if not p:
            issues.append(k+': evidencia inexistente o fuera de audits/evidence_v2733/');continue
        actual_hash=sha(p)
        if x.get('sha256')!=actual_hash:
            issues.append(k+': hash SHA256 de evidencia incorrecto')
        if p.stat().st_size<40:
            issues.append(k+': evidencia insuficiente o vacía')
    sign=a.get('reviewers') or {}
    if not str(sign.get('technical','')).strip() or not str(sign.get('commercial','')).strip():
        issues.append('Faltan revisiones técnica y comercial')
    if str(sign.get('technical','')).strip()==str(sign.get('commercial','')).strip():
        issues.append('Las revisiones técnica y comercial deben estar separadas')
    # This date bounds validity; it is not proof of testing.
    try:
        reviewed=dt.datetime.fromisoformat(a.get('reviewed_utc','').replace('Z','+00:00'))
        if reviewed.tzinfo is None:raise ValueError('no time zone')
        if not (dt.datetime.now(dt.timezone.utc)-dt.timedelta(days=30)<=reviewed<=dt.datetime.now(dt.timezone.utc)+dt.timedelta(minutes=5)):
            issues.append('Evidencia vencida o con fecha futura')
    except (ValueError,TypeError):issues.append('Fecha UTC de revisión inválida')
    return issues

def main(argv=None):
    ap=argparse.ArgumentParser(description=__doc__)
    ap.add_argument('--evidence',default='audits/STAGING_EVIDENCE_V2733.json')
    args=ap.parse_args(argv)
    p=ROOT/args.evidence
    issues=audit(p)
    result={'release':'V27.33','evidence_complete_for_human_review':not bool(issues),
            'production_approved':False,'automatic_deploy_allowed':False,
            'blocking_reasons':issues,'next_action':'Technical and commercial owners must independently inspect staging evidence before any deployment.'}
    print(json.dumps(result,ensure_ascii=False,indent=2))
    return 1 if issues else 0
if __name__=='__main__':sys.exit(main())
