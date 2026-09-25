# ⚡ TGo Booster

> **Ultra-Fast Declarative CRUD Dashboard Engine** for [TGo Framework](https://github.com/tokalink/tgo) (CRUDBooster v5.6 style in Go).

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org)
[![TGo Framework](https://img.shields.io/badge/Framework-TGo-blue.svg)](https://github.com/tokalink/tgo)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

## 🌟 Fitur Utama

- **🚀 Sub-Milidetik TTFB (< 1 ms)**: 50x lebih cepat daripada PHP/Laravel CRUDBooster.
- **📋 Declarative Grid & Form Builder**: Cukup definisikan kolom tabel dan input form dalam struct Go.
- **🎨 100% Non-CDN Self-Hosted UI**: Tampilan Glassmorphism modern tanpa ketergantungan CDN eksternal.
- **🔒 Multi-Tenant Native**: Terintegrasi langsung dengan kernel isolasi schema PostgreSQL / SQLite TGo.
- **🪝 Lifecycle Hooks**: `HookBeforeAdd`, `HookAfterAdd`, `HookBeforeEdit`, `HookAfterEdit`, `HookBeforeDelete`, `HookAfterDelete`.

---

## 📦 Instalasi

```bash
go get github.com/tokalink/tgo-booster@latest
```

---

## 🚀 Quick Start

```go
package main

import (
    "github.com/tokalink/tgo/pkg/app"
    "github.com/tokalink/tgo-booster"
)

func main() {
    application := app.New().SetAddr(":8080")

    admin := booster.NewEngine()
    
    // Daftarkan modul CRUD
    admin.Register(&booster.Controller{
        Title: "Products",
        Table: "products",
        Columns: []booster.Column{
            {Label: "Name", Name: "name", Type: booster.TypeText, Searchable: true},
            {Label: "Price", Name: "price", Type: booster.TypeMoney, Sortable: true},
            {Label: "Status", Name: "status", Type: booster.TypeBadge},
        },
        Forms: []booster.Field{
            {Label: "Product Name", Name: "name", Type: booster.InputText, Required: true},
            {Label: "Price", Name: "price", Type: booster.InputMoney, Required: true},
        },
    })

    admin.Mount(application.Server(), "/admin")

    application.Run()
}
```

Buka **`http://localhost:8080/admin`** di browser.

---

## 📜 Lisensi
MIT License © 2026 [Tokalink](https://github.com/tokalink).
