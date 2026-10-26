#!/usr/bin/env python3
"""V27.34 fail-closed source integrity; no deployment actions."""
import pathlib,hashlib,json,sys
root=pathlib.Path(__file__).resolve().parents[2]
manifest=json.loads((root/'RELEASE_MANIFEST_V2734.json').read_text(encoding='utf-8'))
def include(p):
 return p.is_file() and p.name not in ('RELEASE_MANIFEST_V2734.json',) and ('__pycache__' not in p.parts) and (p.suffix in ('.go','.js','.css','.html','.mod','.sum','.py','.ps1','.sh','.yaml','.yml') or p.name in ('VERSION.txt','.env.render.example'))
actual={p.relative_to(root).as_posix():hashlib.sha256(p.read_bytes()).hexdigest() for p in root.rglob('*') if include(p)}
expected=manifest['sha256_files']
changed=[k for k in actual.keys()&expected.keys() if actual[k]!=expected[k]]
missing=sorted(expected.keys()-actual.keys())
added=sorted(actual.keys()-expected.keys())
print(json.dumps({'release':'V27.34','files':len(actual),'integrity_ok':not(changed or missing or added),'changed':sorted(changed),'missing':missing,'added':added},ensure_ascii=False,indent=2))
sys.exit(bool(changed or missing or added))
