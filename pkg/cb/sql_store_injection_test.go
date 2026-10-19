package cb

import (
	"database/sql"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

// Regresi P0-1: key data yang berbentuk ekspresi tidak boleh disambung ke SQL sebagai nama kolom.
func TestSQLStore_RejectsInjectedColumnNames(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewSQLStore(db, "notes", "id").SetDriver("sqlite3")
	if err := store.AutoMigrate(Field{Name: "title", Type: InputText}); err != nil {
		t.Fatal(err)
	}
	ctx := &Context{}
	if err := store.Create(ctx, map[string]interface{}{"title": "a"}); err != nil {
		t.Fatalf("insert normal: %v", err)
	}
	bad := []string{"title = (SELECT 1), id", "title) VALUES (1); --", "ti tle", "title;", "1title", `"title"`}
	for _, k := range bad {
		if err := store.Create(ctx, map[string]interface{}{"title": "x", k: "y"}); err == nil || !strings.Contains(err.Error(), "nama kolom tidak valid") {
			t.Errorf("Create key %q harus ditolak, got %v", k, err)
		}
		if err := store.Update(ctx, "1", map[string]interface{}{k: "y"}); err == nil || !strings.Contains(err.Error(), "nama kolom tidak valid") {
			t.Errorf("Update key %q harus ditolak, got %v", k, err)
		}
	}
	rec, err := store.FindByID(ctx, "1")
	if err != nil || rec["title"] != "a" {
		t.Fatalf("data tidak boleh berubah: %v %v", rec, err)
	}
}
