package controllers

import (
	booster "github.com/tokalink/tgo-booster"
)

// NewAdminSuppliersController builds the Suppliers resource controller
func NewAdminSuppliersController() *booster.Controller {
	ctrl := &booster.Controller{
		Title:      "Suppliers",
		Table:      "suppliers",
		Icon:       `<svg><rect/></svg>`,
		PrimaryKey: "id",
		OrderBy:    "id DESC",
		DataProvider: booster.NewMemoryStore(),
	}

	// 1. Data Grid Columns (CRUDBooster Table Display)
	ctrl.
		AddCol("ID", "id", booster.ColText, false, true).
		AddCol("Supplier Name", "name", booster.ColText, true, true).
		AddCol("Status", "status", booster.ColBadge, false, false)

	// 2. Form Input Fields (CRUDBooster Form Display)
	ctrl.
		AddForm("Supplier Name", "name", booster.InputText, true, "e.g. Acme Corp").
		AddSelect("Status", "status", true, "",
			booster.Option{Value: "Active", Label: "Active"},
		)

	return ctrl
}
