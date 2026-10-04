package controllers

import (
	"strings"

	booster "github.com/tokalink/tgo-booster"
	"github.com/tokalink/tgo-booster/starter/app/models"
)

// NewAdminProductController builds the Products resource controller
func NewAdminProductController() *booster.Controller {
	ctrl := &booster.Controller{
		Title:        "Products",
		Table:        "products",
		Icon:         `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="m7.5 4.27 9 5.15"/><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/></svg>`,
		DataProvider: models.ProductRepo,
	}

	// 1. Data Grid Columns
	ctrl.
		AddCol("ID", "id", booster.ColText, false, true).
		AddCol("Product Name", "name", booster.ColText, true, true).
		AddCol("Price", "price", booster.ColMoney, false, true).
		AddCol("Stock", "stock", booster.ColNumber, false, true).
		AddCol("Status", "status", booster.ColBadge, false, false)

	// 2. Form Input Fields
	ctrl.
		AddForm("Product Name", "name", booster.InputText, true, "Enter product name...").
		AddForm("Price", "price", booster.InputMoney, true, "Rp 0").
		AddForm("Stock", "stock", booster.InputNumber, true, "10").
		AddSelect("Status", "status", true, "Select status...",
			booster.Option{Value: "Active", Label: "Active"},
			booster.Option{Value: "Processing", Label: "Processing"},
			booster.Option{Value: "Inactive", Label: "Inactive"},
		).
		AddForm("Description", "description", booster.InputWYSIWYG, false, "Product details...")

	// 3. Executive Widgets above Data Grid (CRUDBooster Widget Card equivalent)
	ctrl.AddWidget(booster.WidgetCard{
		Title:     "Catalog Velocity",
		Value:     "6 Active SKUs",
		Subtext:   "Healthy inventory turnover",
		Trend:     "Optimal",
		TrendType: "positive",
		Color:     "blue",
		ColSpan:   3,    // 1/3 lebar layar (12-column grid: 3, 4, 6, 12)
		Size:      "sm", // "sm" (compact), "md" (default), "lg" (hero)
		Icon:      `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m7.5 4.27 9 5.15"/><path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/><path d="m3.3 7 8.7 5 8.7-5"/><path d="M12 22V12"/></svg>`,
	})
	ctrl.AddWidget(booster.WidgetCard{
		Title:     "Inventory Valuation",
		Value:     "Rp 128.450.000",
		Subtext:   "Warehouse stock valuation",
		Trend:     "+14.2%",
		TrendType: "positive",
		Color:     "green",
		ColSpan:   3, // atau Width: "col-4" / "1/3"
		Icon:      `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="12" y1="1" x2="12" y2="23"/><path d="M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>`,
	})
	ctrl.AddWidget(booster.WidgetCard{
		Title:     "Reorder Alert",
		Value:     "1 Low SKU",
		Subtext:   "LG UltraFine (4 units left)",
		Trend:     "Action Req",
		TrendType: "neutral",
		Color:     "amber",
		ColSpan:   3,
		Icon:      `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>`,
	})

	// 4. Custom Top Actions (CRUDBooster index_button equivalent)
	ctrl.AddTopAction(booster.TopAction{
		Label:   "Import Excel",
		Icon:    `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/></svg>`,
		Class:   "btn-secondary",
		OnClick: "openImportModal()",
	})

	// 5. Custom Row Action (CRUDBooster addaction equivalent)
	ctrl.AddRowAction(booster.RowAction{
		Label:   "Duplicate SKU",
		Icon:    `<svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect width="13" height="13" x="9" y="9" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`,
		OnClick: "showToast('Duplicating product #' + %v, 'info')",
	})

	// 6. Pre-Index Announcement Banner (CRUDBooster pre_index_html equivalent)
	ctrl.SetPreIndexHTML(`
		<div style="background: rgba(56, 189, 248, 0.08); border: 1px solid rgba(56, 189, 248, 0.25); border-radius: var(--radius-md); padding: 0.85rem 1.25rem; display: flex; align-items: center; justify-content: space-between; gap: 1rem;">
			<div style="display: flex; align-items: center; gap: 10px;">
				<div style="color: var(--accent);"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg></div>
				<span style="font-size: 0.85rem; color: var(--text-main);"><strong>Warehouse Sync Active:</strong> Automatic reconciliation with SAP/ERP inventory runs every 15 minutes.</span>
			</div>
			<span class="cb-badge cb-badge-primary">Live Sync</span>
		</div>
	`)

	// 3. Lifecycle Hooks
	ctrl.HookBeforeAdd = func(c *booster.Context, data map[string]interface{}) error {
		if status, ok := data["status"]; !ok || status == "" {
			data["status"] = "Active"
		}
		// Format price if user typed only digits
		if price, ok := data["price"].(string); ok && price != "" && !strings.HasPrefix(price, "Rp") {
			data["price"] = "Rp " + price
		}
		return nil
	}

	ctrl.HookBeforeEdit = func(c *booster.Context, data map[string]interface{}) error {
		if price, ok := data["price"].(string); ok && price != "" && !strings.HasPrefix(price, "Rp") {
			data["price"] = "Rp " + price
		}
		return nil
	}

	return ctrl
}
