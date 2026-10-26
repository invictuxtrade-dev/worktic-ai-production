#!/usr/bin/env python3
"""Offline SQLite concurrency test for SQL-level cross-product claim uniqueness.
No app DB, Render or blockchain connection; this is NOT a PostgreSQL E2E test.
"""
import concurrent.futures, os, sqlite3, tempfile

CREATE = '''CREATE TABLE payment_claims (
 network TEXT NOT NULL, tx_hash TEXT NOT NULL, source_type TEXT NOT NULL,
 source_id TEXT NOT NULL, tenant_id INTEGER NOT NULL, claimed_at TEXT NOT NULL,
 PRIMARY KEY(network,tx_hash))'''

def try_claim(db, i):
    con = sqlite3.connect(db, timeout=8.0)
    try:
        con.execute('BEGIN IMMEDIATE')
        con.execute('INSERT INTO payment_claims VALUES(?,?,?,?,?,?)',
                    ('BEP20', '0x'+'a'*64, 'subscription' if i % 2 == 0 else 'studio_topup', str(i), 42, '2026-10-23'))
        con.commit()
        return True
    except sqlite3.IntegrityError:
        con.rollback()
        return False
    finally:
        con.close()


def main():
    with tempfile.TemporaryDirectory() as d:
        db = os.path.join(d, 'claims.db')
        c = sqlite3.connect(db)
        c.execute('PRAGMA journal_mode=WAL')
        c.execute(CREATE)
        c.commit(); c.close()
        with concurrent.futures.ThreadPoolExecutor(max_workers=12) as pool:
            successes = list(pool.map(lambda i: try_claim(db, i),range(48)))
        c=sqlite3.connect(db)
        total=c.execute('SELECT COUNT(*) FROM payment_claims').fetchone()[0]
        source=c.execute('SELECT source_type FROM payment_claims').fetchone()[0]
        c.close()
        assert sum(successes)==1 and total==1, (sum(successes),total)
        print(f'OK: 48 solicitudes simultáneas, {sum(successes)} aceptada, {48-sum(successes)} rechazadas. Origen ganador: {source}')
        print('Esta prueba utiliza una tabla SQLite simulada; PostgreSQL de staging queda pendiente.')
if __name__=='__main__': main()
