# 🎨 TGo Booster UI Template Architecture

Direktori ini (`pkg/cb/templates`) mengorganisasi seluruh template HTML TGo Booster dengan pemisahan tanggung jawab yang rapi, bersih, dan modular antara **Layouts**, **Components**, dan **Pages/Views**.

---

## 📁 Struktur Direktori

```text
pkg/cb/templates/
├── README.md               <-- Dokumentasi & panduan arsitektur template
│
├── layouts/                <-- 🏛️ Skeleton / Master Page Wrappers (Outer Shell)
│   ├── main.html           <-- Master Admin Layout (HTML, Head, Topbar, Sidebar, Content, Modals)
│   └── auth.html           <-- Authentication Layout (Login & Sign-In Card Shell)
│
├── components/             <-- 🧩 Reusable Partial UI Widgets & Blocks
│   ├── sidebar.html        <-- Navigation Sidebar (Logo brand, menu items, dropdowns)
│   ├── topbar.html         <-- Header Topbar (Breadcrumb, search trigger, theme toggle, telemetry)
│   └── modals.html         <-- Global Dialogs (Quick Command Palette Ctrl+K, Lightbox, Toast alerts)
│
└── pages/                  <-- 📄 Feature / Route Content Views (Injected into {{.Content}})
    ├── table.html          <-- Responsive CRUD Data Grid (Search, filter drawer, export/import CSV, pagination)
    ├── form.html           <-- Dynamic Form Builder (Add/Edit records, validation, field widgets)
    ├── menus.html          <-- Menu Management Studio (Drag & drop hierarchical tree, submenu builder)
    ├── pages.html          <-- Custom Pages & SEO Studio (WYSIWYG visual + VS Code-like HTML IDE)
    ├── privileges.html     <-- RBAC Matrix Studio (Roles & Privileges management)
    ├── module_generator.html <-- Booster Studio Code Generator (Instant CRUD generator)
    ├── settings.html       <-- Platform Settings Studio (Branding, Skins, Security, SMTP, Diagnostics)
    └── api_generator.html  <-- API & Tokens Studio (Tokens, Live Explorer, Multi-SDK, OpenAPI 3.0)
```

---

## 🏛️ 1. Layouts (`templates/layouts/`)
Layout adalah kerangka utama halaman HTML luar (*outer shell*) yang mendefinisikan struktur dokumen lengkap:

| File | Deskripsi | Komponen yang Dimuat |
| :--- | :--- | :--- |
| **`layouts/main.html`** | Kerangka utama seluruh halaman admin setelah login | `{{template "sidebar" .}}`, `{{template "topbar" .}}`, slot konten `{{.Content}}`, dan `{{template "modals" .}}` |
| **`layouts/auth.html`** | Kerangka khusus otentikasi login dengan kartu terpusat dan toggle tema | Form login, input credentials, logo aplikasi, remember me |

---

## 🧩 2. Components (`templates/components/`)
Komponen adalah potongan UI modular yang dapat dipakai ulang (*reusable partials*). Masing-masing didefinisikan dengan blok `{{define "nama_komponen"}}`:

| Komponen | Template Name | Deskripsi |
| :--- | :--- | :--- |
| **`components/sidebar.html`** | `{{template "sidebar" .}}` | Sidebar navigasi kiri dengan logo, nama aplikasi, menu hierarkis bertingkat, badge counter, dan tombol collapse. |
| **`components/topbar.html`** | `{{template "topbar" .}}` | Bar atas dengan tombol hamburger mobile, breadcrumbs dinamis, trigger command palette `Ctrl+K`, tombol toggle Dark/Light mode, beacon latency, dan quick profile. |
| **`components/modals.html`** | `{{template "modals" .}}` | Modal global: Quick Search Command Palette (`Ctrl+K`), modal Lightbox pratinjau gambar, dan container floating toast notification. |

---

## 📄 3. Pages / Views (`templates/pages/`)
Halaman adalah view isi spesifik per rute/modul yang dirender dan disuntikkan ke dalam layout master pada slot `{{.Content}}`:

1. **`pages/table.html`**:
   - Menampilkan tabel data CRUD responsif.
   - Fitur: Search live filter, drawer filter lanjutan, tombol ekspor CSV, modal impor CSV, bulk actions (hapus massal), pagination, dan view detail.
2. **`pages/form.html`**:
   - Form builder serbaguna untuk Tambah (`/add`) dan Edit (`/edit/:id`) data record.
   - Fitur: Validasi field, tabbed groups, datepicker, select2 dropdown, file upload preview.
3. **`pages/menus.html`**:
   - Studio Manajemen Menu interaktif dengan drag & drop reorder, sub-menu hierarkis, icon selector, dan RBAC role permissions.
4. **`pages/pages.html`**:
   - Studio Pembuat Halaman Kustom & SEO dengan mode ganda: Visual WYSIWYG Editor dan **Advanced VS Code-like HTML IDE Editor** (lengkap dengan gutter line numbers, format HTML, syntax highlighting, dan search & replace drawer).
5. **`pages/privileges.html`**:
   - Matriks Role-Based Access Control (RBAC) untuk mengatur hak akses read/create/edit/delete setiap role pengguna.
6. **`pages/module_generator.html`**:
   - Generator modul CRUD instan dengan GUI visual untuk membuat model, controller, rute, dan menu otomatis.
7. **`pages/settings.html`**:
   - Studio Pengaturan Platform komprehensif 6 tab: General & Branding, Appearance & Themes, Security & Auth, Data Grid & CRUD, Mail & Alerts (SMTP), dan System Diagnostics & Telemetry.
8. **`pages/api_generator.html`**:
   - Studio API & Tokens enterprise: Manajemen Personal Access Token / API Key dengan scope & rate limiting, Live API Explorer interaktif (Try-It-Out console), Multi-Language SDK Code Generator (cURL, Go, TS/JS, Python), dan OpenAPI 3.0.0 exporter.

---

## 🔄 Alur Rendering di Go (`pkg/cb/ui.go`)

```mermaid
graph TD
    A[HTTP Request] --> B[Controller / Route Handler]
    B --> C[Render Page Template e.g. RenderTableContent]
    C --> D[HTML String {{.Content}}]
    D --> E[RenderMasterLayout data]
    E --> F[Combine with layouts/main.html + components]
    F --> G[Rendered Complete HTML Document to Browser]
```

1. Controller merender halaman konten spesifik (misal: `RenderTableContent(data)`).
2. Hasil HTML string dimasukkan ke dalam `data["Content"]`.
3. `RenderMasterLayout(data)` menggabungkan `layouts/main.html` bersama komponen `sidebar`, `topbar`, dan `modals`.
4. Browser menerima dokumen HTML utuh yang bersih dan terstruktur.
