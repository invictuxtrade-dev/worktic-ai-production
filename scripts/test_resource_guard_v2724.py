#!/usr/bin/env python3
"""SQLite simulation of quota SQL; DOES NOT certify Go / PostgreSQL E2E."""
import sqlite3
import tempfile
import threading
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

with tempfile.TemporaryDirectory() as td:
    db = Path(td)/'pilot.db'
    con=sqlite3.connect(db)
    con.executescript('''CREATE TABLE consumption_tenant_locks_v2723(tenant_id INTEGER PRIMARY KEY);
CREATE TABLE crm_products(id INTEGER PRIMARY KEY,tenant_id INTEGER);
CREATE TABLE resource_meter_v2724(tenant_id INTEGER,period TEXT,resource TEXT,attempts INTEGER DEFAULT 0,PRIMARY KEY(tenant_id,period,resource));''')
    con.commit();con.close()
    bar=threading.Barrier(24)
    def insert(tid):
        c=sqlite3.connect(db,timeout=10,isolation_level=None)
        try:
            bar.wait()
            c.execute('BEGIN IMMEDIATE')
            c.execute('INSERT INTO consumption_tenant_locks_v2723(tenant_id) VALUES(?) ON CONFLICT(tenant_id) DO NOTHING',(tid,))
            c.execute('UPDATE consumption_tenant_locks_v2723 SET tenant_id=tenant_id WHERE tenant_id=?',(tid,))
            n=c.execute('SELECT COUNT(*) FROM crm_products WHERE tenant_id=?',(tid,)).fetchone()[0]
            if n >=5: c.execute('ROLLBACK');return False
            c.execute('INSERT INTO crm_products(tenant_id) VALUES(?)',(tid,))
            c.execute('COMMIT');return True
        except BaseException:
            try:c.execute('ROLLBACK')
            except sqlite3.Error:pass
            raise
        finally:c.close()
    with ThreadPoolExecutor(max_workers=24) as pool:
        result=list(pool.map(insert,[1]*12+[2]*12))
    assert sum(result)==10,sum(result)
    c=sqlite3.connect(db)
    assert c.execute('SELECT tenant_id,COUNT(*) FROM crm_products GROUP BY tenant_id').fetchall()==[(1,5),(2,5)]
    # Identical monthly meter condition used by the Go implementation.
    c.execute('INSERT INTO resource_meter_v2724 VALUES(1,\'2026-10\',\'ads_syncs\',4)')
    r=c.execute("UPDATE resource_meter_v2724 SET attempts=attempts+1 WHERE tenant_id=1 AND period='2026-10' AND resource='ads_syncs' AND attempts<5")
    assert r.rowcount==1
    r=c.execute("UPDATE resource_meter_v2724 SET attempts=attempts+1 WHERE tenant_id=1 AND period='2026-10' AND resource='ads_syncs' AND attempts<5")
    assert r.rowcount==0
    c.commit(); c.close()
    print('PASS: 24 simultaneous product requests, exactly 5 per tenant; monthly meter stops at cap')
