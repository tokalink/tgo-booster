package booster

const (
	// Embedded Admin CSS (Self-hosted, non-CDN, ultra fast)
	AdminCSS = `
:root {
  --bg-main: #090d16;
  --bg-card: rgba(18, 24, 38, 0.8);
  --border: rgba(255, 255, 255, 0.08);
  --accent: #38bdf8;
  --accent-hover: #7dd3fc;
  --text-main: #f8fafc;
  --text-muted: #94a3b8;
  --danger: #ef4444;
  --success: #10b981;
  --warning: #f59e0b;
}

* { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
body { background: var(--bg-main); color: var(--text-main); min-height: 100vh; display: flex; }

.booster-sidebar { width: 240px; background: rgba(15, 23, 42, 0.95); border-right: 1px solid var(--border); padding: 1.5rem 1rem; display: flex; flex-direction: column; gap: 1rem; }
.booster-brand { font-size: 1.15rem; font-weight: 800; color: #fff; display: flex; align-items: center; gap: 8px; margin-bottom: 1rem; }
.booster-nav-item { display: flex; align-items: center; gap: 10px; padding: 0.65rem 0.85rem; border-radius: 8px; color: var(--text-muted); text-decoration: none; font-size: 0.9rem; font-weight: 600; transition: all 0.15s; }
.booster-nav-item:hover, .booster-nav-item.active { background: rgba(56, 189, 248, 0.15); color: var(--accent); }

.booster-main { flex: 1; display: flex; flex-direction: column; min-width: 0; }
.booster-header { padding: 1rem 2rem; border-bottom: 1px solid var(--border); display: flex; justify-content: space-between; align-items: center; background: rgba(9, 13, 22, 0.7); }
.booster-title { font-size: 1.35rem; font-weight: 700; }
.booster-content { padding: 2rem; flex: 1; overflow-y: auto; }

.card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 12px; padding: 1.5rem; backdrop-filter: blur(12px); box-shadow: 0 4px 24px rgba(0,0,0,0.3); }
.table-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.25rem; gap: 1rem; flex-wrap: wrap; }
.search-input { background: rgba(15, 23, 42, 0.8); border: 1px solid var(--border); color: var(--text-main); padding: 0.6rem 1rem; border-radius: 8px; font-size: 0.88rem; outline: none; width: 260px; }
.search-input:focus { border-color: var(--accent); }

.btn { padding: 0.6rem 1.2rem; border-radius: 8px; font-size: 0.85rem; font-weight: 700; cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; gap: 6px; border: none; transition: all 0.2s; }
.btn-primary { background: var(--accent); color: #090d16; }
.btn-primary:hover { background: var(--accent-hover); }
.btn-danger { background: var(--danger); color: #fff; }
.btn-secondary { background: rgba(255,255,255,0.06); color: var(--text-main); border: 1px solid var(--border); }
.btn-sm { padding: 4px 8px; font-size: 0.75rem; border-radius: 6px; }

.custom-table { width: 100%; border-collapse: collapse; text-align: left; }
.custom-table th { padding: 0.75rem 0.6rem; font-size: 0.75rem; color: var(--text-muted); text-transform: uppercase; font-weight: 700; border-bottom: 1px solid var(--border); }
.custom-table td { padding: 0.85rem 0.6rem; font-size: 0.88rem; border-bottom: 1px solid rgba(255,255,255,0.04); }
.actions-cell { display: flex; gap: 6px; }

.badge { padding: 3px 8px; border-radius: 6px; font-size: 0.75rem; font-weight: 700; }
.badge-success { background: rgba(16, 185, 129, 0.15); color: var(--success); }
.badge-warning { background: rgba(245, 158, 11, 0.15); color: var(--warning); }

.form-group { margin-bottom: 1.25rem; display: flex; flex-direction: column; gap: 6px; }
.form-group label { font-size: 0.85rem; font-weight: 600; color: var(--text-muted); }
.form-control { background: rgba(15, 23, 42, 0.8); border: 1px solid var(--border); color: var(--text-main); padding: 0.75rem 1rem; border-radius: 8px; font-size: 0.9rem; outline: none; }
.form-control:focus { border-color: var(--accent); }
`

	// Embedded Admin Base HTML Template
	AdminBaseHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{ .Title }} • TGo Booster</title>
  <style>` + AdminCSS + `</style>
</head>
<body>
  <aside class="booster-sidebar">
    <div class="booster-brand">⚡ TGo Booster</div>
    <nav class="booster-nav">
      {{ range .Modules }}
        <a href="{{ .Path }}" class="booster-nav-item {{ if .IsActive }}active{{ end }}">
          <span>📋</span>
          <span>{{ .Title }}</span>
        </a>
      {{ end }}
    </nav>
  </aside>

  <main class="booster-main">
    <header class="booster-header">
      <div class="booster-title">{{ .Title }}</div>
      <div><span class="badge badge-success">Tenant: {{ .TenantSlug }}</span></div>
    </header>

    <section class="booster-content">
      {{ .BodyContent }}
    </section>
  </main>
</body>
</html>`
)
