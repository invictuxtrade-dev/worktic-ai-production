# Validation — WorkticAI V27.15 Partner Business Backoffice

Build: 2026-10-15

## Static / syntax validation performed
- `gofmt` applied to changed Go sources.
- New `partners_v2715.go` type-checked in an isolated stdlib-only harness with stubs for existing WorkticAI APIs: PASS.
- V27.14 + V27.15 Partner CREATE/ALTER schema blocks executed against an in-memory SQLite validation database: PASS.
- `node --check static/partners-v2715.js`: PASS.
- `node --check static/app.js`: PASS.
- `node --check static/admin-full.js`: PASS.
- Public V27.13.1 files compared byte-for-byte against V27.14 preserved baseline: unchanged.
- Academy V8.1 PHP bridge files pass `php -l`.

## Full Go test limitation in generation environment
The generation environment has local Go 1.23 while this project correctly declares Go 1.26. Network access is blocked, so the Go 1.26 toolchain/modules cannot be downloaded here. The final source keeps `go 1.26.0` unchanged.

Before Git push, run on the Windows development machine with Go 1.26 available:

```powershell
Get-ChildItem -Recurse -Filter *.go | ForEach-Object { gofmt -w $_.FullName }
go test ./...
git diff --check
```

Do not deploy if `go test ./...` fails.

## Migration
No global database migration is required. The new Partner schema is additive and initializes on WorkticAI startup. Do not execute `/app/worktic-migrate`.
