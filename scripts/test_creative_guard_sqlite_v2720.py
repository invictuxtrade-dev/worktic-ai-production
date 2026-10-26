"""Local, network-free approximation of the V27.20 atomic reservation SQL.
This is NOT a Go/Render/PostgreSQL integration test. Run with Python 3.
"""
import sqlite3
import threading
import tempfile
from pathlib import Path

DDL = """CREATE TABLE creative_cost_budgets_v2720 (
 tenant_id INTEGER NOT NULL,period TEXT NOT NULL,limit_cents INTEGER NOT NULL DEFAULT 0,
 reserved_cents INTEGER NOT NULL DEFAULT 0,spent_cents INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(tenant_id,period))"""

def test_concurrent_reservations():
    with tempfile.TemporaryDirectory() as tmp:
        path = str(Path(tmp)/"budget.db")
        con = sqlite3.connect(path)
        con.execute("PRAGMA journal_mode=WAL")
        con.execute(DDL)
        con.commit(); con.close()
        successes=[]
        lock=threading.Lock()
        barrier=threading.Barrier(12)
        def request(i):
            db=sqlite3.connect(path, timeout=10)
            barrier.wait()
            try:
                db.execute("BEGIN IMMEDIATE")
                for tid,limit in ((0,400),(7,300)):
                    db.execute("INSERT INTO creative_cost_budgets_v2720(tenant_id,period,limit_cents,reserved_cents,spent_cents) VALUES(?,?,?,0,0) ON CONFLICT(tenant_id,period) DO UPDATE SET limit_cents=excluded.limit_cents",(tid,"2026-10",limit))
                    r=db.execute("UPDATE creative_cost_budgets_v2720 SET reserved_cents=reserved_cents+? WHERE tenant_id=? AND period=? AND spent_cents+reserved_cents+?<=limit_cents",(75,tid,"2026-10",75))
                    if r.rowcount!=1:
                        db.rollback()
                        return
                db.commit()
                with lock: successes.append(i)
            finally: db.close()
        ts=[threading.Thread(target=request,args=(i,)) for i in range(12)]
        for t in ts: t.start()
        for t in ts: t.join()
        assert len(successes)==4, f"expected exactly 4 reservations, got {len(successes)}"
        db=sqlite3.connect(path)
        rows=db.execute("SELECT tenant_id,reserved_cents,spent_cents FROM creative_cost_budgets_v2720 ORDER BY tenant_id").fetchall()
        assert rows==[(0,300,0),(7,300,0)], rows
        db.close()
        print("PASS: 12 simulated concurrent requests, exactly 4 reservations; tenant and global caps enforced")

if __name__=='__main__':test_concurrent_reservations()
