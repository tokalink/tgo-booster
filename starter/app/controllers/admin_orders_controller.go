package controllers

import (
	booster "github.com/tokalink/tgo-booster"
	"github.com/tokalink/tgo-booster/starter/app/models"
)

// NewAdminOrders2Controller builds the Orders2 resource controller
func NewAdminOrders2Controller() *booster.Controller {
	ctrl := &booster.Controller{
		Title:      "Orders2",
		Table:      "orders",
		Icon:       `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" x2="12" y1="2" y2="22"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>`,
		PrimaryKey: "id",
		OrderBy:    "id DESC",
		DataProvider: booster.NewSQLStore(models.DB, "orders", "id"),
	}

	// 1. Data Grid Columns (CRUDBooster Table Display)
	ctrl.
		AddCol("ID", "id", booster.ColText, false, true).
		AddCol("Name", "name", booster.ColText, true, true).
		AddCol("Status", "status", booster.ColBadge, false, false).
		AddCol("Created At", "created_at", booster.ColDateTime, false, true)

	// 2. Form Input Fields (CRUDBooster Form Display)
	ctrl.
		AddForm("Name", "name", booster.InputText, true, "Enter name...").
		AddSelectTable("Status", "status", "Active:Active, Inactive:Inactive", true, "Select status...")

	// 3. Executive Widgets (CRUDBooster Widget Card equivalent)
	ctrl.AddWidget(booster.WidgetCard{
		Title:     "Active Records",
		Value:     "Healthy Flow",
		Trend:     "Optimal",
		TrendType: "positive",
		Color:     "blue",
		ColSpan:   4,
	})

	return ctrl
}
