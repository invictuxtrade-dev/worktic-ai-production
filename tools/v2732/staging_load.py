#!/usr/bin/env python3
"""SAFE read-only request load on a deliberately isolated staging server.
NOT a tenant/user capacity test; not valid for production domains or business writes.
Will not run without --acknowledge and WORKTIC_QA_STAGING_ONLY env token.
Only GET /healthz and GET /readyz. Requires localhost or staging/qa/test hostname.
"""
import argparse,concurrent.futures,datetime,json,os,sys,time,urllib.parse,urllib.request,urllib.error

CONFIRM='I_CONFIRM_ISOLATED_STAGING'

def validate_url(url):
    parsed=urllib.parse.urlsplit(url)
    host=(parsed.hostname or '').lower()
    host_parts=host.split('.')
    # Deny the entire real production domain, including deceptive staging-like subdomains.
    production=any(host==domain or host.endswith('.'+domain) for domain in ('workticai.com','worktic.com.co'))
    is_local=host in ('localhost','127.0.0.1')
    is_stage=(host.startswith(('staging-','staging.','qa-','qa.','test-','test.','worktic-qa','worktic-staging')) or any(part in ('staging','qa','test') for part in host_parts[:-1]))
    if (parsed.scheme not in ('http','https') or not host or production or not (is_local or is_stage) or parsed.username or parsed.password or parsed.query or parsed.fragment or parsed.path not in ('','/')):
        raise ValueError('URL rechazado. Usar localhost o un hostname de staging/qa/test explícito, sin rutas ni credenciales.')
    if parsed.scheme!='https' and not is_local:raise ValueError('Staging remoto debe usar HTTPS')
    return url.rstrip('/')

class _NoRedirect(urllib.request.HTTPRedirectHandler):
    """Never follow a 3xx response: redirects might point to production."""
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None

_SAFE_OPENER = urllib.request.build_opener(_NoRedirect)

def trial(base,path,timeout):
    started=time.perf_counter()
    req=urllib.request.Request(base+path,headers={'User-Agent':'WorkticAI-QA-V2732-ReadOnly','Cache-Control':'no-store'},method='GET')
    try:
        with _SAFE_OPENER.open(req,timeout=timeout) as r:
            code=r.status
            r.read(4096)
    except Exception as exc:
        code=getattr(exc,'code',0) or 0
    return code,1000*(time.perf_counter()-started)

def main(argv=None):
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--base',required=True)
    p.add_argument('--requests',type=int,default=30)
    p.add_argument('--concurrency',type=int,default=3)
    p.add_argument('--path',choices=['/healthz','/readyz'],default='/healthz')
    p.add_argument('--acknowledge',action='store_true',help='I confirm this is isolated staging, never production')
    a=p.parse_args(argv)
    if not a.acknowledge or os.environ.get('WORKTIC_QA_STAGING_ONLY')!=CONFIRM:
        p.error('Requires --acknowledge and WORKTIC_QA_STAGING_ONLY='+CONFIRM)
    try:base=validate_url(a.base)
    except ValueError as e:p.error(str(e))
    if not 1<=a.requests<=300 or not 1<=a.concurrency<=10 or a.concurrency>a.requests:
        p.error('Safety limits: 1..300 requests, concurrency 1..10')
    if a.requests/a.concurrency>300:p.error('Excessive work')
    start=time.monotonic()
    with concurrent.futures.ThreadPoolExecutor(max_workers=a.concurrency) as ex:
        res=list(ex.map(lambda _:trial(base,a.path,timeout=4),range(a.requests)))
    elapsed=time.monotonic()-start
    lat=sorted(t for _,t in res)
    status={str(k):sum(1 for s,_ in res if s==k) for k,_ in res}
    output={'test':'Read-only staging endpoint request load; NOT tenant simulation',
        'target_host':urllib.parse.urlsplit(base).hostname,'path':a.path,'requests':a.requests,
        'concurrency':a.concurrency,'elapsed_seconds':round(elapsed,3),
        'status_counts':status,'success_ratio':round(sum(s==200 for s,_ in res)/len(res),4),
        'latency_p50_ms':round(lat[(len(lat)-1)//2],2),'latency_p95_ms':round(lat[min(len(lat)-1,round(.95*(len(lat)-1)))],2),
        'tenant_e2e_certified':False,'production_approved':False,'generated_utc':datetime.datetime.now(datetime.timezone.utc).isoformat()}
    print(json.dumps(output,ensure_ascii=False,indent=2))
    return 0 if output['success_ratio']==1 else 1

if __name__=='__main__':sys.exit(main())
