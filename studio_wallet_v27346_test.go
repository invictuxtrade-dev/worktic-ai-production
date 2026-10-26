package main
import "testing"
func TestStudioWalletLabelsV27346(t *testing.T) {
    for k,v:=range map[string]string{"image":"Imagen IA","video":"Video IA","narration":"Narración IA","copy":"Texto IA","storyboard":"Guion IA","weird":"Generación IA"} {
        if got:=studioWalletLabelV27346(k);got!=v {t.Errorf("label %s = %q, expected %q",k,got,v)}
    }
}
