# V27.33 - Run inside an ISOLATED local/staging copy from VS Code PowerShell.
# Does not deploy. Returns an error if a required check fails.
$ErrorActionPreference = 'Stop'
Set-Location (Resolve-Path (Join-Path $PSScriptRoot '..\..'))
Write-Host 'WorkticAI V27.33 - Quality and release preflight (NO DEPLOY)'
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'Python is required' }
if (-not (Get-Command node -ErrorAction SilentlyContinue)) { throw 'Node.js is required' }
if (-not (Get-Command go -ErrorAction SilentlyContinue)) { throw 'Go 1.26 is required' }
$version = go version
Write-Host $version
if ($version -notmatch 'go1\.26\.') { throw 'Expected Go 1.26.x. Upgrade Go before proceeding.' }
python tools/v2732/qa_runner.py
if ($LASTEXITCODE -ne 0) { throw 'Offline QA failed' }
python -m unittest discover -s tools/v2732 -p 'test_*.py' -v
if ($LASTEXITCODE -ne 0) { throw 'Staging safety tests failed' }
python -m unittest discover -s tools/v2733 -p 'test_*.py' -v
if ($LASTEXITCODE -ne 0) { throw 'Release-gate tests failed' }
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Project Go tests failed' }
go build ./...
if ($LASTEXITCODE -ne 0) { throw 'Project Go build failed' }
python tools/v2733/verify_manifest.py
if ($LASTEXITCODE -ne 0) { throw 'Release manifest mismatch' }
Write-Host 'Local preflight passed. Staging E2E and human finance approval STILL REQUIRED.'
python tools/v2733/release_gate.py
if ($LASTEXITCODE -ne 0) { throw 'RELEASE BLOCKED: Real staging acceptance evidence and human signoff still required.' }
