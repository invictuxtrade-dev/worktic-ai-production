#!/usr/bin/env python3
"""Re-check source files in the release manifest; never upload, push or deploy."""
import hashlib,json,pathlib,sys
ROOT=pathlib.Path(__file__).resolve().parents[2]
MANIFEST=ROOT/'RELEASE_MANIFEST_V2733.json'
def include(p):
 r=p.relative_to(ROOT).as_posix()
 return p.is_file() and (p.suffix in ('.go','.js','.css','.html','.mod','.sum','.py','.ps1','.sh','.yaml','.yml') or p.name in ('VERSION.txt','.env.render.example')) and not '__pycache__' in p.parts

def run():
 man=json.loads(MANIFEST.read_text())
 known=man['sha256_files']
 actual={p.relative_to(ROOT).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in ROOT.rglob('*') if include(p)}
 unexpected=sorted(set(actual)-set(known))
 missing=sorted(set(known)-set(actual))
 tampered=sorted(k for k in actual.keys()&known.keys() if actual[k]!=known[k])
 print(json.dumps({'expected_files':len(known),'actual_files':len(actual),'added':unexpected,'missing':missing,'changed':tampered,'integrity_ok':not(any((unexpected,missing,tampered)))},ensure_ascii=False))
 return 1 if any((unexpected,missing,tampered)) else 0
if __name__=='__main__':sys.exit(run())
