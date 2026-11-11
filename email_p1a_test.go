package main

import (
 "encoding/base64"
 "encoding/json"
 "net/http"
 "testing"
)
func TestEmailP1Address(t *testing.T){
 for _,s:=range []string{"bad","", "a@@b.com","a@b.com\r\nBcc:evil@b.com"}{if _,e:=emailP1Address(s);e==nil{t.Fatalf("aceptó dirección inválida: %q",s)}}
 x,e:=emailP1Address("Nombre <ABC@EXAMPLE.COM>");if e!=nil||x!="abc@example.com"{t.Fatal(x,e)}
}
func TestEmailP1Heuristic(t *testing.T){
 c,p,_:=emailP1Heuristic("Necesito cotización urgente","precio plan");if c!="Ventas"||p!="alta"{t.Fatalf("%s %s",c,p)}
 c,p,_=emailP1Heuristic("Hola","Buen día");if c!="General"||p!="normal"{t.Fatal(c,p)}
}
func TestEmailP1GmailPlain(t *testing.T){
 body:=base64.RawURLEncoding.EncodeToString([]byte("Mensaje de prueba"))
 payload,_:=json.Marshal(map[string]any{"mimeType":"multipart/alternative","parts":[]any{map[string]any{"mimeType":"text/plain","body":map[string]string{"data":body}}}})
 if s:=emailP1GmailPlain(payload);s!="Mensaje de prueba"{t.Fatalf("body=%q",s)}
}
func TestEmailP1Origin(t *testing.T){
 r,_:=http.NewRequest("POST","https://www.workticai.com/api/email/p1/reply",nil);r.Header.Set("Origin","https://evil.example");if emailP1Origin(r){t.Fatal("origen cruzado admitido")}
 r.Header.Set("Origin","https://www.workticai.com");if !emailP1Origin(r){t.Fatal("origen propio rechazado")}
}
func TestEmailMktUnsubscribeSigned(t *testing.T){
 t.Setenv("WORKTIC_EMAIL_MARKETING_UNSUBSCRIBE_SECRET","muy-larga-clave-de-prueba-de-32-caracteres-o-mas")
 tok:=emailMktUnsubToken(22,19);tenant,subscriber,valid:=emailMktVerifyUnsub(tok);if !valid||tenant!=22||subscriber!=19{t.Fatal("firma falló")}
 if _,_,valid=emailMktVerifyUnsub(tok+"x");valid{t.Fatal("firma manipulada aceptada")}
 if _,_,valid=emailMktVerifyUnsub("invalid");valid{t.Fatal("token inválido aceptado")}
}
func TestEmailMktDisabledByDefault(t *testing.T){
 for _,k:=range []string{"WORKTIC_EMAIL_P1_ENABLED","WORKTIC_EMAIL_MARKETING_P1_ENABLED","WORKTIC_EMAIL_MARKETING_P1_SEND_ENABLED"}{t.Setenv(k,"false")}
 if emailMktSendingReady(){t.Fatal("envío activado por defecto")}
}
