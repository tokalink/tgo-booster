package main

import (
	"log"
	"net/http"

	"github.com/tokalink/tgo/pkg/app"
	"github.com/tokalink/tgo-booster"
	"github.com/tokalink/tgo-booster/starter/app/controllers"
	"github.com/tokalink/tgo-booster/starter/app/models"
)

func main() {
	// 0. Initialize Database from .env (MySQL or Memory fallback)
	db, _ := models.InitDatabase()

	application := app.New().SetAddr(":8080")

	// 1. Initialize TGo Booster Engine
	admin := booster.NewEngine("TGo Enterprise Admin")
	if db != nil {
		admin.SetDB(db)
	}

	// 2. Register MVC Controllers
	admin.Register(controllers.NewAdminProductController())
	admin.Register(controllers.NewAdminCustomerController())
	admin.Register(controllers.NewAdminOrderController())

	// 3. Mount Booster Dashboard to /admin
	admin.Mount(application.Server(), "/admin")

	// 4. Root /: Serve custom homepage from Pages Studio if configured, else redirect to /admin
	application.Server().Register("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "" {
			if admin.ServePublicPage(w, r) {
				return
			}
		}
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}))

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
