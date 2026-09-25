# ⚡ TGo Booster

> **Ultra-Fast Declarative CRUD Dashboard Engine for Go** — The modern Golang evolution of **CRUDBooster (v5.6)**.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![TGo Framework](https://img.shields.io/badge/Framework-TGo-blue.svg)](https://github.com/tokalink/tgo)
[![Response Time](https://img.shields.io/badge/Response-0.4ms-10b981.svg)](#)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

## 🌟 Fitur Utama (CRUDBooster v5.6 in Go)

- **🔐 CBAuth (Authentication & Session)**:
  - Halaman Login modern siap pakai di `/admin/login`.
  - Session cookie terenkripsi, Remember Me, dan secure logout.
  - Akun bawaan starter: **`admin@tgo.io`** / **`admin123`**.
- **📊 CBDashboard (Starter Dashboard)**:
  - KPI Statistic Widgets (Revenue, Active Users, Total Orders, Server Latency).
  - Live Audit Trail Logs (`CBLogs`) & Quick Action shortcuts.
- **📋 CBController (Declarative Grid & Form Builder)**:
  - Builder kolom fluent: `AddCol("Name", "name", booster.ColText, true, true)`.
  - Builder form fluent: `AddForm("Price", "price", booster.InputMoney, true, "Rp 0")`.
  - Format kolom: `ColText`, `ColImage`, `ColNumber`, `ColMoney`, `ColBadge`, `ColDateTime`, `ColEmail`.
  - Format input: `InputText`, `InputMoney`, `InputSelect`, `InputWYSIWYG`, `InputUpload`, `InputDate`, `InputPassword`.
- **🪝 Complete Hook Lifecycle**:
  - `HookBeforeAdd`, `HookAfterAdd`, `HookBeforeEdit`, `HookAfterEdit`, `HookBeforeDelete`, `HookAfterDelete`, `HookRowListing`.
- **🎨 100% Non-CDN Self-Hosted Theme**:
  - Tampilan Glassmorphism modern tanpa CDN eksternal (0.4 ms load time).

---

## 📂 Struktur Repositori

```text
tgo-booster/
├── pkg/
│   └── cb/                     # Core CRUDBooster Engine
│       ├── auth.go             # CBAuth (Login, Logout, Session)
│       ├── controller.go       # CBController (Grid, Form, Hooks)
│       ├── dashboard.go        # CBDashboard (KPI Widgets, Audit Logs)
│       ├── engine.go           # Engine Router & Master Layout
│       ├── types.go            # Column, Field, Menu, User, Hook Types
│       └── ui.go               # Self-Hosted Responsive HTML/CSS Layouts
├── starter/                    # Ready-to-Run Starter App
│   ├── config/
│   │   └── booster.yaml        # Booster configuration
│   └── main.go                 # Ready-to-Run Demo (Login + Dashboard + 3 CRUD Modules)
├── booster.go                  # Root Package Type Aliases
├── go.mod                      # module github.com/tokalink/tgo-booster
└── README.md
```

---

## 🚀 Menjalankan Starter Application

```bash
cd tgo-booster
go run starter/main.go
```

1. Buka browser di **`http://localhost:8080/admin/login`**.
2. Masuk dengan kredensial bawaan:
   - **Email**: `admin@tgo.io`
   - **Password**: `admin123`
3. Nikmati Dashboard Starter lengkap dengan widget KPI dan 3 modul CRUD (Products, Customers, Orders)!

---

## 💻 Contoh Penggunaan di Proyek Anda

```go
package main

import (
    "github.com/tokalink/tgo/pkg/app"
    "github.com/tokalink/tgo-booster"
)

func main() {
    application := app.New().SetAddr(":8080")

    // 1. Inisialisasi Booster Engine
    admin := booster.NewEngine("My Company Admin")

    // 2. Buat Modul CRUD Produk
    products := &booster.Controller{
        Title: "Products",
        Table: "products",
        Icon:  "🛍️",
    }
    products.
        AddCol("ID", "id", booster.ColText, false, true).
        AddCol("Product Name", "name", booster.ColText, true, true).
        AddCol("Price", "price", booster.ColMoney, false, true).
        AddCol("Status", "status", booster.ColBadge, false, false)

    products.
        AddForm("Product Name", "name", booster.InputText, true, "Enter product name...").
        AddForm("Price", "price", booster.InputMoney, true, "Rp 0")

    // 3. Daftarkan dan Pasang ke Server
    admin.Register(products)
    admin.Mount(application.Server(), "/admin")

    application.Run()
}
```

---

## 📜 Lisensi
MIT License © 2026 [Tokalink](https://github.com/tokalink).
