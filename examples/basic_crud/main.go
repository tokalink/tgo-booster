package main

import (
	"log"

	"github.com/tokalink/tgo/pkg/app"
	"github.com/tokalink/tgo-booster"
)

func main() {
	application := app.New().SetAddr(":8080")

	// Create Booster Admin Engine
	admin := booster.NewEngine()

	// 1. Register Product Module (CRUDBooster style)
	productCtrl := &booster.Controller{
		Title: "Products",
		Table: "products",
		Columns: []booster.Column{
			{Label: "ID", Name: "id", Type: booster.TypeText, Sortable: true},
			{Label: "Product Name", Name: "name", Type: booster.TypeText, Searchable: true, Sortable: true},
			{Label: "Price", Name: "price", Type: booster.TypeMoney, Sortable: true},
			{Label: "Stock", Name: "stock", Type: booster.TypeNumber},
			{Label: "Status", Name: "status", Type: booster.TypeBadge},
		},
		Forms: []booster.Field{
			{Label: "Product Name", Name: "name", Type: booster.InputText, Required: true, Placeholder: "Enter product name..."},
			{Label: "Price", Name: "price", Type: booster.InputMoney, Required: true},
			{Label: "Stock", Name: "stock", Type: booster.InputNumber, DefaultValue: "10"},
			{Label: "Status", Name: "status", Type: booster.InputSelect, Required: true, Options: []booster.Option{
				{Value: "In Stock", Label: "In Stock"},
				{Value: "Out of Stock", Label: "Out of Stock"},
			}},
		},
	}

	// 2. Register Customer Module
	customerCtrl := &booster.Controller{
		Title: "Customers",
		Table: "customers",
		Columns: []booster.Column{
			{Label: "ID", Name: "id", Type: booster.TypeText, Sortable: true},
			{Label: "Customer Name", Name: "name", Type: booster.TypeText, Searchable: true},
			{Label: "Email", Name: "email", Type: booster.TypeText, Searchable: true},
			{Label: "Status", Name: "status", Type: booster.TypeBadge},
		},
		Forms: []booster.Field{
			{Label: "Customer Name", Name: "name", Type: booster.InputText, Required: true},
			{Label: "Email Address", Name: "email", Type: booster.InputText, Required: true},
		},
	}

	admin.Register(productCtrl)
	admin.Register(customerCtrl)

	// Mount Admin Dashboard to TGo Server
	admin.Mount(application.Server(), "/admin")

	log.Println("⚡ [TGo Booster Demo] Running on http://localhost:8080/admin")
	if err := application.Run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
