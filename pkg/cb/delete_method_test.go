package cb

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDelete_RejectsGET(t *testing.T) {
	e := newAuthEngine(t)
	store := NewMemoryStore()
	_ = store.Create(nil, map[string]interface{}{"id": "1", "name": "Alice"})
	ctrl := &Controller{Title: "Staff", Table: "staff", PrimaryKey: "id", DataProvider: store, BasePath: "/admin/staff"}
	ctrl.AddCol("ID", "id", ColText, false, true).AddCol("Name", "name", ColText, true, true)
	e.Register(ctrl)

	for _, m := range []string{http.MethodGet, http.MethodHead} {
		rec := httptest.NewRecorder()
		ctrl.handleDelete(rec, httptest.NewRequest(m, "/admin/staff/delete/1", nil), "1")
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: %d, harus 405", m, rec.Code)
		}
	}
	if rows, _ := store.FindAll(nil); len(rows) != 1 {
		t.Fatalf("data terhapus oleh GET: %d", len(rows))
	}
	rec := httptest.NewRecorder()
	ctrl.handleDelete(rec, httptest.NewRequest(http.MethodPost, "/admin/staff/delete/1", nil), "1")
	if rec.Code != http.StatusSeeOther {
		t.Errorf("POST: %d", rec.Code)
	}
	if rows, _ := store.FindAll(nil); len(rows) != 0 {
		t.Errorf("POST harus menghapus: %d", len(rows))
	}
}
