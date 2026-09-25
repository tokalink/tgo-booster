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

	// 1. KPI Statistic Cards with SVG Sparklines
	statsHTML := fmt.Sprintf(`
		<div class="cb-stats-grid">
			<!-- CARD 1: REVENUE -->
			<div class="cb-stat-card">
				<div class="stat-card-header">
					<div class="cb-stat-icon stat-blue">
						<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
					</div>
					<span class="cb-trend-badge positive">↑ +18.4%%</span>
				</div>
				<div class="cb-stat-info">
					<span class="cb-stat-title">Gross Revenue (MTD)</span>
					<span class="cb-stat-value">Rp 142.850.000</span>
					<div class="sparkline-wrap">
						<svg viewBox="0 0 120 28" class="sparkline sparkline-blue">
							<path d="M0,22 Q20,18 40,24 T80,10 T120,4" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
						</svg>
						<span class="sparkline-label">vs Rp 120.6M last month</span>
					</div>
				</div>
			</div>

			<!-- CARD 2: CUSTOMERS -->
			<div class="cb-stat-card">
				<div class="stat-card-header">
					<div class="cb-stat-icon stat-purple">
						<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/></svg>
					</div>
					<span class="cb-trend-badge positive">↑ +12.1%%</span>
				</div>
				<div class="cb-stat-info">
					<span class="cb-stat-title">Active Customers</span>
					<span class="cb-stat-value">3,420</span>
					<div class="sparkline-wrap">
						<svg viewBox="0 0 120 28" class="sparkline sparkline-purple">
							<path d="M0,25 Q30,12 60,18 T120,6" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
						</svg>
						<span class="sparkline-label">+340 new this week</span>
					</div>
				</div>
			</div>

			<!-- CARD 3: ORDERS -->
			<div class="cb-stat-card">
				<div class="stat-card-header">
					<div class="cb-stat-icon stat-green">
						<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/></svg>
					</div>
					<span class="cb-trend-badge positive">↑ 98.6%% Success</span>
				</div>
				<div class="cb-stat-info">
					<span class="cb-stat-title">Fulfilled Orders</span>
					<span class="cb-stat-value">1,894</span>
					<div class="sparkline-wrap">
						<svg viewBox="0 0 120 28" class="sparkline sparkline-green">
							<path d="M0,20 Q30,22 60,10 T120,5" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/>
						</svg>
						<span class="sparkline-label">42 orders placed today</span>
					</div>
				</div>
			</div>

			<!-- CARD 4: LATENCY -->
			<div class="cb-stat-card">
				<div class="stat-card-header">
					<div class="cb-stat-icon stat-amber">
						<svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>
					</div>
					<span class="cb-trend-badge neutral">⚡ 0.38 ms TTFB</span>
				</div>
				<div class="cb-stat-info">
					<span class="cb-stat-title">TGo Kernel Engine</span>
					<span class="cb-stat-value">Zero Alloc</span>
					<div class="sparkline-wrap">
						<span style="color: #38bdf8; font-size: 0.76rem; font-weight: 700; display: flex; align-items: center; gap: 6px;">
							<span class="beacon-dot"></span> In-Memory & HTTP/2 h2c
						</span>
					</div>
				</div>
			</div>
		</div>
	`)

	// 2. Charts & Breakdown Row
	chartsHTML := `
		<div style="display: grid; grid-template-columns: 1.35fr 0.65fr; gap: 1.5rem; margin-bottom: 2rem;">
			<!-- REVENUE VELOCITY CHART -->
			<div class="cb-card">
				<div class="cb-card-header" style="margin-bottom: 1rem;">
					<div>
						<h3 class="cb-card-title">
							<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="20" x2="18" y2="10"/><line x1="12" y1="20" x2="12" y2="4"/><line x1="6" y1="20" x2="6" y2="14"/></svg>
							Revenue Velocity & Growth Analytics
						</h3>
						<p style="font-size: 0.8rem; color: var(--text-muted); margin-top: 2px;">Monthly sales trajectory vs targeted enterprise budget</p>
					</div>
					<div style="display: flex; gap: 8px;">
						<span class="cb-badge cb-badge-primary">2026 Q1-Q3</span>
					</div>
				</div>

				<!-- SVG Area Chart -->
				<div style="position: relative; width: 100%; height: 210px; margin-top: 1rem;">
					<svg viewBox="0 0 600 200" style="width: 100%; height: 100%; overflow: visible;">
						<defs>
							<linearGradient id="chartGrad" x1="0" y1="0" x2="0" y2="1">
								<stop offset="0%" stop-color="#38bdf8" stop-opacity="0.35"/>
								<stop offset="100%" stop-color="#38bdf8" stop-opacity="0.0"/>
							</linearGradient>
							<linearGradient id="chartGradPurple" x1="0" y1="0" x2="0" y2="1">
								<stop offset="0%" stop-color="#818cf8" stop-opacity="0.25"/>
								<stop offset="100%" stop-color="#818cf8" stop-opacity="0.0"/>
							</linearGradient>
						</defs>

						<!-- Grid lines -->
						<line x1="0" y1="40" x2="600" y2="40" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
						<line x1="0" y1="90" x2="600" y2="90" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
						<line x1="0" y1="140" x2="600" y2="140" stroke="rgba(255,255,255,0.05)" stroke-dasharray="4 4" />
						<line x1="0" y1="190" x2="600" y2="190" stroke="rgba(255,255,255,0.08)" />

						<!-- Area & Path 1 (Actual Revenue) -->
						<path d="M 0,160 Q 60,140 120,130 T 240,90 T 360,65 T 480,45 T 600,20 L 600,190 L 0,190 Z" fill="url(#chartGrad)"/>
						<path d="M 0,160 Q 60,140 120,130 T 240,90 T 360,65 T 480,45 T 600,20" fill="none" stroke="#38bdf8" stroke-width="3" stroke-linecap="round"/>

						<!-- Target Line (Dashed) -->
						<path d="M 0,170 Q 150,135 300,105 T 600,55" fill="none" stroke="#818cf8" stroke-width="2" stroke-dasharray="6 6"/>

						<!-- Data Dots -->
						<circle cx="120" cy="130" r="4" fill="#38bdf8" stroke="#080c14" stroke-width="2"/>
						<circle cx="240" cy="90" r="4" fill="#38bdf8" stroke="#080c14" stroke-width="2"/>
						<circle cx="360" cy="65" r="4" fill="#38bdf8" stroke="#080c14" stroke-width="2"/>
						<circle cx="480" cy="45" r="4" fill="#38bdf8" stroke="#080c14" stroke-width="2"/>
						<circle cx="600" cy="20" r="5" fill="#38bdf8" stroke="#fff" stroke-width="2"/>
					</svg>
				</div>
				<div style="display: flex; justify-content: space-between; font-size: 0.75rem; color: var(--text-dim); margin-top: 0.75rem; padding: 0 5px;">
					<span>Jan</span>
					<span>Feb</span>
					<span>Mar</span>
					<span>Apr</span>
					<span>May</span>
					<span>Jun (Current)</span>
				</div>
			</div>

			<!-- REVENUE BY CATEGORY GAUGE -->
			<div class="cb-card">
				<div class="cb-card-header" style="margin-bottom: 1.25rem;">
					<h3 class="cb-card-title">
						<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 2a10 10 0 0 1 10 10"/></svg>
						Revenue Distribution
					</h3>
					<span style="font-size: 0.75rem; color: var(--text-muted);">By Category</span>
				</div>

				<div style="display: flex; flex-direction: column; gap: 1.1rem;">
					<!-- Category 1 -->
					<div>
						<div style="display: flex; justify-content: space-between; font-size: 0.82rem; margin-bottom: 6px;">
							<span style="font-weight: 700; color: var(--text-main);">Enterprise Software</span>
							<span style="color: var(--accent); font-weight: 800;">45%% • Rp 64.2M</span>
						</div>
						<div class="progress-track"><div class="progress-fill" style="width: 45%%; background: linear-gradient(90deg, #38bdf8, #6366f1);"></div></div>
					</div>

					<!-- Category 2 -->
					<div>
						<div style="display: flex; justify-content: space-between; font-size: 0.82rem; margin-bottom: 6px;">
							<span style="font-weight: 700; color: var(--text-main);">Cloud Subscriptions</span>
							<span style="color: #a855f7; font-weight: 800;">30%% • Rp 42.8M</span>
						</div>
						<div class="progress-track"><div class="progress-fill" style="width: 30%%; background: linear-gradient(90deg, #a855f7, #ec4899);"></div></div>
					</div>

					<!-- Category 3 -->
					<div>
						<div style="display: flex; justify-content: space-between; font-size: 0.82rem; margin-bottom: 6px;">
							<span style="font-weight: 700; color: var(--text-main);">Developer Hardware</span>
							<span style="color: #10b981; font-weight: 800;">18%% • Rp 25.7M</span>
						</div>
						<div class="progress-track"><div class="progress-fill" style="width: 18%%; background: linear-gradient(90deg, #10b981, #14b8a6);"></div></div>
					</div>

					<!-- Category 4 -->
					<div>
						<div style="display: flex; justify-content: space-between; font-size: 0.82rem; margin-bottom: 6px;">
							<span style="font-weight: 700; color: var(--text-main);">Consulting Services</span>
							<span style="color: #f59e0b; font-weight: 800;">7%% • Rp 9.9M</span>
						</div>
						<div class="progress-track"><div class="progress-fill" style="width: 7%%; background: linear-gradient(90deg, #f59e0b, #ef4444);"></div></div>
					</div>
				</div>
			</div>
		</div>
	`

	// 3. Operational Tables & Activity Log
	bodyHTML := fmt.Sprintf(`
		%s
		%s
		<div style="display: grid; grid-template-columns: 1.25fr 0.75fr; gap: 1.5rem;">
			<!-- RECENT TRANSACTIONS / ORDERS -->
			<div class="cb-card">
				<div class="cb-card-header">
					<div>
						<h3 class="cb-card-title">
							<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 2L3 6v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2V6l-3-4z"/><line x1="3" y1="6" x2="21" y2="6"/><path d="M16 10a4 4 0 0 1-8 0"/></svg>
							Recent Orders & Fulfillment
						</h3>
						<p style="font-size: 0.8rem; color: var(--text-muted); margin-top: 2px;">Real-time checkout records & order lifecycles</p>
					</div>
					<a href="%s/orders" class="btn btn-secondary btn-sm">View All Orders →</a>
				</div>
				<div style="overflow-x: auto;">
					<table class="cb-table">
						<thead>
							<tr>
								<th>Order ID</th>
								<th>Customer</th>
								<th>Amount</th>
								<th>Status</th>
								<th style="text-align: right;">Action</th>
							</tr>
						</thead>
						<tbody>
							<tr>
								<td><strong style="color: var(--accent);">#ORD-9024</strong></td>
								<td>
									<div style="display: flex; align-items: center; gap: 8px;">
										<div class="table-avatar" style="background: #38bdf8;">BC</div>
										<span>Budi Cahyono</span>
									</div>
								</td>
								<td><strong>Rp 4.850.000</strong></td>
								<td><span class="cb-badge cb-badge-success">● Completed</span></td>
								<td style="text-align: right;">
									<a href="%s/orders" class="btn btn-secondary btn-sm" title="View details">👁️</a>
								</td>
							</tr>
							<tr>
								<td><strong style="color: var(--accent);">#ORD-9023</strong></td>
								<td>
									<div style="display: flex; align-items: center; gap: 8px;">
										<div class="table-avatar" style="background: #a855f7;">SW</div>
										<span>Siti Wulandari</span>
									</div>
								</td>
								<td><strong>Rp 12.400.000</strong></td>
								<td><span class="cb-badge cb-badge-primary">● In Transit</span></td>
								<td style="text-align: right;">
									<a href="%s/orders" class="btn btn-secondary btn-sm" title="View details">👁️</a>
								</td>
							</tr>
							<tr>
								<td><strong style="color: var(--accent);">#ORD-9022</strong></td>
								<td>
									<div style="display: flex; align-items: center; gap: 8px;">
										<div class="table-avatar" style="background: #10b981;">AR</div>
										<span>Ahmad Ridwan</span>
									</div>
								</td>
								<td><strong>Rp 1.250.000</strong></td>
								<td><span class="cb-badge cb-badge-warning">● Processing</span></td>
								<td style="text-align: right;">
									<a href="%s/orders" class="btn btn-secondary btn-sm" title="View details">👁️</a>
								</td>
							</tr>
						</tbody>
					</table>
				</div>
			</div>

			<!-- AUDIT TRAIL LOGS & SHORTCUTS -->
			<div class="cb-card">
				<div class="cb-card-header">
					<h3 class="cb-card-title">
						<svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/><polyline points="10 9 9 9 8 9"/></svg>
						Live Audit (CBLogs)
					</h3>
					<span class="cb-badge cb-badge-success">Live Engine</span>
				</div>

				<div class="activity-timeline">
					<div class="timeline-item">
						<div class="timeline-dot dot-blue"></div>
						<div class="timeline-content">
							<div class="timeline-header">
								<span class="timeline-user">%s</span>
								<span class="timeline-time">Just now</span>
							</div>
							<p class="timeline-desc">Published new SKU item: <strong>MacBook Pro M3 Max</strong></p>
						</div>
					</div>

					<div class="timeline-item">
						<div class="timeline-dot dot-green"></div>
						<div class="timeline-content">
							<div class="timeline-header">
								<span class="timeline-user">System Scheduler</span>
								<span class="timeline-time">8 mins ago</span>
							</div>
							<p class="timeline-desc">Daily ledger & inventory reconciliation verified (0 errors)</p>
						</div>
					</div>

					<div class="timeline-item">
						<div class="timeline-dot dot-purple"></div>
						<div class="timeline-content">
							<div class="timeline-header">
								<span class="timeline-user">%s</span>
								<span class="timeline-time">22 mins ago</span>
							</div>
							<p class="timeline-desc">Logged in from secure corporate gateway <code>127.0.0.1</code></p>
						</div>
					</div>
				</div>

				<div style="margin-top: 1.5rem; padding-top: 1.25rem; border-top: 1px solid var(--border);">
					<h4 style="font-size: 0.85rem; font-weight: 800; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.6px; margin-bottom: 0.85rem;">⚡ Quick Actions</h4>
					<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 0.6rem;">
						<a href="%s/products/add" class="btn btn-primary btn-sm" style="justify-content: center;">+ New Product</a>
						<a href="%s/customers/add" class="btn btn-secondary btn-sm" style="justify-content: center;">+ New Customer</a>
					</div>
				</div>
			</div>
		</div>
	`, statsHTML, chartsHTML, d.Engine.AdminPath, d.Engine.AdminPath, d.Engine.AdminPath, d.Engine.AdminPath, user.Name, user.Name, d.Engine.AdminPath, d.Engine.AdminPath)

	d.Engine.RenderLayout(w, r, "Executive Dashboard", template.HTML(bodyHTML))
}

