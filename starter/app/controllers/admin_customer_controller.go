package controllers

import (
	"errors"
	"strings"

	"github.com/tokalink/tgo-booster"
	"github.com/tokalink/tgo-booster/starter/app/models"
)

// NewAdminCustomerController builds the Customers resource controller
func NewAdminCustomerController() *booster.Controller {
	ctrl := &booster.Controller{
		Title:        "Customers",
		Table:        "customers",
		Icon:         `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>`,
		DataProvider: models.CustomerRepo,
	}

	// 1. Data Grid Columns
	ctrl.
		AddCol("ID", "id", booster.ColText, false, true).
		AddCol("Full Name", "name", booster.ColText, true, true).
		AddCol("Email", "email", booster.ColEmail, true, false).
		AddCol("Status", "status", booster.ColBadge, false, false)

	// 2. Form Input Fields
	ctrl.
		AddForm("Full Name", "name", booster.InputText, true, "Enter customer name...").
		AddForm("Email Address", "email", booster.InputEmail, true, "user@company.com").
		AddSelect("Customer Status", "status", true, "Select status...",
			booster.Option{Value: "Active", Label: "Active"},
			booster.Option{Value: "Inactive", Label: "Inactive"},
		)

	// 3. Lifecycle Hooks
	ctrl.HookBeforeAdd = func(c *booster.Context, data map[string]interface{}) error {
		email, _ := data["email"].(string)
		if email != "" && !strings.Contains(email, "@") {
			return errors.New("invalid email address format")
		}
		if status, ok := data["status"]; !ok || status == "" {
			data["status"] = "Active"
		}
		return nil
	}

	return ctrl
}
