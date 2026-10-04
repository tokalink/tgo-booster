# TGo Booster - Sistem Pembenahan & Perbaikan Bug (TodoList 03-10-2026)

Dokumen ini melacak eksekusi bertahap perbaikan bug, penguatan keamanan, persistensi data, dan perapian arsitektur framework TGo Booster.

---

## 📌 Status Ringkasan Eksekusi

| Fase | Fokus Area | Target | Status |
|---|---|---|---|
| **Fase 1** | Keamanan, User Accounts & Session Hardening | `users.json`, HMAC Auth, RBAC Guards | ✅ Selesai (100%) |
| **Fase 2** | Real Audit Trail Logging Engine | `logs.json`, Lifecycle Hooks, Audit Provider | ✅ Selesai (100%) |
| **Fase 3** | Perbaikan Data Grid & Forms | Dynamic PK, Real CSV Import, WYSIWYG Editor | ✅ Selesai (100%) |
| **Fase 4** | Executive Dashboard & Email Templates | `dashboard.html` template, Dynamic KPIs, `email_templates.json` | ✅ Selesai (100%) |
| **Fase 5** | Pengujian & Verifikasi Menyeluruh | Unit Tests, Integration Tests, Zero Breakage | ✅ Selesai (100%) |
| **Fase 6** | Custom Pages, GetIndex & GetDetail Overrides | `GetIndex`, `GetDetail`, `AddCustomAction`, `AddCustomPage` | ✅ Selesai (100%) |
| **Fase 7** | Theme Mode & Color Skin Synchronization | Light/Dark Mode Studio, Multi-Skin, Persistence | ✅ Selesai (100%) |
| **Fase 8** | Bespoke Responsive Landing & Public Pages Layout | `landing.html`, Bento Grid, Interactive Showcase, Multi-Template | ✅ Selesai (100%) |
| **Fase 9** | AI & LLM Service Studio (OpenAI Compatible) & AI Page Generator | `pkg/cb/ai.go`, Hybrid Settings Fallback, AI Page Generator Modal | ✅ Selesai (100%) |
| **Fase 10** | Optimasi Base Framework & Dokumentasi Proyek Baru | `SQLStore.AutoMigrate`, `tgo-cms` Build, `getting-started-new-project.md` | ✅ Selesai (100%) |

---

## 📋 Checklist Eksekusi Rinci

### Fase 1: Keamanan, User Accounts & Session Hardening
- [x] **1.1 Persistensi User Accounts (`data/users.json`) & Password Hashing**
  - Implementasi struct `UserAccount` lengkap dengan `PasswordHash` (SHA-256 + Salt acak 16-byte).
  - Buat `UserManager` dengan thread-safe persistence (`sync.RWMutex`).
  - Auto-seed default accounts (`admin@tgo.io`, `manager@tgo.io`, `auditor@tgo.io`) saat file belum ada.
  - Implementasikan `UserDataProvider` khusus untuk controller `/admin/users` agar mendukung Create, Read, Update, Delete secara persisten ke `data/users.json`.
- [x] **1.2 Integrasi Autentikasi Riil (`auth.go`)**
  - Perbarui `AuthManager.ServeLogin` agar memverifikasi password terhadap `data/users.json` dan bukan hardcoded credentials.
  - Perbarui status `LastLoginAt` saat user berhasil masuk.
  - Tampilkan pesan error ramah jika akun dalam status `Suspended`.
- [x] **1.3 Session Cookie Signing dengan HMAC-SHA256**
  - Amankan cookie `cb_session_token` dengan secret signing HMAC-SHA256 (`payload.signature`) untuk mencegah cookie forgery / privilege escalation.
  - Validasi signature saat decode session token di `GetSessionUser(r)`.
- [x] **1.4 Sinkronisasi Role Dropdown dari `data/privileges.json`**
  - Ganti static options di `userCtrl` dengan dynamic options yang ditarik dari daftar role aktif di `e.roles` / `data/privileges.json`.
- [x] **1.5 Superadmin & RBAC Guards pada Seluruh Studio Engine**
  - Tambahkan pengecekan autentikasi dan Superadmin role guard pada:
    - `/admin/settings` & `/admin/settings/save`
    - `/admin/api_generator`, `/save`, `/delete`
    - `/admin/module_generator`, `/save`, `/delete`
    - `/admin/menus`, `/save`, `/delete`, `/reorder`
    - `/admin/privileges`, `/save`, `/delete`, `/switch`
- [x] **1.6 Runtime Security Enforcement**
  - Terapkan brute-force rate limiter / login attempt counter berdasarkan `AppSettings.max_login_attempts` & `AppSettings.lockout_duration_min`.
  - Terapkan `AppSettings.maintenance_mode` guard: blokir non-superadmin jika maintenance mode aktif.

---

### Fase 2: Real Audit Trail Logging Engine
- [x] **2.1 Engine Audit Logger & Persistensi `data/logs.json`**
  - Implementasikan method `e.LogAudit(r *http.Request, action, module, description string)`.
  - Batasi riwayat log (e.g. 1000 entri terbaru) agar performa tetap ringan dan thread-safe.
  - Hormati toggle konfigurasi `AppSettings.enable_audit_log`.
- [x] **2.2 Event Hooking: Autentikasi**
  - Rekam audit saat `LOGIN` berhasil dan `LOGOUT`.
- [x] **2.3 Event Hooking: CRUD Data Lifecycle**
  - Rekam audit otomatis di `controller.go`:
    - `CREATE`: Record baru ditambahkan.
    - `UPDATE`: Record diperbarui.
    - `DELETE`: Record dihapus.
    - `BULK_DELETE`: Penghapusan massal.
    - `IMPORT_CSV`: Impor massal data CSV.
- [x] **2.4 Event Hooking: System Studio Events**
  - Rekam audit saat:
    - `SETTINGS_UPDATE`
    - `TOKEN_CREATE`, `TOKEN_REVOKE`
    - `MENU_UPDATE`, `MENU_DELETE`, `MENU_REORDER`
    - `PAGE_UPDATE`, `PAGE_DELETE`
    - `ROLE_UPDATE`, `ROLE_DELETE`, `ROLE_SWITCH`
- [x] **2.5 Real DataProvider untuk `/admin/logs`**
  - Ganti mock static `logStore` dengan DataProvider riil berbasis `data/logs.json`.

---

### Fase 3: Perbaikan CRUD Data Grid & Forms
- [x] **3.1 Dynamic Primary Key di `table.html`**
  - Teruskan `"PrimaryKey": c.PrimaryKey` dari `handleIndexHTML` ke template data.
  - Ganti seluruh pemanggilan hardcoded `{{index $row "id"}}` di `table.html` menjadi `{{index $row $.PrimaryKey}}`.
  - Tombol View Details, Edit URL, Delete confirmation, Checkbox row, dan custom `RowActions` bekerja untuk tabel dengan PK selain `"id"`.
- [x] **3.2 Real CSV Import Handler Backend & Frontend**
  - Buat handler `POST /:module/import-csv` di `controller.go` yang mendukung:
    - JSON payload `{ "rows": [...] }` dari browser.
    - Direct multipart file upload `csv_file` dengan parser `encoding/csv`.
  - Hubungkan fungsi `submitImportCSV()` di `table.html` agar mengirim payload ke server dan menampilkan feedback toast riil dengan jumlah baris yang berhasil diimpor.
- [x] **3.3 Visual WYSIWYG Editor di `form.html`**
  - Tambahkan container WYSIWYG editor pada `InputWYSIWYG` di `form.html` lengkap dengan toolbar formatting (Bold, Italic, Heading 2/3, Unordered/Ordered List, Link, Image URL, dan Code switch) dan sinkronisasi otomatis ke hidden input textarea saat form submit.

---

### Fase 4: Executive Dashboard & Email Templates Persistence
- [x] **4.1 Ekstraksi Dashboard ke Modular Template**
  - Buat file template `pkg/cb/templates/pages/dashboard.html`.
  - Pindahkan render dari string `fmt.Sprintf` inline di `dashboard.go` ke pemanggilan template resmi `RenderDashboardContent`.
- [x] **4.2 Dynamic KPI Metrics**
  - Hitung metrik agregat secara dinamis: Total active modules, Total records, Total users, Total custom pages, System Diagnostics (Go version, RAM alloc, uptime).
  - Tampilkan recent activity feed dari `data/logs.json`.
- [x] **4.3 Persistensi Modul Email Templates (`data/email_templates.json`)**
  - Buat `EmailTemplateManager` dan `EmailTemplateDataProvider` untuk modul `/admin/email_templates` agar template email tersimpan aman di `data/email_templates.json`.

---

### Fase 5: Pengujian & Validasi Menyeluruh
- [x] **5.1 Unit Tests & Test Automation**
  - Tambahkan unit test komprehensif di `pkg/cb/hardening_test.go`:
    - `TestUserManager_AuthenticationAndHashing`: Password verification, salt hashing, brute-force lockout, CRUD operations.
    - `TestAuthSession_HMACSignatureTampering`: Session token signing, cookie tampering detection & rejection.
    - `TestAuditLogger_PersistenceAndCap`: Log writing, retrieval via DataProvider, persistence to `data/logs.json`.
    - `TestCSVImport_ControllerHandler`: Multipart CSV upload & JSON row parsing, record persistence.
    - `TestEmailTemplates_CRUD`: Persistent email templates CRUD & slug retrieval.
  - Jalankan `go test -v ./...` dan pastikan seluruh test (17/17 tests di seluruh package) PASS 100%.
- [x] **5.2 Live Verification**
  - Verifikasi tampilan dan fungsionalitas di browser:
    - Executive Dashboard (KPI stats, recent audit trail, module launcher).
    - User Management (`/admin/users`).
    - Security Audit Trail (`/admin/logs`).
    - Email Templates (`/admin/email_templates`).

---

### Fase 6: Custom Pages, GetIndex & GetDetail Overrides
- [x] **6.1 Controller `GetIndex` Hook & Override**
  - Developer dapat meng-override halaman tabel utama suatu Controller dengan mendefinisikan `ctrl.GetIndex = func(w, r) bool`.
  - Jika mengembalikan `true`, custom view dieksekusi. Jika `false`, fallback ke data table default.
- [x] **6.2 Controller `GetDetail` Hook & Override**
  - Developer dapat meng-override tampilan detail record `/admin/:module/detail/:id` dengan `ctrl.GetDetail = func(w, r, id) bool`.
  - Dilengkapi helper `ctrl.FindRecord(r, id)` dan `ctrl.RenderView(w, r, title, contentHTML)` untuk merender halaman kustom langsung di dalam shell admin layout.
- [x] **6.3 Controller `AddCustomAction` Sub-Pages**
  - Mendukung sub-halaman khusus pada modul (contoh: `/admin/orders/invoice/:id` atau `/admin/products/inventory-audit`).
  - Didaftarkan secara fleksibel lewat `ctrl.AddCustomAction(slug, handler)`.
- [x] **6.4 Engine `AddCustomPage` (Standalone Admin Page)**
  - Mendukung pembuatan custom page independen di admin panel dengan 1 baris kode:
    `engine.AddCustomPage("/reports/sales", "Sales Report", iconSVG, handler, addToMenu)`
  - Otomatis memproteksi sesi autentikasi, pengecekan role RBAC, dan menambahkan item ke sidebar menu.
- [x] **6.5 Comprehensive Test Suite**
  - Unit test `TestCustomPage_GetIndex_GetDetail_Overrides` berjalan 100% PASS.
- [x] **6.6 Rendering dari File Template Eksternal (`.html`) & `embed.FS`**
  - Dukungan method `ctrl.RenderViewFile(...)` dan `ctrl.RenderViewFS(...)` agar developer bisa memisahkan file view HTML dari kode Go.
  - Dilengkapi sistem template caching dengan deteksi perubahan timestamp file (`ModTime`) otomatis (hot-reload saat development, zero overhead di production).
  - Method `engine.AddCustomPageWithFile(...)` untuk mendaftarkan standalone custom admin page langsung dari file template fisik.
  - Unit test `TestRenderViewFile_HTMLTemplateFile` berjalan 100% PASS (Total 19/19 tests pass).

---

### Fase 7: Theme Mode & Color Skin Synchronization
- [x] **7.1 Pemisahan Dependensi Skin Preset dari Theme Mode**
  - Perbaiki bug di `pkg/cb/templates/pages/settings.html` (`selectSkinPreset`) yang sebelumnya memaksa `setTheme('dark')` setiap kali user memilih preset skin selain Clean Slate (seperti Emerald Enterprise, Midnight, Obsidian, Royal Amethyst).
  - Sekarang setiap color skin dapat digunakan secara fleksibel baik di **Light Mode** maupun **Dark Mode** tanpa saling menimpa.
- [x] **7.2 Sinkronisasi Real-Time Radio Pills & DOM Events**
  - Tambahkan handler `onclick` langsung pada wrapper `<label class="theme-mode-pill">` agar klik pada pill langsung memicu `handleThemeModeChange` secara andal di semua browser.
  - Sinkronisasi otomatis radio button state di `main.html` (`setTheme`) dan `settings.html` (`DOMContentLoaded`).
- [x] **7.3 Persistensi Sinkron `data/settings.json` & `localStorage`**
  - Update `saveAllSettings()` untuk menyimpan state `theme_mode` dan `theme_skin` ke `data/settings.json` sekaligus menyinkronkan client storage `localStorage['tgo_booster_theme']` dan `localStorage['tgo_booster_skin']`.
  - Verifikasi browser: Light Mode + Emerald Enterprise berhasil aktif instan dan tetap persisten setelah reload maupun navigasi antar halaman.

---

### Fase 8: Bespoke Responsive Landing & Public Pages Layout System
- [x] **8.1 Desain Layout Eksklusif Non-Admin (`pkg/cb/templates/layouts/landing.html`)**
  - Membuat layout terpisah untuk halaman publik / landing pages agar tidak lagi memakai layout dashboard admin (tanpa admin sidebar, tanpa module search palette).
  - Mengusung estetika enterprise kelas dunia (terinspirasi Linear/Vercel/Stripe) dengan typography modern, ambient radial glow, dan frosted glassmorphism.
- [x] **8.2 Fitur Interaktif & UX yang Menarik (Menghindari Kesan Template AI)**
  - Floating pill navigation bar dengan logo, live status beacon (`🟢 0.38ms TTFB`), Theme switcher (Dark/Light), dan CTA button.
  - Mobile hamburger drawer menu animasi slide-down yang responsif untuk layar smartphone.
  - Interactive macOS product showcase window dengan tab switcher nyata (Dashboard, CRUD Data Grid, REST API, Audit Stream) yang dapat diklik langsung oleh pengunjung.
  - Bento Grid Feature section asimetris dengan kartu fitur micro-hover interaktif.
  - One-click copy snippet helper untuk perintah terminal.
  - Multi-column enterprise footer lengkap dengan live status beacon dan Back to Top button.
- [x] **8.3 Dukungan Multi-Template Sesuai Kebutuhan Halaman**
  - `landing`: Hero showcase lengkap, bento grid, stats counter, injected custom content, dan CTA banner.
  - `article`: Editorial magazine layout dengan reading progress bar real-time, author card, reading time, dan tag.
  - `default`: Corporate page layout dengan breadcrumbs navigasi, clean card container, dan responsive typography.
  - `blank`: Clean canvas untuk embed / kustomisasi bebas tanpa wrapper nav & footer.
- [x] **8.4 Verifikasi Browser & 100% Test Passing**
  - Pengujian langsung di browser: tampilan desktop, tablet, dan smartphone responsif mulus.
  - Mode Dark dan Light berganti secara instan dengan transisi lembut.
  - Seluruh 19 unit test (`go test -v ./...`) lulus 100% tanpa error.

---

### Fase 9: Integrasi AI & LLM Service Studio (OpenAI Compatible) & AI Page Generator
- [x] **9.1 Arsitektur Hybrid Fallback Settings (`pkg/cb/settings.go` & `pkg/cb/ai.go`)**
  - Menghubungkan konfigurasi OpenAI `.env` (`OPENAI_HOST`, `OPENAI_APIKEY`, `OPENAI_DEFAULT_MODEL`) dengan Platform Settings Studio (`data/settings.json`).
  - Hierarki Fallback: Nilai yang diatur di UI Settings Studio (`/admin/settings`) memiliki prioritas utama; jika kosong, otomatis fallback ke `.env`.
  - Normalisasi endpoint otomatis (`BuildAIEndpoint`) agar fleksibel menerima base URL maupun full path (`/v1/chat/completions`).
- [x] **9.2 Tab AI & LLM Integration di Platform Settings Studio (`settings.html`)**
  - Menambahkan Tab 6: **AI & LLM Services** di `/admin/settings`.
  - Input Host, Model ID, dan API Secret Key yang dimask dengan toggle visibilitas 👁️.
  - Quick Provider Presets: SumoPod (DeepSeek v4), OpenAI (GPT-4o), OpenAI (GPT-4o Mini).
  - Diagnostic Test Connection (`POST /admin/settings/test-ai`): Menguji handshake dan mengukur roundtrip latency real-time dengan feedback badge hijau (e.g. `959ms`).
- [x] **9.3 AI Page Generator Studio di Custom Pages (`pages.html` & `pages.go`)**
  - Menambahkan tombol **✨ AI Page Generator** di halaman daftar pages (`/admin/pages`) dan di action bar studio editor.
  - Modal interaktif dengan prompt textarea, quick inspiration chips (SaaS, Cybersecurity, Analytics, Promo, Whitepaper), template selector (`landing`, `article`, `default`, `blank`), tone of voice, dan pilihan bahasa (Bahasa Indonesia / English).
  - Endpoint `POST /admin/pages/generate-ai` yang memanggil LLM OpenAI-compatible untuk mensintesis struktur halaman lengkap (Title, Slug, Route URL, Template, Excerpt, SEO Meta, OpenGraph, dan rich semantic HTML content).
  - Integrasi otomatis hasil generasi AI langsung ke form studio dan visual editor canvas.
- [x] **9.4 Verifikasi Live Test & Browser Subagent**
  - Unit test `TestAIService_ConfigAndEndpoint`, `TestAIService_LiveConnectionIfConfigured`, dan `TestAIService_GeneratePageWithAI` berjalan 100% PASS.
  - Pengujian browser otomatis: Berhasil melakukan handshake LLM (latency `959ms`) dan mensintesis landing page SaaS dengan rich HTML secara instan.
- [x] **9.5 Auto-Get AI Model List dari Server Gateway (`/v1/models`)**
  - Endpoint discovery backend `BuildAIModelsEndpoint(host)` & `FetchAIModels(ctx, host, apiKey)` di `pkg/cb/ai.go` yang memanggil OpenAI-compatible `GET /v1/models`, mengurai list ID model aktif, menduplikasi, dan mengurutkan secara alfabetis.
  - Endpoint diagnostic `POST /admin/settings/models` di `pkg/cb/settings.go` untuk menarik daftar model secara langsung dari server/provider yang dikonfigurasi.
  - UI Platform Settings Studio (`settings.html`): Input combobox hybrid (`<input list="aiModelDatalist">`), dynamic dropdown `<select id="aiModelSelectDropdown">`, serta tombol `🔄 Auto-Get Server Models` (`fetchAIModelsList`). Model otomatis ditarik saat tab dibuka atau preset provider diklik.
  - UI Pages & SEO Studio (`pages.html`): Modal "✨ AI Page Generator" kini dilengkapi dropdown model dinamis dengan tombol sinkronisasi server dan auto-fetch saat modal dibuka. Field `model` diteruskan ke `/admin/pages/generate-ai` sehingga user dapat memilih model spesifik per-generasi halaman.
  - Unit test `TestAIService_FetchModels` (PASS) dan verifikasi visual browser subagent pada Settings Studio dan Pages Studio (66 model aktif berhasil ditarik).

---

### Fase 10: Optimasi Base Framework & Dokumentasi Proyek Baru
- [x] **10.1 Fitur Database Schema Auto-Migrate Otomatis**
  - Implementasi method `s.AutoMigrate(fields ...Field)` pada `SQLStore` (`pkg/cb/sql_store.go`) yang mendeteksi dialect database (MySQL, SQLite, PostgreSQL) dan meng-generate query `CREATE TABLE IF NOT EXISTS` lengkap dengan tipe data (TEXT, VARCHAR, INT, TIMESTAMP) dan primary key secara otomatis.
  - Implementasi method `c.AutoMigrate()` pada `Controller` dan `e.AutoMigrateAll()` pada `Engine` sehingga developer tidak perlu menulis skema SQL manual saat membuat modul baru.
  - Unit test `TestSQLStore_AutoMigrate_SQLite` berjalan 100% PASS.
- [x] **10.2 Verifikasi Kompatibilitas Proyek Turunan (`tgo-cms`)**
  - Memperbaiki path dependensi modul pada `tgo-cms/go.mod` (`replace github.com/tokalink/tgo => ../tgo-core`).
  - Menguji `go build .` pada `tgo-cms` menggunakan `tgo-booster` sebagai base framework (berhasil compile 100% tanpa error).
- [x] **10.3 Dokumentasi Komprehensif Panduan Proyek Baru (`docs/getting-started-new-project.md`)**
  - Membuat tutorial terperinci 9 langkah: Inisialisasi modul, struktur MVC standar, koneksi database agnostik, auto-migrate, declarative CRUD controller & relations, custom pages & hot-reload HTML view, RBAC security, AI page generator, dan deployment single binary.
  - Menghubungkan panduan baru di bagian teratas `README.md`.







