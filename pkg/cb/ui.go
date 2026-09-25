package cb

const (
	// Embedded Self-Hosted Admin CSS
	ThemeCSS = `
:root {
  --bg-body: #090d16;
  --bg-sidebar: #0f172a;
  --bg-topbar: rgba(15, 23, 42, 0.92);
  --bg-card: rgba(18, 24, 38, 0.85);
  --border: rgba(255, 255, 255, 0.08);
  --border-focus: rgba(56, 189, 248, 0.4);
  --accent: #38bdf8;
  --accent-hover: #7dd3fc;
  --accent-glow: rgba(56, 189, 248, 0.15);
  --text-main: #f8fafc;
  --text-muted: #94a3b8;
  --text-dim: #64748b;
  --danger: #ef4444;
  --success: #10b981;
  --warning: #f59e0b;
  --sidebar-w: 250px;
  --topbar-h: 62px;
}

* { box-sizing: border-box; margin: 0; padding: 0; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
body { background: var(--bg-body); color: var(--text-main); min-height: 100vh; display: flex; overflow-x: hidden; }

/* SIDEBAR */
.cb-sidebar { width: var(--sidebar-w); background: var(--bg-sidebar); border-right: 1px solid var(--border); display: flex; flex-direction: column; position: fixed; top: 0; bottom: 0; left: 0; z-index: 50; }
.cb-brand { padding: 1.25rem 1.25rem; border-bottom: 1px solid var(--border); display: flex; align-items: center; gap: 10px; text-decoration: none; }
.cb-brand-icon { font-size: 1.4rem; }
.cb-brand-text { font-size: 1.1rem; font-weight: 800; color: #fff; letter-spacing: -0.3px; }
.cb-brand-badge { font-size: 0.65rem; background: var(--accent-glow); color: var(--accent); padding: 2px 6px; border-radius: 4px; font-weight: 700; margin-left: auto; }

.cb-nav { flex: 1; padding: 1rem 0.75rem; display: flex; flex-direction: column; gap: 4px; overflow-y: auto; }
.cb-nav-heading { font-size: 0.68rem; font-weight: 800; color: var(--text-dim); padding: 0.75rem 0.5rem 0.25rem 0.5rem; text-transform: uppercase; letter-spacing: 0.8px; }
.cb-nav-item { display: flex; align-items: center; gap: 10px; padding: 0.65rem 0.85rem; border-radius: 8px; color: var(--text-muted); text-decoration: none; font-size: 0.88rem; font-weight: 600; transition: all 0.15s; }
.cb-nav-item:hover { background: rgba(255, 255, 255, 0.05); color: var(--text-main); }
.cb-nav-item.active { background: var(--accent-glow); color: var(--accent); font-weight: 700; border-left: 3px solid var(--accent); }
.cb-nav-icon { font-size: 1rem; width: 20px; text-align: center; }
.cb-nav-badge { margin-left: auto; font-size: 0.7rem; padding: 2px 6px; border-radius: 9999px; font-weight: 700; }

/* MAIN CONTENT */
.cb-main { margin-left: var(--sidebar-w); flex: 1; display: flex; flex-direction: column; min-height: 100vh; min-width: 0; }

/* TOPBAR */
.cb-topbar { height: var(--topbar-h); background: var(--bg-topbar); backdrop-filter: blur(12px); border-bottom: 1px solid var(--border); padding: 0 2rem; display: flex; justify-content: space-between; align-items: center; position: sticky; top: 0; z-index: 40; }
.cb-breadcrumb { display: flex; align-items: center; gap: 8px; font-size: 0.85rem; color: var(--text-muted); }
.cb-breadcrumb span.current { color: var(--text-main); font-weight: 600; }
.cb-topbar-right { display: flex; align-items: center; gap: 1.25rem; }

.cb-user-pill { display: flex; align-items: center; gap: 10px; text-decoration: none; color: inherit; padding: 4px 8px; border-radius: 8px; transition: background 0.15s; }
.cb-user-pill:hover { background: rgba(255, 255, 255, 0.05); }
.cb-avatar { width: 34px; height: 34px; border-radius: 8px; background: linear-gradient(135deg, #38bdf8 0%, #3b82f6 100%); color: #090d16; display: flex; align-items: center; justify-content: center; font-weight: 800; font-size: 0.8rem; }
.cb-user-info { display: flex; flex-direction: column; }
.cb-user-name { font-size: 0.82rem; font-weight: 700; color: var(--text-main); }
.cb-user-role { font-size: 0.68rem; color: var(--text-dim); }

.btn-logout { background: transparent; border: 1px solid var(--border); color: var(--text-muted); padding: 6px 12px; border-radius: 6px; font-size: 0.78rem; font-weight: 600; cursor: pointer; text-decoration: none; display: flex; align-items: center; gap: 5px; transition: all 0.2s; }
.btn-logout:hover { color: var(--danger); border-color: var(--danger); background: rgba(239, 68, 68, 0.1); }

/* CONTENT CONTAINER */
.cb-content { padding: 2rem; flex: 1; }

/* STATS GRID */
.cb-stats-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 1.5rem; margin-bottom: 2rem; }
.cb-stat-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 12px; padding: 1.5rem; display: flex; align-items: center; gap: 1.25rem; backdrop-filter: blur(12px); }
.cb-stat-icon { width: 50px; height: 50px; border-radius: 12px; display: flex; align-items: center; justify-content: center; font-size: 1.5rem; background: var(--accent-glow); color: var(--accent); }
.cb-stat-info { display: flex; flex-direction: column; gap: 2px; }
.cb-stat-title { font-size: 0.78rem; font-weight: 700; color: var(--text-muted); text-transform: uppercase; }
.cb-stat-value { font-size: 1.5rem; font-weight: 800; color: var(--text-main); }
.cb-stat-sub { font-size: 0.72rem; color: var(--success); font-weight: 600; }

/* DATA GRID CARD */
.cb-card { background: var(--bg-card); border: 1px solid var(--border); border-radius: 14px; padding: 1.75rem; backdrop-filter: blur(12px); box-shadow: 0 8px 32px rgba(0,0,0,0.3); }
.cb-toolbar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.5rem; gap: 1rem; flex-wrap: wrap; }
.cb-search { background: rgba(15, 23, 42, 0.8); border: 1px solid var(--border); color: var(--text-main); padding: 0.65rem 1rem; border-radius: 8px; font-size: 0.88rem; outline: none; width: 280px; transition: all 0.2s; }
.cb-search:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); }

/* BUTTONS */
.btn { padding: 0.65rem 1.25rem; border-radius: 8px; font-size: 0.85rem; font-weight: 700; cursor: pointer; text-decoration: none; display: inline-flex; align-items: center; gap: 6px; border: none; transition: all 0.2s; }
.btn-primary { background: linear-gradient(135deg, #38bdf8 0%, #3b82f6 100%); color: #090d16; }
.btn-primary:hover { opacity: 0.95; transform: translateY(-1px); }
.btn-secondary { background: rgba(255, 255, 255, 0.05); color: var(--text-main); border: 1px solid var(--border); }
.btn-secondary:hover { background: rgba(255, 255, 255, 0.1); }
.btn-danger { background: var(--danger); color: #fff; }
.btn-danger:hover { opacity: 0.9; }
.btn-sm { padding: 5px 10px; font-size: 0.75rem; border-radius: 6px; }

/* TABLES */
.cb-table { width: 100%; border-collapse: collapse; text-align: left; }
.cb-table th { padding: 0.85rem 0.75rem; font-size: 0.75rem; color: var(--text-dim); text-transform: uppercase; font-weight: 800; border-bottom: 1px solid var(--border); letter-spacing: 0.5px; }
.cb-table td { padding: 0.95rem 0.75rem; font-size: 0.88rem; border-bottom: 1px solid rgba(255,255,255,0.04); }
.cb-table tr:hover td { background: rgba(255,255,255,0.02); }
.cb-actions { display: flex; gap: 6px; justify-content: flex-end; }

/* BADGES */
.cb-badge { padding: 4px 8px; border-radius: 6px; font-size: 0.72rem; font-weight: 700; display: inline-block; }
.cb-badge-success { background: rgba(16, 185, 129, 0.15); color: var(--success); }
.cb-badge-warning { background: rgba(245, 158, 11, 0.15); color: var(--warning); }
.cb-badge-danger { background: rgba(239, 68, 68, 0.15); color: var(--danger); }

/* FORMS */
.cb-form-group { margin-bottom: 1.35rem; display: flex; flex-direction: column; gap: 6px; }
.cb-form-group label { font-size: 0.85rem; font-weight: 700; color: var(--text-muted); }
.cb-form-control { background: rgba(15, 23, 42, 0.85); border: 1px solid var(--border); color: var(--text-main); padding: 0.75rem 1rem; border-radius: 8px; font-size: 0.9rem; outline: none; transition: all 0.2s; }
.cb-form-control:focus { border-color: var(--accent); box-shadow: 0 0 0 3px var(--accent-glow); }
.cb-help-text { font-size: 0.75rem; color: var(--text-dim); }

/* AUTH / LOGIN */
.cb-auth-wrap { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 1.5rem; width: 100%; }
.cb-auth-card { width: 100%; max-width: 440px; background: var(--bg-card); border: 1px solid var(--border); border-radius: 16px; padding: 2.5rem; backdrop-filter: blur(16px); box-shadow: 0 8px 32px rgba(0,0,0,0.4); }
.cb-auth-header { text-align: center; margin-bottom: 2rem; }
.cb-auth-header h1 { font-size: 1.75rem; font-weight: 800; margin-bottom: 0.4rem; color: #fff; }
.cb-auth-header p { color: var(--text-muted); font-size: 0.9rem; }
`

	// Login View Template
	LoginHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Admin Login • {{ .AppName }}</title>
  <style>` + ThemeCSS + `</style>
</head>
<body>
  <div class="cb-auth-wrap">
    <div class="cb-auth-card">
      <div class="cb-auth-header">
        <div style="font-size: 2.5rem; margin-bottom: 0.5rem;">⚡</div>
        <h1>{{ .AppName }}</h1>
        <p>Sign in to your administration dashboard</p>
      </div>

      {{ if .Error }}
        <div style="background: rgba(239,68,68,0.15); border: 1px solid rgba(239,68,68,0.3); color: #ef4444; padding: 0.75rem; border-radius: 8px; font-size: 0.85rem; margin-bottom: 1.25rem;">
          {{ .Error }}
        </div>
      {{ end }}

      <form method="POST" action="{{ .LoginAction }}">
        <div class="cb-form-group">
          <label>Email Address</label>
          <input type="email" name="email" class="cb-form-control" placeholder="admin@tgo.io" value="{{ .DefaultEmail }}" required autofocus />
        </div>

        <div class="cb-form-group">
          <label>Password</label>
          <input type="password" name="password" class="cb-form-control" placeholder="••••••••••••" value="admin123" required />
        </div>

        <button type="submit" class="btn btn-primary" style="width: 100%; justify-content: center; margin-top: 1rem; padding: 0.85rem;">
          Sign In to Dashboard
        </button>
      </form>

      <div style="margin-top: 2rem; text-align: center; font-size: 0.75rem; color: var(--text-dim);">
        Powered by TGo Booster • Sub-Millisecond Go Engine
      </div>
    </div>
  </div>
</body>
</html>`

	// App Master Layout Template (Sidebar, Topbar, Body)
	MasterHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{ .PageTitle }} • {{ .AppName }}</title>
  <style>` + ThemeCSS + `</style>
</head>
<body>
  <!-- SIDEBAR -->
  <aside class="cb-sidebar">
    <a href="{{ .AdminPath }}" class="cb-brand">
      <span class="cb-brand-icon">⚡</span>
      <span class="cb-brand-text">{{ .AppName }}</span>
      <span class="cb-brand-badge">v5.6</span>
    </a>

    <nav class="cb-nav">
      <div class="cb-nav-heading">MAIN NAVIGATION</div>
      <a href="{{ .AdminPath }}" class="cb-nav-item {{ if eq .ActivePath .AdminPath }}active{{ end }}">
        <span class="cb-nav-icon">📊</span>
        <span>Dashboard</span>
      </a>

      <div class="cb-nav-heading">MODULES</div>
      {{ range .MenuItems }}
        <a href="{{ .Path }}" class="cb-nav-item {{ if .IsActive }}active{{ end }}">
          <span class="cb-nav-icon">{{ .Icon }}</span>
          <span>{{ .Title }}</span>
          {{ if .Badge }}
            <span class="cb-nav-badge" style="background: {{ .BadgeColor }}; color: #fff;">{{ .Badge }}</span>
          {{ end }}
        </a>
      {{ end }}

      <div class="cb-nav-heading">SETTINGS</div>
      <a href="{{ .AdminPath }}/users" class="cb-nav-item {{ if eq .ActivePath (print .AdminPath "/users") }}active{{ end }}">
        <span class="cb-nav-icon">👥</span>
        <span>Users Management</span>
      </a>
      <a href="{{ .AdminPath }}/logs" class="cb-nav-item {{ if eq .ActivePath (print .AdminPath "/logs") }}active{{ end }}">
        <span class="cb-nav-icon">📜</span>
        <span>Audit Trail Logs</span>
      </a>
    </nav>
  </aside>

  <!-- MAIN WRAPPER -->
  <div class="cb-main">
    <!-- TOPBAR -->
    <header class="cb-topbar">
      <div class="cb-breadcrumb">
        <span>Admin</span>
        <span>/</span>
        <span class="current">{{ .PageTitle }}</span>
      </div>

      <div class="cb-topbar-right">
        <div class="cb-user-pill">
          <div class="cb-avatar">{{ .UserInitials }}</div>
          <div class="cb-user-info">
            <span class="cb-user-name">{{ .User.Name }}</span>
            <span class="cb-user-role">{{ .User.RoleName }}</span>
          </div>
        </div>

        <a href="{{ .AdminPath }}/logout" class="btn-logout" title="Sign Out">
          <span>Logout</span>
          <span>🚪</span>
        </a>
      </div>
    </header>

    <!-- CONTENT -->
    <main class="cb-content">
      {{ .Content }}
    </main>
  </div>
</body>
</html>`
)
