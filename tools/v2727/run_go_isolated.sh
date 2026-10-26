#!/usr/bin/env bash
# Compile and test the actual V27.27 preflight source with disposable type stubs.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cp "$ROOT/commercial_validation_v2727.go" "$TMP/"
cat > "$TMP/stub_test.go" <<'GO'
package main
import("database/sql";"net/http";"testing")
type DB struct{}
func(*DB)QueryRow(q string,args ...any)*sql.Row{panic("DB queries disabled in isolated tests")}
type User struct{}
type Config struct{AppEnv string}
type App struct{ db *DB;cfg Config }
func(*App)requireSuperadmin(http.ResponseWriter,*http.Request)(*User,bool){return nil,false}
func writeJSON(http.ResponseWriter,any){}
type commercialRule struct{Plan string}
func commercialPlanSeed()[]commercialRule{return []commercialRule{{"free"},{"personal"},{"business"},{"enterprise"}}}
func TestProductionNeverConfusedWithStaging(t *testing.T){
 c:=commercialPreflightChecksV2727("production",map[string]string{})
 if c[0].Key!="isolation"||c[0].Status!="block"{t.Fatal("no production block")}
}
func TestCommercialDisabledAndQuotasOff(t *testing.T){
 c:=commercialPreflightChecksV2727("staging",map[string]string{})
 m:=map[string]string{};for _,x:=range c{m[x.Key]=x.Status}
 if m["sale_switch"]!="pass" || m["resource_quotas"]!="block" || m["operation_meter"]!="block"{t.Fatalf("unexpected flags %+v",m)}
}
func TestFlagsRequireExternalEvidence(t *testing.T){
 c:=commercialPreflightChecksV2727("staging",map[string]string{"WORKTIC_COMMERCIAL_ENABLE":"1","WORKTIC_MODULE_GATES_V2722":"1","WORKTIC_RESOURCE_GUARD_V2724":"1","WORKTIC_METER_V2726":"1"})
 m:=map[string]string{};for _,x:=range c{m[x.Key]=x.Status}
 if m["sale_switch"]!="manual"||m["module_gates"]!="manual"||m["resource_quotas"]!="manual"||m["operation_meter"]!="manual"{t.Fatalf("unexpected flags %+v",m)}
}
GO
(cd "$TMP" && GO111MODULE=off go test -v .)
