package main

import (
	"log"

	"github.com/tokalink/tgo/pkg/app"
	"github.com/tokalink/tgo-booster"
)

func main() {
	application := app.New().SetAddr(":8080")

	// 1. Initialize CRUDBooster Engine
	admin := booster.NewEngine("TGo Enterprise Admin")

	// 2. Register Products Module (CRUDBooster-Style)
	productCtrl := &booster.Controller{
		Title: "Products",
		Table: "products",
		Icon:  "🛍️",
	}
	productCtrl.
		AddCol("ID", "id", booster.ColText, false, true).
		AddCol("Product Name", "name", booster.ColText, true, true).
		AddCol("Price", "price", booster.ColMoney, false, true).
		AddCol("Stock", "stock", booster.ColNumber, false, true).
		AddCol("Status", "status", booster.ColBadge, false, false)

	productCtrl.
		AddForm("Product Name", "name", booster.InputText, true, "Enter product name...").
		AddForm("Price", "price", booster.InputMoney, true, "Rp 0").
		AddForm("Stock", "stock", booster.InputNumber, true, "10").
		AddForm("Description", "description", booster.InputWYSIWYG, false, "Product details...")

	// 3. Register Customers Module
	customerCtrl := &booster.Controller{
		Title: "Customers",
		Table: "customers",
		Icon:  "👥",
	}
	customerCtrl.
		AddCol("ID", "id", booster.ColText, false, true).
		AddCol("Full Name", "name", booster.ColText, true, true).
		AddCol("Email", "email", booster.ColEmail, true, false).
		AddCol("Status", "status", booster.ColBadge, false, false)

	customerCtrl.
		AddForm("Full Name", "name", booster.InputText, true, "Enter customer name...").
		AddForm("Email Address", "email", booster.InputEmail, true, "user@company.com")

	// 4. Register Orders Module
	orderCtrl := &booster.Controller{
		Title: "Orders",
		Table: "orders",
		Icon:  "📦",
	}
	orderCtrl.
		AddCol("Order ID", "id", booster.ColText, false, true).
		AddCol("Customer", "customer_name", booster.ColText, true, true).
		AddCol("Total Amount", "total_amount", booster.ColMoney, false, true).
		AddCol("Status", "status", booster.ColBadge, false, false)

	admin.Register(productCtrl)
	admin.Register(customerCtrl)
	admin.Register(orderCtrl)

	// 5. Mount Booster Dashboard to /admin
	admin.Mount(application.Server(), "/admin")

	log.Println("=========================================================")
	log.Println("⚡ [TGo Booster Starter] Server running on http://localhost:8080")
	log.Println("   ├─ Login Page: http://localhost:8080/admin/login")
	log.Println("   ├─ Default Email: admin@tgo.io")
	log.Println("   └─ Default Password: admin123")
	log.Println("=========================================================")

	if err := application.Run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
