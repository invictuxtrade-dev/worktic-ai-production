package main

import "testing"

func TestCommercialPlanMatrix(t *testing.T) {
    seeds:=commercialPlanSeed()
    if len(seeds)!=4 {t.Fatal("deben existir 4 niveles principales")}
    for _,r:=range seeds {
        if r.Plan=="free" && (r.Monthly!=0||r.Image||r.Text||r.Voice||r.Video) {t.Fatal("Free no puede tener costo externo")}
        if r.Plan!="free" && (r.Monthly==0||!r.Editor||!r.Image||!r.Video||!r.Voice||!r.Text){t.Fatalf("plan %s incompleto",r.Plan)}
    }
}
func TestCommercialCreditsTariffsBounded(t *testing.T) {
    for _,tc:=range []struct{action string; sec,chars int;want,provider int64}{
        {"image",0,0,4,8},{"video",5,0,15,36},{"video",10,0,45,99},
        {"copy",0,0,2,5},{"storyboard",0,0,2,5},{"narration",0,1,5,10},{"narration",0,1001,10,20},
    } {
        got,err:=commercialCreditsQuote(tc.action,tc.sec,tc.chars);if err!=nil||got!=tc.want{t.Errorf("quote %v %d, want %d (%v)",tc.action,got,tc.want,err)}
        if !commercialQuoteWithinBudget(tc.provider,got){t.Errorf("%s exceeds cost-per-credit ceiling",tc.action)}
    }
    if commercialQuoteWithinBudget(75,4) {t.Fatal("precio de proveedor caro no debe pasar")}
    if _,err:=commercialCreditsQuote("video",15,0);err==nil{t.Fatal("duración no permitida")}
    if _,err:=commercialCreditsQuote("narration",0,5000);err==nil{t.Fatal("texto ilimitado")}
}
func TestCommercialHashAndFinancialProtection(t *testing.T){
    if !commercialTxHash("BEP20","0x"+repeatForCommercialTest("a",64)){t.Fatal("hash bep20")}
    if !commercialTxHash("TRC20",repeatForCommercialTest("1",64)){t.Fatal("hash trc20")}
    if commercialTxHash("BEP20","0x1234")||commercialTxHash("TRC20","../../etc/passwd") {t.Fatal("hash inválido aceptado")}
    v,err:=commercialExampleFinancial(2500,20,50,30,2)
    if err!=nil||v!=940 {t.Fatalf("escenario %d %v",v,err)} // 2000 - 1000 - 60
    if _,err:=commercialExampleFinancial(2500,20,60,30,2);err==nil {t.Fatal("comisión sobre el 50%")}
}
func repeatForCommercialTest(s string,n int)string{out:="";for i:=0;i<n;i++{out+=s};return out}
