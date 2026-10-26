#!/usr/bin/env python3
"""Standalone SQLite concurrency rehearsal, NOT an execution of Worktic Go handlers.
PostgreSQL + actual Go+HTTP workers are mandatory before production."""
import sqlite3, tempfile, os, concurrent.futures

with tempfile.TemporaryDirectory() as tmp:
    db=os.path.join(tmp,'staging_quota.db')
    con=sqlite3.connect(db)
    con.execute('PRAGMA journal_mode=WAL')
    con.executescript('''CREATE TABLE locks(tenant_id INTEGER PRIMARY KEY);
      CREATE TABLE channels(id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL,type TEXT NOT NULL,status TEXT NOT NULL);
      CREATE TABLE contacts(id INTEGER PRIMARY KEY AUTOINCREMENT,tenant_id INTEGER NOT NULL,external_id TEXT NOT NULL,deleted_at TEXT NOT NULL DEFAULT '');''')
    con.commit();con.close()

    def channel_job(arg):
        tid,i=arg
        c=sqlite3.connect(db,timeout=15,isolation_level=None)
        try:
            c.execute('BEGIN IMMEDIATE')
            c.execute('INSERT INTO locks(tenant_id) VALUES(?) ON CONFLICT DO NOTHING',(tid,))
            c.execute('UPDATE locks SET tenant_id=tenant_id WHERE tenant_id=?',(tid,))
            n=c.execute("SELECT COUNT(*) FROM channels WHERE tenant_id=? AND status<>'deleted'",(tid,)).fetchone()[0]
            if n>=5: c.execute('ROLLBACK');return False
            c.execute('INSERT INTO channels(tenant_id,type,status) VALUES(?,?,?)',(tid,'whatsapp_cloud','draft'))
            c.execute('COMMIT')
            return True
        except: c.execute('ROLLBACK');raise
        finally:c.close()

    with concurrent.futures.ThreadPoolExecutor(max_workers=20) as ex:
        answers=list(ex.map(channel_job, [(1,i) for i in range(24)]+[(2,i) for i in range(24)]))
    assert sum(answers)==10, sum(answers)
    con=sqlite3.connect(db)
    counts=con.execute('SELECT tenant_id,COUNT(*) FROM channels GROUP BY tenant_id ORDER BY tenant_id').fetchall()
    assert counts==[(1,5),(2,5)],counts
    print('Canales simulated: 48 concurrent attempts, 10 allowed, 38 refused; 5 per tenant')
    con.close()
