#!/usr/bin/env python3
"""Offline model, not application E2E. SQLite concurrency simulates atomic wallet holds.
No customer data, network calls, provider costs or paid operations.
"""
import concurrent.futures,json,os,sqlite3,tempfile
from pathlib import Path
from datetime import datetime,timezone

def run():
 fd,path=tempfile.mkstemp(prefix='worktic_v2727_',suffix='.db');os.close(fd)
 try:
  db=sqlite3.connect(path);db.executescript('''PRAGMA journal_mode=WAL;
  CREATE TABLE wallets(tenant INTEGER PRIMARY KEY, amount INTEGER NOT NULL, reserved INTEGER NOT NULL DEFAULT 0, spent INTEGER NOT NULL DEFAULT 0,CHECK(amount>=0 AND reserved>=0 AND spent>=0));
  CREATE TABLE holds(id TEXT PRIMARY KEY,tenant INTEGER NOT NULL,credits INTEGER NOT NULL,status TEXT NOT NULL);
  INSERT INTO wallets(tenant,amount)VALUES(101,5),(202,5);''');db.commit();db.close()
  def reserve(tid,request_id):
   conn=sqlite3.connect(path,timeout=20,isolation_level=None)
   try:
    conn.execute('BEGIN IMMEDIATE')
    r=conn.execute('UPDATE wallets SET reserved=reserved+1 WHERE tenant=? AND amount-spent-reserved>=1',(tid,))
    if r.rowcount!=1:conn.rollback();return 'no_balance'
    conn.execute('INSERT INTO holds(id,tenant,credits,status)VALUES(?,?,1,?)',(request_id,tid,'held'))
    conn.commit();return 'held'
   except sqlite3.IntegrityError:conn.rollback();return 'duplicate'
   finally:conn.close()
  def settle(tid,request_id):
   conn=sqlite3.connect(path,timeout=20,isolation_level=None)
   try:
    conn.execute('BEGIN IMMEDIATE')
    r=conn.execute("UPDATE holds SET status='spent' WHERE id=? AND tenant=? AND status='held'",(request_id,tid))
    if r.rowcount!=1:conn.rollback();return False
    r=conn.execute('UPDATE wallets SET reserved=reserved-1,spent=spent+1 WHERE tenant=? AND reserved>=1',(tid,))
    if r.rowcount!=1:conn.rollback();return False
    conn.commit();return True
   finally:conn.close()
  tasks=[(t,f'{t}-{i}') for t in (101,202) for i in range(24)]
  with concurrent.futures.ThreadPoolExecutor(max_workers=16) as ex: outcomes=list(ex.map(lambda args:reserve(*args),tasks))
  conn=sqlite3.connect(path)
  holds=conn.execute('SELECT tenant,COUNT(*) FROM holds GROUP BY tenant ORDER BY tenant').fetchall()
  conn.close()
  assert holds==[(101,5),(202,5)],holds
  assert outcomes.count('held')==10
  duplicate=reserve(101,'101-0') # wallet exhausted => no_balance; existing ledger remains 1
  with concurrent.futures.ThreadPoolExecutor(max_workers=10) as ex: results=list(ex.map(lambda args:settle(*args),tasks))
  assert sum(results)==10
  conn=sqlite3.connect(path)
  wallets=conn.execute('SELECT tenant,amount,reserved,spent FROM wallets ORDER BY tenant').fetchall()
  count=conn.execute('SELECT COUNT(*) FROM holds WHERE status="spent"').fetchone()[0]
  conn.close()
  assert wallets==[(101,5,0,5),(202,5,0,5)],wallets
  assert count==10
  assert not settle(101,'101-0')
  return {'test':'SQLite isolated transactional wallet model, NOT actual WorkticAI/PostgreSQL','requested':len(tasks),'allowed':outcomes.count('held'),'blocked':len(tasks)-outcomes.count('held'),'tenants':2,'per_tenant_cap':5,'final_wallets':wallets,'settlements':count,'repeat_settlement_blocked':True,'duplicate_after_exhaustion':duplicate,'provider_calls':0,'passed':True,'generated_utc':datetime.now(timezone.utc).isoformat()}
 finally:
  try:os.unlink(path)
  except OSError:pass
if __name__=='__main__':print(json.dumps(run(),ensure_ascii=False,indent=2))
