#!/usr/bin/env python3
"""SAFE OFFLINE unit simulation. Does not access Worktic DB/Render or APIs."""
import concurrent.futures
import sqlite3
import tempfile
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent


def run():
    source=(ROOT/'main.go').read_text(encoding='utf-8')
    routes=re.findall(r'\bmux\.HandleFunc\(\s*"([^"]+)"',source)
    assert len(routes)>=198, f'router unexpectedly shrank to {len(routes)}'
    assert len(routes)==len(set(routes)), 'duplicate router paths'
    assert '/api/consumption/v2723/summary' in routes
    assert '/api/consumption/v2723/legacy-rules' in routes
    src=(ROOT/'consumption_guard_v2723.go').read_text()
    assert 'SELECT tenant_id FROM consumption_tenant_locks_v2723 WHERE tenant_id=? FOR UPDATE' in src
    assert 'estimated_cost_cents INTEGER' in src
    assert 'UNIQUE(tenant_id,provider,operation,external_ref)' in src
    assert 'WHERE tenant_id=? AND period=? AND attempts<?' in src

    with tempfile.TemporaryDirectory(prefix='worktic_safe_v2723_') as tmp:
        dbpath=str(pathlib.Path(tmp)/'isolated_simulation.db')
        con=sqlite3.connect(dbpath,timeout=10)
        con.executescript('''
        CREATE TABLE automation_monthly_usage_v2723 (
         tenant_id INTEGER NOT NULL, period TEXT NOT NULL,attempts INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(tenant_id,period));
        CREATE TABLE worktic_auto_rules_v2723(
         id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,name TEXT NOT NULL,
         match_type TEXT NOT NULL DEFAULT 'contains',keyword TEXT NOT NULL,
         response TEXT NOT NULL,enabled INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL);
        CREATE TABLE external_usage_v2723(
         id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,
         provider TEXT NOT NULL,operation TEXT NOT NULL,external_ref TEXT NOT NULL,status TEXT NOT NULL,
         units INTEGER NOT NULL DEFAULT 1,estimated_cost_cents INTEGER,currency TEXT NOT NULL DEFAULT 'USD',
         created_at TEXT NOT NULL,UNIQUE(tenant_id,provider,operation,external_ref));''')
        con.commit();con.close()
        def work(tid, cap):
            c=sqlite3.connect(dbpath,timeout=30)
            try:
                c.execute('INSERT INTO automation_monthly_usage_v2723(tenant_id,period,attempts) VALUES(?,?,0) ON CONFLICT(tenant_id,period) DO NOTHING',(tid,'2026-10'))
                res=c.execute('UPDATE automation_monthly_usage_v2723 SET attempts=attempts+1 WHERE tenant_id=? AND period=? AND attempts<?',(tid,'2026-10',cap))
                c.commit()
                return res.rowcount
            finally:c.close()
        with concurrent.futures.ThreadPoolExecutor(max_workers=12) as pool:
            results=list(pool.map(lambda _:work(1,5),range(24)))
        assert sum(results)==5,(results,sum(results))
        assert results.count(0)==19
        assert work(2,1)==1 and work(2,1)==0
        c=sqlite3.connect(dbpath)
        c.execute("INSERT INTO worktic_auto_rules_v2723(tenant_id,name,keyword,response,created_at) VALUES(1,'Empresa A','hi','A','now')")
        c.execute("INSERT INTO worktic_auto_rules_v2723(tenant_id,name,keyword,response,created_at) VALUES(2,'Empresa B','hi','B','now')")
        assert c.execute('SELECT response FROM worktic_auto_rules_v2723 WHERE tenant_id=?',(1,)).fetchall()==[('A',)]
        insert="INSERT INTO external_usage_v2723(tenant_id,provider,operation,external_ref,status,estimated_cost_cents,created_at) VALUES(1,'meta_whatsapp','template','mid-1','accepted',NULL,'now') ON CONFLICT(tenant_id,provider,operation,external_ref) DO NOTHING"
        c.execute(insert);c.execute(insert)
        assert c.execute('SELECT COUNT(*) FROM external_usage_v2723').fetchone()[0]==1
        assert c.execute('SELECT estimated_cost_cents FROM external_usage_v2723').fetchone()[0] is None
        c.commit();c.close()
    print(f'OK: {len(routes)} router registrations; 24 simultaneous attempts => 5 accepted, 19 rejected; tenant isolation + idempotency OK')

if __name__=='__main__':
    try:run()
    except Exception as exc: print('FAIL:',exc,file=sys.stderr);raise
