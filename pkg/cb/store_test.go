package cb

import (
	"context"
	"testing"
)

func TestMemoryStoreCRUD(t *testing.T) {
	initial := []map[string]interface{}{
		{"id": "1", "name": "Test Item 1", "status": "Active"},
		{"id": "2", "name": "Test Item 2", "status": "Inactive"},
	}

	store := NewMemoryStore(initial...)
	ctx := &Context{Ctx: context.Background()}

	// 1. FindAll
	rows, err := store.FindAll(ctx)
	if err != nil {
		t.Fatalf("FindAll failed: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// 2. FindByID
	row, err := store.FindByID(ctx, "1")
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if row["name"] != "Test Item 1" {
		t.Fatalf("expected 'Test Item 1', got '%v'", row["name"])
	}

	// 3. Create
	newItem := map[string]interface{}{
		"name":   "New Item",
		"status": "Pending",
	}
	err = store.Create(ctx, newItem)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if newItem["id"] != "3" {
		t.Fatalf("expected auto-allocated id 3, got '%v'", newItem["id"])
	}

	// 4. Update
	err = store.Update(ctx, "3", map[string]interface{}{
		"name":   "Updated Item",
		"status": "Active",
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	updated, err := store.FindByID(ctx, "3")
	if err != nil {
		t.Fatalf("FindByID after update failed: %v", err)
	}
	if updated["name"] != "Updated Item" {
		t.Fatalf("expected 'Updated Item', got '%v'", updated["name"])
	}

	// 5. Delete
	err = store.Delete(ctx, "3")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err = store.FindByID(ctx, "3")
	if err == nil {
		t.Fatalf("expected error finding deleted item, got nil")
	}
}
