package cb

import (
	"fmt"
	"html/template"
	"net/http"
)

// DashboardHandler renders the main Starter Admin Dashboard
type DashboardHandler struct {
	Engine *Engine
}

func (d *DashboardHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user := d.Engine.Auth.GetSessionUser(r)
	if user == nil {
		http.Redirect(w, r, d.Engine.AdminPath+"/login", http.StatusSeeOther)
		return
	}

	// 1. KPI Statistic Cards
	statsHTML := fmt.Sprintf(`
		<div class="cb-stats-grid">
			<div class="cb-stat-card">
				<div class="cb-stat-icon">💰</div>
				<div class="cb-stat-info">
					<span class="cb-stat-title">Monthly Revenue</span>
					<span class="cb-stat-value">Rp 142.850.000</span>
					<span class="cb-stat-sub">↑ +18.4%% vs last month</span>
				</div>
			</div>

			<div class="cb-stat-card">
				<div class="cb-stat-icon" style="background: rgba(99,102,241,0.15); color: #818cf8;">👥</div>
				<div class="cb-stat-info">
					<span class="cb-stat-title">Active Customers</span>
					<span class="cb-stat-value">3,420</span>
					<span class="cb-stat-sub">↑ +12.1%% growth</span>
				</div>
			</div>

			<div class="cb-stat-card">
				<div class="cb-stat-icon" style="background: rgba(16,185,129,0.15); color: #10b981;">📦</div>
				<div class="cb-stat-info">
					<span class="cb-stat-title">Total Orders</span>
					<span class="cb-stat-value">1,894</span>
					<span class="cb-stat-sub">↑ 42 orders today</span>
				</div>
			</div>

			<div class="cb-stat-card">
				<div class="cb-stat-icon" style="background: rgba(245,158,11,0.15); color: #f59e0b;">⚡</div>
				<div class="cb-stat-info">
					<span class="cb-stat-title">Server Response</span>
					<span class="cb-stat-value">0.4 ms</span>
					<span class="cb-stat-sub" style="color: #38bdf8;">TGo Core Engine</span>
				</div>
			</div>
		</div>
	`)

	// 2. Recent Activity & Quick Navigation
	bodyHTML := fmt.Sprintf(`
		%s
		<div style="display: grid; grid-template-columns: 1.2fr 0.8fr; gap: 1.5rem;">
			<!-- AUDIT TRAIL LOGS -->
			<div class="cb-card">
				<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.25rem;">
					<h3 style="font-size: 1.1rem; font-weight: 800;">📋 Recent Activity (CBLogs)</h3>
					<span class="cb-badge cb-badge-success">Live Audit</span>
				</div>
				<div style="overflow-x: auto;">
					<table class="cb-table">
						<thead>
							<tr>
								<th>User</th>
								<th>Action</th>
								<th>IP Address</th>
								<th>Time</th>
							</tr>
						</thead>
						<tbody>
							<tr>
								<td><strong>%s</strong></td>
								<td>Created Product #104</td>
								<td><code>127.0.0.1</code></td>
								<td style="color: #94a3b8;">Just now</td>
							</tr>
							<tr>
								<td><strong>%s</strong></td>
								<td>Updated Order Status #882</td>
								<td><code>127.0.0.1</code></td>
								<td style="color: #94a3b8;">5 mins ago</td>
							</tr>
							<tr>
								<td><strong>System</strong></td>
								<td>Schema Migrations Verified</td>
								<td><code>127.0.0.1</code></td>
								<td style="color: #94a3b8;">15 mins ago</td>
							</tr>
						</tbody>
					</table>
				</div>
			</div>

			<!-- SYSTEM INFO & SHORTCUTS -->
			<div class="cb-card">
				<h3 style="font-size: 1.1rem; font-weight: 800; margin-bottom: 1.25rem;">⚡ Quick Actions</h3>
				<div style="display: flex; flex-direction: column; gap: 0.75rem;">
					<a href="%s/products/add" class="btn btn-primary" style="justify-content: center;">+ Create New Product</a>
					<a href="%s/users/add" class="btn btn-secondary" style="justify-content: center;">+ Register New User</a>
					<a href="%s/logs" class="btn btn-secondary" style="justify-content: center;">View Full Audit Trail</a>
				</div>

				<div style="margin-top: 1.5rem; padding-top: 1.25rem; border-top: 1px solid var(--border);">
					<div style="font-size: 0.8rem; color: var(--text-dim); margin-bottom: 4px;">SYSTEM SPECIFICATION</div>
					<div style="font-size: 0.85rem; color: var(--text-muted);">
						<div>Engine: <strong>TGo Kernel v1.0</strong></div>
						<div>Database: <strong>PostgreSQL / SQLite Native</strong></div>
						<div>Booster: <strong>CRUDBooster Go v5.6</strong></div>
					</div>
				</div>
			</div>
		</div>
	`, statsHTML, user.Name, user.Name, d.Engine.AdminPath, d.Engine.AdminPath, d.Engine.AdminPath)

	d.Engine.RenderLayout(w, r, "Dashboard Overview", template.HTML(bodyHTML))
}
