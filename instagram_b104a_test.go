package main

import (
 "encoding/json"
 "strings"
 "testing"
)

func TestB104ScopeVerificationFailClosed(t *testing.T) {
 for _, v:=range []json.RawMessage{nil, json.RawMessage(`null`),json.RawMessage(`{"unknown":1}`),json.RawMessage(`"instagram_business_basic"`)} {
  if instagramB104Granted(v,instagramB104MessagesPermission) { t.Fatalf("false positive: %s",v) }
 }
 if !instagramB104Granted(json.RawMessage(`"instagram_business_basic,instagram_business_manage_messages"`),instagramB104MessagesPermission){t.Fatal("string grant")}
 if !instagramB104Granted(json.RawMessage(`["instagram_business_basic","instagram_business_manage_messages"]`),instagramB104MessagesPermission){t.Fatal("array grant")}
 if instagramB104HasScope("instagram_business_manage_messages_other",instagramB104MessagesPermission){t.Fatal("substring match")}
}
func TestB104ExtractTextEchoAndAttachments(t *testing.T) {
 event:=[]byte(`{"object":"instagram","entry":[{"id":"17841435991636187","messaging":[{"sender":{"id":"1111111111111"},"recipient":{"id":"17841435991636187"},"timestamp":1794243600000,"message":{"mid":"mid-1","text":"hola"}},{"sender":{"id":"17841435991636187"},"recipient":{"id":"1111111111111"},"message":{"mid":"mid-2","text":"eco","is_echo":true}},{"sender":{"id":"1111111111111"},"recipient":{"id":"17841435991636187"},"message":{"mid":"mid-3"}}]}]}`)
 out,err:=instagramB104Extract(event)
 if err!=nil || len(out)!=1 || out[0].Text!="hola" || out[0].SenderID!="1111111111111" {t.Fatal(out,err)}
}
func TestB104RejectMisroutedTenantIdentity(t *testing.T) {
 bad:=[]byte(`{"object":"instagram","entry":[{"id":"17841435991636187","messaging":[{"sender":{"id":"1111111111111"},"recipient":{"id":"999999999999"},"message":{"mid":"m","text":"hello"}}]}]}`)
 if _,err:=instagramB104Extract(bad);err==nil {t.Fatal("accepted mismatched recipient")}
 if instagramB104Chat(7,9,"1111111111111")==instagramB104Chat(8,9,"1111111111111") {t.Fatal("tenant collision")}
 if instagramB104Chat(7,9,"1111111111111")==instagramB104Chat(7,10,"1111111111111") {t.Fatal("connection collision")}
}
func TestB104ReceiptHashNoRawMid(t *testing.T) {
 a:=instagramB104Hash("17841435991636187","mid-private")
 if strings.Contains(a,"private") || len(a)!=64 {t.Fatal(a)}
 if a==instagramB104Hash("999999999999","mid-private") {t.Fatal("cross account collision")}
}
