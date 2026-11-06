package main

import (
    "bytes"
    "encoding/json"
    "strings"
    "testing"
)

func TestMetaSocialB2ConfigID(t *testing.T) {
    for _, tc := range []struct{ id string; ok bool } {
        {"948728128081443", true}, {"", false}, {"abc", false}, {"948728128081443&scope=admin", false}, {"12345678", true},
    } {
        if isMetaSocialConfigID(tc.id) != tc.ok {t.Fatalf("config id %q unexpected result", tc.id)}
    }
}

func TestMetaSocialB2GrantedScopesAndTasks(t *testing.T) {
    granted := metaGrantedSocialPermissions([]metaSocialPermission{
        {"pages_show_list", "granted"},
        {"pages_read_engagement", "granted"},
        {"pages_manage_posts", "declined"},
        {"pages_manage_metadata", "granted"},
    })
    scopes := metaEffectiveSocialScopes("facebook", granted, []string{"ANALYZE", "MANAGE"})
    for _, req := range []string{"pages_show_list", "pages_read_engagement", "pages_manage_metadata"} {
        if !metaHasScope(scopes, req) {t.Errorf("missing granted scope %s in %s", req, scopes)}
    }
    if metaHasScope(scopes, "pages_manage_posts") || metaHasScope(scopes, "leads_retrieval") {
        t.Errorf("unapproved scopes were claimed: %s", scopes)
    }
    granted["pages_manage_posts"] = true
    scopes = metaEffectiveSocialScopes("facebook", granted, []string{"ANALYZE"})
    if metaHasScope(scopes, "pages_manage_posts") {t.Error("claimed post permission without page task")}
    scopes = metaEffectiveSocialScopes("facebook", granted, []string{"CREATE_CONTENT"})
    if !metaHasScope(scopes, "pages_manage_posts") {t.Error("missing valid post scope")}
    if metaHasScope(scopes, "pages_manage_metadata") {t.Error("claimed metadata without MANAGE page task")}
}

func TestMetaSocialB2MissingInstagramPermissions(t *testing.T) {
    granted := map[string]bool{"pages_read_engagement":true, "instagram_basic":false}
    if scopes := metaEffectiveSocialScopes("instagram", granted, []string{"MANAGE"}); scopes != "" {
        t.Fatalf("unexpected Instagram scopes: %s", scopes)
    }
}

func TestMetaSocialB2PageFieldsDecoding(t *testing.T) {
    var p metaSocialPage
    if err := json.Unmarshal([]byte(`{"id":"123","name":"Pagina A","access_token":"PAGE_TOKEN","tasks":["MANAGE"]}`), &p); err != nil {t.Fatal(err)}
    if p.ID != "123" || p.Name != "Pagina A" || p.AccessToken != "PAGE_TOKEN" {t.Fatalf("unexpected fields: %+v", p)}
}

func TestMetaSocialB2TemplateEscapesPageNames(t *testing.T) {
    var out bytes.Buffer
    err := metaSocialSelectTemplate.Execute(&out, struct{
        Platform string
        State string
        Action string
        Pages []metaSocialPage
    }{"facebook", "state", "https://example.com/callback", []metaSocialPage{{ID:"123",Name:`<script>alert("x")</script>`}}})
    if err != nil {t.Fatal(err)}
    if strings.Contains(out.String(), `<script>`) || !strings.Contains(out.String(), "&lt;script&gt;") {t.Fatal("template did not escape page name")}
}
