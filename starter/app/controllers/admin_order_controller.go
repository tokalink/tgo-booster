package controllers

import (
	"strings"

	"github.com/tokalink/tgo-booster"
	"github.com/tokalink/tgo-booster/starter/app/models"
)

// NewAdminOrderController builds the Orders resource controller
func NewAdminOrderController() *booster.Controller {
	ctrl := &booster.Controller{
		Title:        "Orders",
		Table:        "orders",
		Icon:         `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><rect width="20" height="14" x="2" y="7" rx="2"/><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/></svg>`,
		DataProvider: models.OrderRepo,
	}

	// 1. Data Grid Columns
	ctrl.
		AddCol("Order ID", "id", booster.ColText, false, true).
		AddCol("Customer", "customer_name", booster.ColText, true, true).
		AddCol("Total Amount", "total_amount", booster.ColMoney, false, true).
		AddCol("Status", "status", booster.ColBadge, false, false)

	// 2. Form Input Fields
	ctrl.
		AddLOV("Customer", "customer_name", "customers,name", true, "Klik Cari untuk memilih customer...").
		AddForm("Total Amount", "total_amount", booster.InputMoney, true, "Rp 0").
		AddSelect("Order Status", "status", true, "Select status...",
			booster.Option{Value: "Processing", Label: "Processing"},
			booster.Option{Value: "In Transit", Label: "In Transit"},
			booster.Option{Value: "Completed", Label: "Completed"},
		)

	// 3. Lifecycle Hooks
	ctrl.HookBeforeAdd = func(c *booster.Context, data map[string]interface{}) error {
		if status, ok := data["status"]; !ok || status == "" {
			data["status"] = "Processing"
		}
		if amt, ok := data["total_amount"].(string); ok && amt != "" && !strings.HasPrefix(amt, "Rp") {
			data["total_amount"] = "Rp " + amt
		}
		return nil
	}

	return ctrl
}
