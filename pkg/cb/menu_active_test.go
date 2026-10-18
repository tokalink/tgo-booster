package cb

import (
	"net/http/httptest"
	"testing"
)

func TestKeepMostSpecificActive(t *testing.T) {
	devices := &MenuItem{ID: "devices", Type: "url", Path: "/admin/wa", IsActive: true}
	keys := &MenuItem{ID: "keys", Type: "url", Path: "/admin/wa/keys", IsActive: true}
	other := &MenuItem{ID: "other", Type: "url", Path: "/admin/x"}
	drop := &MenuItem{ID: "drop", Type: "dropdown", IsActive: true, IsOpen: true, Children: []*MenuItem{
		{ID: "sub", Type: "url", Path: "/admin/wa/x", IsActive: false},
	}}
	items := []*MenuItem{{ID: "g", Type: "header", Children: []*MenuItem{devices, keys, other, drop}}}

	keepMostSpecificActive(items)

	if devices.IsActive {
		t.Error("/admin/wa tidak boleh aktif saat /admin/wa/keys cocok")
	}
	if !keys.IsActive {
		t.Error("/admin/wa/keys harus tetap aktif")
	}
	if drop.IsActive || drop.IsOpen {
		t.Error("dropdown tanpa anak aktif harus tertutup")
	}

	// Satu-satunya kecocokan tetap aktif.
	only := &MenuItem{ID: "only", Type: "url", Path: "/admin/wa", IsActive: true}
	keepMostSpecificActive([]*MenuItem{only})
	if !only.IsActive {
		t.Error("kecocokan tunggal harus tetap aktif")
	}
}

func TestWriteTrackerSkipsLayout(t *testing.T) {
	rec := httptest.NewRecorder()
	tw := &writeTracker{ResponseWriter: rec}
	if tw.wrote {
		t.Fatal("belum menulis")
	}
	tw.Header().Set("Content-Type", "application/json")
	if tw.wrote {
		t.Fatal("hanya mengatur header tidak dihitung menulis")
	}
	_, _ = tw.Write([]byte(`{"url":"x"}`))
	if !tw.wrote || rec.Body.String() != `{"url":"x"}` {
		t.Fatalf("tracker: wrote=%v body=%q", tw.wrote, rec.Body.String())
	}
}

func TestExternalTokenValidator(t *testing.T) {
	e := &Engine{}
	e.ExternalTokenValidator = func(raw, scope string) (*APIToken, bool, string, bool) {
		if len(raw) < 4 || raw[:4] != "axk_" {
			return nil, false, "", false
		}
		if raw == "axk_ok" {
			return &APIToken{ID: "ext", Name: "Kunci", Status: "active", Scopes: []string{"*"}}, true, "", true
		}
		return nil, false, "Forbidden: kunci tanpa akses", true
	}
	if tok, ok, _ := e.ValidateAPIToken("axk_ok", "read:products"); !ok || tok.Name != "Kunci" {
		t.Fatal("kunci eksternal yang sah harus diterima")
	}
	if _, ok, msg := e.ValidateAPIToken("axk_bad", "read:products"); ok || msg == "" {
		t.Fatal("kunci eksternal yang ditolak harus gagal dengan pesan")
	}
	if _, ok, msg := e.ValidateAPIToken("tgo_live_x", "read:products"); ok || msg != "Invalid or unrecognized API token" {
		t.Fatalf("token biasa tetap diperiksa Booster: %v %q", ok, msg)
	}
}
