package cb

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tokalink/tgo/pkg/transport/connect"
)

// permissiveLookupProvider wraps a DataProvider and falls back to the first record on FindByID
// to emulate MySQL's silent numeric prefix coercion (e.g. "1\"><script>" -> 1).
type permissiveLookupProvider struct {
	DataProvider
}

func (p *permissiveLookupProvider) FindByID(ctx *Context, id string) (map[string]interface{}, error) {
	if rec, err := p.DataProvider.FindByID(ctx, id); err == nil && rec != nil {
		return rec, nil
	}
	rows, err := p.DataProvider.FindAll(ctx)
	if err != nil || len(rows) == 0 {
		return nil, errors.New("not found")
	}
	return rows[0], nil
}

func TestWave2_DetailHTMLEscapesIDTitleAndLabels(t *testing.T) {
	t.Chdir(t.TempDir())
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	e := newAuthEngine(t)
	e.SetDB(db)

	sqlStore := NewSQLStore(db, "gadgets", "id").SetDriver("sqlite3")
	ctrl := &Controller{
		Title:        `Gadgets <script>evilTitle()</script>`,
		Table:        "gadgets",
		PrimaryKey:   "id",
		DataProvider: &permissiveLookupProvider{DataProvider: sqlStore},
	}
	ctrl.AddCol(`Name <img src=x onerror=alert(1)>`, "name", ColText, true, true)
	ctrl.AddForm("Name", "name", InputText, true, "")
	e.Register(ctrl)
	if err := sqlStore.AutoMigrate(ctrl.Forms...); err != nil {
		t.Fatal(err)
	}
	if err := sqlStore.Create(nil, map[string]interface{}{"name": "Safe Gadget"}); err != nil {
		t.Fatal(err)
	}

	srv := connect.NewServer()
	e.Mount(srv)

	adminUser := &User{ID: "1", Name: "Superadmin", Email: "admin@tgo.io", PrivilegeID: "1", RoleName: "Superadmin"}
	req := sessionReq(e, adminUser)
	req.Method = http.MethodGet
	req.URL, _ = url.Parse("http://example.com/admin/gadgets/detail/1%22%3E%3Cscript%3Ealert(9)%3C%2Fscript%3E")

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on detail, got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, rawForbidden := range []string{
		`<script>alert(9)`,
		`<script>evilTitle()`,
		`<img src=x onerror=alert(1)>`,
	} {
		if strings.Contains(body, rawForbidden) {
			t.Fatalf("detail HTML contains unescaped XSS payload %q:\n%s", rawForbidden, body)
		}
	}
	if !strings.Contains(body, `/admin/gadgets/edit/1"`) {
		t.Fatalf("expected edit link to use record primary key /admin/gadgets/edit/1, got:\n%s", body)
	}
}

func TestWave2_EditFormWYSIWYGSanitizedOnRenderAndSave(t *testing.T) {
	t.Chdir(t.TempDir())
	e := newAuthEngine(t)

	store := NewMemoryStore(map[string]interface{}{
		"id":      "1",
		"title":   "Post 1",
		"content": `<p>Existing</p><img src=x onerror=alert(1)>`,
	})
	ctrl := &Controller{
		Title:        "Articles",
		Table:        "articles",
		PrimaryKey:   "id",
		DataProvider: store,
	}
	ctrl.AddCol("ID", "id", ColText, false, true).
		AddCol("Title", "title", ColText, true, true)
	ctrl.AddForm("Title", "title", InputText, true, "").
		AddForm("Content", "content", InputWYSIWYG, false, "")
	e.Register(ctrl)

	srv := connect.NewServer()
	e.Mount(srv)
	adminUser := &User{ID: "1", Name: "Superadmin", Email: "admin@tgo.io", PrivilegeID: "1", RoleName: "Superadmin"}

	// 1. GET /admin/articles/edit/1 must not contain onerror
	reqGet := sessionReq(e, adminUser)
	reqGet.Method = http.MethodGet
	reqGet.URL, _ = url.Parse("http://example.com/admin/articles/edit/1")
	recGet := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recGet, reqGet)

	if recGet.Code != http.StatusOK {
		t.Fatalf("expected 200 on edit page, got %d", recGet.Code)
	}
	// Check the wysiwyg editor div specifically and full HTML:
	// Note: textarea also renders {{.Value}} which html/template auto-escapes; the wysiwyg div uses safeRichHTML.
	if strings.Contains(recGet.Body.String(), `<img src=x onerror=alert(1)>`) {
		t.Fatalf("edit HTML should not contain raw unescaped <img src=x onerror=alert(1)>")
	}
	editorIdx := strings.Index(recGet.Body.String(), `id="wysiwyg_editor_content"`)
	if editorIdx < 0 {
		t.Fatalf("wysiwyg_editor_content not found in edit HTML")
	}
	editorLine := recGet.Body.String()[editorIdx : editorIdx+200]
	if strings.Contains(strings.ToLower(editorLine), "onerror") {
		t.Fatalf("wysiwyg editor div still contains onerror: %s", editorLine)
	}

	// 2. POST /admin/articles/edit/1 sanitizes wysiwyg before saving
	form := url.Values{}
	form.Set("title", "Post 1 Updated")
	form.Set("content", `<p>Safe body</p><img src=x onerror=alert(1)><script>evil()</script>`)
	reqPost := sessionReq(e, adminUser)
	reqPost.Method = http.MethodPost
	reqPost.URL, _ = url.Parse("http://example.com/admin/articles/edit/1")
	reqPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPost.Body = io.NopCloser(strings.NewReader(form.Encode()))
	recPost := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recPost, reqPost)

	if recPost.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on edit submit, got %d", recPost.Code)
	}
	saved, err := store.FindByID(nil, "1")
	if err != nil {
		t.Fatal(err)
	}
	savedContent := fmt.Sprint(saved["content"])
	if strings.Contains(strings.ToLower(savedContent), "onerror") || strings.Contains(strings.ToLower(savedContent), "script") {
		t.Fatalf("saved wysiwyg content still contains dangerous markup: %q", savedContent)
	}
	if !strings.Contains(savedContent, "<p>Safe body</p>") {
		t.Fatalf("expected <p>Safe body</p> to be preserved, got %q", savedContent)
	}
}

func TestWave2_LOVAndTemplatesNoDangerousSinks(t *testing.T) {
	t.Chdir(t.TempDir())

	formBytes, err := boosterFS.ReadFile("templates/pages/form.html")
	if err != nil {
		t.Fatal(err)
	}
	formSrc := string(formBytes)
	if strings.Contains(formSrc, "tr.innerHTML") {
		t.Fatal("templates/pages/form.html must not contain tr.innerHTML")
	}
	if strings.Contains(formSrc, `onclick="selectLOVItem(`) {
		t.Fatal(`templates/pages/form.html must not contain onclick="selectLOVItem(`)
	}

	tableBytes, err := boosterFS.ReadFile("templates/pages/table.html")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(tableBytes), "previewTable.innerHTML") {
		t.Fatal("templates/pages/table.html must not use previewTable.innerHTML for CSV preview")
	}
}

func TestWave2_PagesStudioSuperadminOnlyAndCSPSandboxAndSchemaJSON(t *testing.T) {
	t.Chdir(t.TempDir())
	e := newAuthEngine(t)
	srv := connect.NewServer()
	e.Mount(srv)
	h := srv.Handler()

	nonSuper := &User{
		ID:          "2",
		Name:        "Ops Manager",
		Email:       "manager@tgo.io",
		PrivilegeID: "2",
		RoleName:    "Operations Manager",
	}
	superAdmin := &User{
		ID:          "1",
		Name:        "Super Admin",
		Email:       "admin@tgo.io",
		PrivilegeID: "1",
		RoleName:    "Superadmin",
	}

	// 1. Non-superadmin -> 403 on /admin/pages/save, /admin/pages, /admin/pages/preview?id=1
	for _, tc := range []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/admin/pages/save", `{"title":"Hack","slug":"hack"}`},
		{http.MethodGet, "/admin/pages", ""},
		{http.MethodGet, "/admin/pages/preview?id=1", ""},
		{http.MethodGet, "/admin/pages/api/page?id=1", ""},
		{http.MethodPost, "/admin/pages/delete", `{"id":"1"}`},
		{http.MethodPost, "/admin/pages/generate-ai", `{"prompt":"hello"}`},
	} {
		req := sessionReq(e, nonSuper)
		req.Method = tc.method
		req.URL, _ = url.Parse("http://example.com" + tc.path)
		if tc.body != "" {
			req.Header.Set("Content-Type", "application/json")
			req.Body = io.NopCloser(strings.NewReader(tc.body))
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("non-superadmin %s %s: expected 403, got %d", tc.method, tc.path, rec.Code)
		}
	}

	// 2. Superadmin preview includes CSP sandbox header
	reqPrev := sessionReq(e, superAdmin)
	reqPrev.Method = http.MethodGet
	reqPrev.URL, _ = url.Parse("http://example.com/admin/pages/preview?id=1")
	recPrev := httptest.NewRecorder()
	h.ServeHTTP(recPrev, reqPrev)
	if recPrev.Code != http.StatusOK {
		t.Fatalf("superadmin preview expected 200, got %d", recPrev.Code)
	}
	csp := recPrev.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "sandbox") {
		t.Fatalf("expected Content-Security-Policy sandbox header on preview, got %q", csp)
	}

	// 3. SchemaJSON with </script> breakout attempt is escaped; invalid JSON is not rendered
	evilPage := &Page{
		ID:         "901",
		Title:      "Schema Escape Test",
		Slug:       "schema-escape",
		RouteURL:   "/schema-escape",
		Status:     "published",
		SchemaJSON: `{"@context":"https://schema.org","name":"</script><script>alert(1)</script>"}`,
		Content:    "<p>ok</p>",
	}
	renderedEvil := e.RenderPublicPageHTML(evilPage, false, "example.com")
	if strings.Contains(renderedEvil, "</script><script>alert(1)</script>") {
		t.Fatalf("SchemaJSON broke out of </script>:\n%s", renderedEvil)
	}
	if !strings.Contains(renderedEvil, `\u003c/script\u003e`) {
		t.Fatalf("expected json.HTMLEscape output \\u003c/script\\u003e in SchemaJSON:\n%s", renderedEvil)
	}

	invalidPage := &Page{
		ID:         "902",
		Title:      "Invalid Schema Test",
		Slug:       "invalid-schema",
		RouteURL:   "/invalid-schema",
		Status:     "published",
		SchemaJSON: `{not valid json </script><script>alert(1)</script>`,
		Content:    "<p>ok</p>",
	}
	renderedInvalid := e.RenderPublicPageHTML(invalidPage, false, "example.com")
	if strings.Contains(renderedInvalid, "application/ld+json") || strings.Contains(renderedInvalid, "not valid json") {
		t.Fatalf("invalid SchemaJSON should not be rendered:\n%s", renderedInvalid)
	}
}

func TestWave2_EditFormFormattingPasswordAndCheckbox(t *testing.T) {
	t.Chdir(t.TempDir())
	e := newAuthEngine(t)

	dt := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	dttm := time.Date(2026, 10, 20, 10, 30, 45, 0, time.UTC)

	store := NewMemoryStore(map[string]interface{}{
		"id":          "1",
		"birth_date":  dt,
		"meeting_at":  dttm,
		"notes_bytes": []byte("DecodedFromBytes"),
		"password":    "HashedSecret#99",
		"cb_bool":     true,
		"cb_active":   "Active",
		"cb_num":      1,
		"cb_opts":     "ENABLED_VAL",
		"cb_default":  "1",
	})

	ctrl := &Controller{
		Title:        "Profiles",
		Table:        "profiles",
		PrimaryKey:   "id",
		DataProvider: store,
	}
	ctrl.AddForm("Birth Date", "birth_date", InputDate, false, "").
		AddForm("Meeting At", "meeting_at", InputDateTime, false, "").
		AddForm("Notes", "notes_bytes", InputText, false, "").
		AddForm("Password", "password", InputPassword, false, "").
		AddForm("Bool Flag", "cb_bool", InputCheckbox, false, "").
		AddForm("Active Flag", "cb_active", InputCheckbox, false, "").
		AddForm("Num Flag", "cb_num", InputCheckbox, false, "").
		AddForm("Default Flag", "cb_default", InputCheckbox, false, "")
	ctrl.Forms = append(ctrl.Forms, Field{
		Label: "Options Flag",
		Name:  "cb_opts",
		Type:  InputCheckbox,
		Options: []Option{
			{Value: "ENABLED_VAL", Label: "Enabled"},
			{Value: "DISABLED_VAL", Label: "Disabled"},
		},
	})
	e.Register(ctrl)

	srv := connect.NewServer()
	e.Mount(srv)
	superAdmin := &User{ID: "1", Name: "Super Admin", Email: "admin@tgo.io", PrivilegeID: "1", RoleName: "Superadmin"}

	// 1. GET /admin/profiles/edit/1 formats time.Time date/datetime and []byte
	reqEdit := sessionReq(e, superAdmin)
	reqEdit.Method = http.MethodGet
	reqEdit.URL, _ = url.Parse("http://example.com/admin/profiles/edit/1")
	recEdit := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recEdit, reqEdit)
	if recEdit.Code != http.StatusOK {
		t.Fatalf("expected 200 on edit HTML, got %d", recEdit.Code)
	}
	editHTML := recEdit.Body.String()
	for _, want := range []string{
		`value="2026-10-20"`,
		`value="2026-10-20T10:30"`,
		`value="DecodedFromBytes"`,
	} {
		if !strings.Contains(editHTML, want) {
			t.Fatalf("edit HTML missing %s:\n%s", want, editHTML)
		}
	}

	// 2. POST /admin/profiles/edit/1 with empty password, empty birth_date, datetime with T, and all checkboxes unchecked
	form := url.Values{}
	form.Set("birth_date", "")
	form.Set("meeting_at", "2026-10-20T10:30")
	form.Set("notes_bytes", "UpdatedText")
	form.Set("password", "") // empty password must not overwrite existing password
	// Note: checkboxes (cb_bool, cb_active, cb_num, cb_opts, cb_default) are omitted (unchecked)

	reqSave := sessionReq(e, superAdmin)
	reqSave.Method = http.MethodPost
	reqSave.URL, _ = url.Parse("http://example.com/admin/profiles/edit/1")
	reqSave.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqSave.Body = io.NopCloser(strings.NewReader(form.Encode()))
	recSave := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recSave, reqSave)

	if recSave.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on edit submit, got %d: %s", recSave.Code, recSave.Body.String())
	}

	updated, err := store.FindByID(nil, "1")
	if err != nil {
		t.Fatal(err)
	}
	if updated["password"] != "HashedSecret#99" {
		t.Fatalf("expected password to remain 'HashedSecret#99', got %v", updated["password"])
	}
	if updated["birth_date"] != nil {
		t.Fatalf("expected empty birth_date to be stored as nil, got %#v", updated["birth_date"])
	}
	if updated["meeting_at"] != "2026-10-20 10:30:00" {
		t.Fatalf("expected meeting_at normalized to '2026-10-20 10:30:00', got %#v", updated["meeting_at"])
	}
	if updated["cb_bool"] != false {
		t.Fatalf("expected unchecked bool checkbox to be false (bool), got %#v", updated["cb_bool"])
	}
	if updated["cb_active"] != "Inactive" {
		t.Fatalf("expected unchecked Active checkbox to be 'Inactive', got %#v", updated["cb_active"])
	}
	if updated["cb_num"] != 0 {
		t.Fatalf("expected unchecked numeric checkbox to be 0, got %#v", updated["cb_num"])
	}
	if updated["cb_opts"] != "DISABLED_VAL" {
		t.Fatalf("expected unchecked Options checkbox to be 'DISABLED_VAL', got %#v", updated["cb_opts"])
	}
	if updated["cb_default"] != "0" {
		t.Fatalf("expected unchecked default checkbox to be '0', got %#v", updated["cb_default"])
	}
}

func TestWave2_MassAssignmentFilterWritable(t *testing.T) {
	t.Chdir(t.TempDir())
	e := newAuthEngine(t)

	store := NewMemoryStore()
	ctrl := &Controller{
		Title:        "Members",
		Table:        "members",
		PrimaryKey:   "id",
		DataProvider: store,
	}
	ctrl.AddForm("Name", "name", InputText, true, "")
	ctrl.Forms = append(ctrl.Forms, Field{
		Label:    "ReadOnlyField",
		Name:     "ro_field",
		Type:     InputText,
		ReadOnly: true,
	})
	ctrl.AllowFields("allowed_extra")
	ctrl.HookBeforeAdd = func(ctx *Context, data map[string]interface{}) error {
		data["hook_injected"] = "by_hook"
		return nil
	}
	e.Register(ctrl)

	srv := connect.NewServer()
	e.Mount(srv)
	superAdmin := &User{ID: "1", Name: "Super Admin", Email: "admin@tgo.io", PrivilegeID: "1", RoleName: "Superadmin"}

	// 1. Create with foreign fields (role, is_admin, tenant_id, ro_field) + allowed_extra
	form := url.Values{}
	form.Set("name", "Budi")
	form.Set("role", "Superadmin")
	form.Set("is_admin", "true")
	form.Set("tenant_id", "evil-tenant")
	form.Set("ro_field", "tampered")
	form.Set("allowed_extra", "permitted-val")

	reqAdd := sessionReq(e, superAdmin)
	reqAdd.Method = http.MethodPost
	reqAdd.URL, _ = url.Parse("http://example.com/admin/members/add")
	reqAdd.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqAdd.Body = io.NopCloser(strings.NewReader(form.Encode()))
	recAdd := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recAdd, reqAdd)
	if recAdd.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on add, got %d", recAdd.Code)
	}

	rows, _ := store.FindAll(nil)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	r0 := rows[0]
	for _, forbidden := range []string{"role", "is_admin", "tenant_id", "ro_field"} {
		if _, exists := r0[forbidden]; exists {
			t.Fatalf("mass-assigned field %q must not be stored, got %#v", forbidden, r0)
		}
	}
	if r0["allowed_extra"] != "permitted-val" {
		t.Fatalf("expected AllowFields 'allowed_extra' to be saved, got %#v", r0["allowed_extra"])
	}
	if r0["hook_injected"] != "by_hook" {
		t.Fatalf("expected HookBeforeAdd field 'hook_injected' to be saved, got %#v", r0["hook_injected"])
	}

	// 2. Edit with foreign fields
	editForm := url.Values{}
	editForm.Set("name", "Budi Edited")
	editForm.Set("is_admin", "1")
	editForm.Set("role", "root")
	reqEdit := sessionReq(e, superAdmin)
	reqEdit.Method = http.MethodPost
	reqEdit.URL, _ = url.Parse("http://example.com/admin/members/edit/" + fmt.Sprint(r0["id"]))
	reqEdit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqEdit.Body = io.NopCloser(strings.NewReader(editForm.Encode()))
	recEdit := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recEdit, reqEdit)
	if recEdit.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on edit, got %d", recEdit.Code)
	}
	rEdited, _ := store.FindByID(nil, fmt.Sprint(r0["id"]))
	if _, exists := rEdited["is_admin"]; exists {
		t.Fatalf("mass-assigned field is_admin must not be stored on edit")
	}

	// 3. CSV import strips foreign columns
	csvPayload := `{"rows":[{"name":"Siti","is_admin":"true","role":"Superadmin"}]}`
	reqCSV := sessionReq(e, superAdmin)
	reqCSV.Method = http.MethodPost
	reqCSV.URL, _ = url.Parse("http://example.com/admin/members/import-csv")
	reqCSV.Header.Set("Content-Type", "application/json")
	reqCSV.Body = io.NopCloser(strings.NewReader(csvPayload))
	recCSV := httptest.NewRecorder()
	srv.Handler().ServeHTTP(recCSV, reqCSV)
	if recCSV.Code != http.StatusOK {
		t.Fatalf("expected 200 on import-csv, got %d: %s", recCSV.Code, recCSV.Body.String())
	}
	allRows, _ := store.FindAll(nil)
	if len(allRows) != 2 {
		t.Fatalf("expected 2 rows after CSV import, got %d", len(allRows))
	}
	for _, row := range allRows {
		if row["name"] == "Siti" {
			if _, exists := row["is_admin"]; exists {
				t.Fatalf("CSV import must strip foreign column is_admin: %#v", row)
			}
			if _, exists := row["role"]; exists {
				t.Fatalf("CSV import must strip foreign column role: %#v", row)
			}
		}
	}
}

func TestWave2_PublicREST_OptInSizeLimitAndHooks(t *testing.T) {
	t.Chdir(t.TempDir())
	e := newAuthEngine(t)

	privStore := NewMemoryStore(map[string]interface{}{"id": "1", "name": "Secret Item"})
	privCtrl := &Controller{
		Title:        "Private Items",
		Table:        "private_items",
		PrimaryKey:   "id",
		DataProvider: privStore,
	}
	privCtrl.AddForm("Name", "name", InputText, true, "")
	e.Register(privCtrl) // PublicAPI is NOT enabled

	pubStore := NewMemoryStore(map[string]interface{}{"id": "1", "name": "Public Item 1"})
	pubCtrl := &Controller{
		Title:        "Public Items",
		Table:        "public_items",
		PrimaryKey:   "id",
		DataProvider: pubStore,
	}
	pubCtrl.AddForm("Name", "name", InputText, true, "").EnablePublicAPI()

	deleteHookCalled := false
	pubCtrl.HookBeforeAdd = func(ctx *Context, data map[string]interface{}) error {
		if fmt.Sprint(data["name"]) == "REJECT_ME" {
			return errors.New("rejected by HookBeforeAdd")
		}
		return nil
	}
	pubCtrl.HookBeforeDelete = func(ctx *Context, data map[string]interface{}) error {
		deleteHookCalled = true
		if fmt.Sprint(data["id"]) == "protected" {
			return errors.New("cannot delete protected item")
		}
		return nil
	}
	e.Register(pubCtrl)

	srv := connect.NewServer()
	e.Mount(srv)
	h := srv.Handler()

	// Create valid token with read & write scopes
	tokSecret := GenerateSecureAPIToken("live")
	if err := e.SaveAPIToken(&APIToken{
		ID:          "tok-1",
		Name:        "Test Token",
		Token:       tokSecret,
		Environment: "production",
		Scopes:      []string{"read:all", "write:all"},
		Status:      "active",
	}); err != nil {
		t.Fatal(err)
	}

	// 1. Controller without EnablePublicAPI -> 404 on /api/v1/private_items
	reqPriv := httptest.NewRequest(http.MethodGet, "/api/v1/private_items", nil)
	reqPriv.Header.Set("Authorization", "Bearer "+tokSecret)
	recPriv := httptest.NewRecorder()
	h.ServeHTTP(recPriv, reqPriv)
	if recPriv.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for controller without EnablePublicAPI, got %d", recPriv.Code)
	}

	// 2. Controller with EnablePublicAPI -> 200 on /api/v1/public_items
	reqPub := httptest.NewRequest(http.MethodGet, "/api/v1/public_items", nil)
	reqPub.Header.Set("Authorization", "Bearer "+tokSecret)
	recPub := httptest.NewRecorder()
	h.ServeHTTP(recPub, reqPub)
	if recPub.Code != http.StatusOK {
		t.Fatalf("expected 200 for controller with EnablePublicAPI, got %d: %s", recPub.Code, recPub.Body.String())
	}

	// 3. Body > 1 MiB on POST -> 413
	bigJSON := `{"name":"` + strings.Repeat("x", (1<<20)+100) + `"}`
	reqBig := httptest.NewRequest(http.MethodPost, "/api/v1/public_items", strings.NewReader(bigJSON))
	reqBig.Header.Set("Authorization", "Bearer "+tokSecret)
	reqBig.Header.Set("Content-Type", "application/json")
	recBig := httptest.NewRecorder()
	h.ServeHTTP(recBig, reqBig)
	if recBig.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for REST POST > 1 MiB, got %d: %s", recBig.Code, recBig.Body.String())
	}

	// 4. HookBeforeAdd returning error -> 400 and not saved
	reqReject := httptest.NewRequest(http.MethodPost, "/api/v1/public_items", strings.NewReader(`{"name":"REJECT_ME","is_admin":true}`))
	reqReject.Header.Set("Authorization", "Bearer "+tokSecret)
	reqReject.Header.Set("Content-Type", "application/json")
	recReject := httptest.NewRecorder()
	h.ServeHTTP(recReject, reqReject)
	if recReject.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when HookBeforeAdd returns error, got %d", recReject.Code)
	}
	if rows, _ := pubStore.FindAll(nil); len(rows) != 1 {
		t.Fatalf("rejected record must not be saved, count=%d", len(rows))
	}

	// 5. DELETE runs HookBeforeDelete
	reqDel := httptest.NewRequest(http.MethodDelete, "/api/v1/public_items/1", nil)
	reqDel.Header.Set("Authorization", "Bearer "+tokSecret)
	recDel := httptest.NewRecorder()
	h.ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 on REST DELETE, got %d: %s", recDel.Code, recDel.Body.String())
	}
	if !deleteHookCalled {
		t.Fatal("expected HookBeforeDelete to be executed on REST DELETE")
	}
	if rows, _ := pubStore.FindAll(nil); len(rows) != 0 {
		t.Fatalf("expected record 1 to be deleted, count=%d", len(rows))
	}
	_ = e.DeleteAPIToken("tok-1")
	time.Sleep(20 * time.Millisecond)
}

func TestWave2_BulkAction_MethodHooksAndValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	e := newAuthEngine(t)

	store := NewMemoryStore(
		map[string]interface{}{"id": "1", "name": "Protected Item"},
		map[string]interface{}{"id": "2", "name": "Deletable Item"},
	)
	ctrl := &Controller{
		Title:        "Orders",
		Table:        "orders",
		PrimaryKey:   "id",
		DataProvider: store,
	}
	ctrl.AddForm("Name", "name", InputText, true, "")
	ctrl.HookBeforeDelete = func(ctx *Context, data map[string]interface{}) error {
		if fmt.Sprint(data["id"]) == "1" {
			return errors.New("order #1 is locked")
		}
		return nil
	}
	e.Register(ctrl)

	srv := connect.NewServer()
	e.Mount(srv)
	h := srv.Handler()
	superAdmin := &User{ID: "1", Name: "Super Admin", Email: "admin@tgo.io", PrivilegeID: "1", RoleName: "Superadmin"}

	// 1. GET -> 405
	reqGet := sessionReq(e, superAdmin)
	reqGet.Method = http.MethodGet
	reqGet.URL, _ = url.Parse("http://example.com/admin/orders/bulk-action?action=delete&ids=1,2")
	recGet := httptest.NewRecorder()
	h.ServeHTTP(recGet, reqGet)
	if recGet.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405 on GET bulk-action, got %d", recGet.Code)
	}

	// 2. action=foo -> 400
	reqFoo := sessionReq(e, superAdmin)
	reqFoo.Method = http.MethodPost
	reqFoo.URL, _ = url.Parse("http://example.com/admin/orders/bulk-action")
	reqFoo.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqFoo.Body = io.NopCloser(strings.NewReader("action=foo&ids=1,2"))
	recFoo := httptest.NewRecorder()
	h.ServeHTTP(recFoo, reqFoo)
	if recFoo.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on action=foo, got %d", recFoo.Code)
	}

	// 3. POST action=delete runs HookBeforeDelete: id=1 fails hook (failed=1), id=2 deleted (count=1)
	reqBulk := sessionReq(e, superAdmin)
	reqBulk.Method = http.MethodPost
	reqBulk.URL, _ = url.Parse("http://example.com/admin/orders/bulk-action")
	reqBulk.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqBulk.Header.Set("Accept", "application/json")
	reqBulk.Body = io.NopCloser(strings.NewReader("action=delete&ids=1,2"))
	recBulk := httptest.NewRecorder()
	h.ServeHTTP(recBulk, reqBulk)
	if recBulk.Code != http.StatusOK {
		t.Fatalf("expected 200 on bulk-action, got %d: %s", recBulk.Code, recBulk.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(recBulk.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode bulk-action JSON: %v", err)
	}
	if int(resp["failed"].(float64)) != 1 || int(resp["count"].(float64)) != 1 || resp["success"] != false {
		t.Fatalf("unexpected bulk-action response: %#v", resp)
	}
	if _, err := store.FindByID(nil, "1"); err != nil {
		t.Fatal("record #1 rejected by HookBeforeDelete must not be deleted")
	}
	if _, err := store.FindByID(nil, "2"); err == nil {
		t.Fatal("record #2 allowed by HookBeforeDelete should have been deleted")
	}
}

func TestWave2_ImportCSV_MaxBytesLimit(t *testing.T) {
	t.Chdir(t.TempDir())
	e := newAuthEngine(t)

	store := NewMemoryStore()
	ctrl := &Controller{
		Title:        "Imports",
		Table:        "imports",
		PrimaryKey:   "id",
		DataProvider: store,
	}
	ctrl.AddForm("Name", "name", InputText, true, "")
	e.Register(ctrl)

	srv := connect.NewServer()
	e.Mount(srv)
	h := srv.Handler()
	superAdmin := &User{ID: "1", Name: "Super Admin", Email: "admin@tgo.io", PrivilegeID: "1", RoleName: "Superadmin"}

	// 1. Multipart body > 10 MiB -> 413
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("csv_file", "huge.csv")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("name\n"))
	_, _ = fw.Write(bytes.Repeat([]byte("a"), (10<<20)+1024))
	_ = mw.Close()

	reqMulti := sessionReq(e, superAdmin)
	reqMulti.Method = http.MethodPost
	reqMulti.URL, _ = url.Parse("http://example.com/admin/imports/import-csv")
	reqMulti.Header.Set("Content-Type", mw.FormDataContentType())
	reqMulti.Body = io.NopCloser(&buf)
	recMulti := httptest.NewRecorder()
	h.ServeHTTP(recMulti, reqMulti)
	if recMulti.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for multipart import-csv > 10 MiB, got %d: %s", recMulti.Code, recMulti.Body.String())
	}

	// 2. JSON body > 10 MiB -> 413
	hugeJSON := `{"rows":[{"name":"` + strings.Repeat("b", (10<<20)+1024) + `"}]}`
	reqJSON := sessionReq(e, superAdmin)
	reqJSON.Method = http.MethodPost
	reqJSON.URL, _ = url.Parse("http://example.com/admin/imports/import-csv")
	reqJSON.Header.Set("Content-Type", "application/json")
	reqJSON.Body = io.NopCloser(strings.NewReader(hugeJSON))
	recJSON := httptest.NewRecorder()
	h.ServeHTTP(recJSON, reqJSON)
	if recJSON.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413 for JSON import-csv > 10 MiB, got %d: %s", recJSON.Code, recJSON.Body.String())
	}
}
