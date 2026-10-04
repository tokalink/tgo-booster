package cb

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tokalink/tgo/pkg/transport/connect"
)

// Page represents a customizable public or CMS page with SEO, marketing pixels, and custom routing
type Page struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Slug      string `json:"slug"`
	RouteURL  string `json:"route_url"`
	Content   string `json:"content"`
	Excerpt   string `json:"excerpt,omitempty"`
	Template  string `json:"template"` // "default", "landing", "article", "blank"
	Status    string `json:"status"`   // "published", "draft", "archived"
	Author    string `json:"author,omitempty"`
	Views     int64  `json:"views"`

	// SEO Suite
	MetaTitle       string `json:"meta_title,omitempty"`
	MetaDescription string `json:"meta_description,omitempty"`
	MetaKeywords    string `json:"meta_keywords,omitempty"`
	CanonicalURL    string `json:"canonical_url,omitempty"`
	Robots          string `json:"robots,omitempty"`     // "index, follow", "noindex, follow", etc.
	SchemaJSON      string `json:"schema_json,omitempty"` // JSON-LD structured data

	// Social Sharing & OpenGraph
	OGTitle     string `json:"og_title,omitempty"`
	OGDescription string `json:"og_description,omitempty"`
	OGImage     string `json:"og_image,omitempty"`
	OGType      string `json:"og_type,omitempty"`       // "website", "article", "product"
	TwitterCard string `json:"twitter_card,omitempty"`  // "summary_large_image", "summary"

	// Marketing Pixels & Script Injection
	PixelMetaID   string `json:"pixel_meta_id,omitempty"`   // Meta/Facebook Pixel ID (e.g. 1234567890)
	PixelGTMID    string `json:"pixel_gtm_id,omitempty"`    // GTM (GTM-XXXX) or GA4 (G-XXXX)
	PixelTikTokID string `json:"pixel_tiktok_id,omitempty"` // TikTok Pixel ID (e.g. C12345678)
	HeaderScripts string `json:"header_scripts,omitempty"` // Raw <head> scripts (Hotjar, clarity, etc.)
	FooterScripts string `json:"footer_scripts,omitempty"` // Raw before </body> scripts
	CustomCSS     string `json:"custom_css,omitempty"`     // Custom page CSS rules

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

var (
	pagesPersistenceMu sync.Mutex
)

// normalizeRoute ensures leading slash and clean path
func normalizeRoute(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for strings.Contains(path, "//") {
		path = strings.ReplaceAll(path, "//", "/")
	}
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

func (e *Engine) getPagesPersistencePath() string {
	candidates := []string{
		"data/pages.json",
		"starter/data/pages.json",
		"../starter/data/pages.json",
	}
	for _, c := range candidates {
		dir := filepath.Dir(c)
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			return c
		}
	}
	return "data/pages.json"
}

// LoadDynamicPages restores custom pages from pages.json or seeds defaults
func (e *Engine) LoadDynamicPages() {
	pagesPersistenceMu.Lock()
	defer pagesPersistenceMu.Unlock()

	p := e.getPagesPersistencePath()
	data, err := os.ReadFile(p)
	if err == nil && len(data) > 0 {
		var items []*Page
		if err := json.Unmarshal(data, &items); err == nil && len(items) > 0 {
			e.pages = items
			return
		}
	}

	// Initialize default pages if missing or empty
	if len(e.pages) == 0 {
		e.initDefaultPages()
		_ = e.savePagesUnsafe()
	}
}

// SavePages persists custom pages to disk
func (e *Engine) SavePages() error {
	pagesPersistenceMu.Lock()
	defer pagesPersistenceMu.Unlock()
	return e.savePagesUnsafe()
}

func (e *Engine) savePagesUnsafe() error {
	p := e.getPagesPersistencePath()
	_ = os.MkdirAll(filepath.Dir(p), 0755)
	data, err := json.MarshalIndent(e.pages, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0644)
}

// initDefaultPages generates seed pages with complete SEO, pixels, and routes
func (e *Engine) initDefaultPages() {
	now := time.Now().Format("2006-01-02 15:04:05")

	e.pages = []*Page{
		{
			ID:          "1",
			Title:       "About Our Company",
			Slug:        "about-us",
			RouteURL:    "/about-us",
			Template:    "default",
			Status:      "published",
			Author:      "Editorial Team",
			Views:       1240,
			CreatedAt:   now,
			UpdatedAt:   now,
			Excerpt:     "Discover our company journey, leadership team, and mission to deliver enterprise software with extreme developer productivity.",
			MetaTitle:   "About Us | Enterprise Next-Gen Cloud Platform",
			MetaDescription: "Learn about our company history, mission, leadership, and high-performance cloud architecture built for scale.",
			MetaKeywords: "enterprise platform, golang, cloud native, company vision, about us",
			CanonicalURL: "https://yourdomain.com/about-us",
			Robots:      "index, follow",
			OGTitle:     "About Our Company - Building the Future of Enterprise Software",
			OGDescription: "Learn how our engineering team delivers mission-critical platforms with unparalleled speed and reliability.",
			OGImage:     "https://images.unsplash.com/photo-1497366216548-37526070297c?w=1200&auto=format&fit=crop&q=80",
			OGType:      "website",
			TwitterCard: "summary_large_image",
			SchemaJSON: `{
  "@context": "https://schema.org",
  "@type": "AboutPage",
  "name": "About Our Company",
  "description": "Learn about our company history, mission, leadership, and enterprise architecture.",
  "publisher": {
    "@type": "Organization",
    "name": "TGo Enterprise"
  }
}`,
			Content: `<div style="text-align: center; margin-bottom: 3rem;">
  <span style="display: inline-block; padding: 6px 16px; border-radius: 9999px; background: rgba(56, 189, 248, 0.15); color: #0284c7; font-size: 0.85rem; font-weight: 700; text-transform: uppercase; letter-spacing: 0.05em; margin-bottom: 1rem;">Our Heritage & Vision</span>
  <h2 style="font-size: 2.25rem; font-weight: 800; color: #0f172a; line-height: 1.25; margin-bottom: 1rem;">Empowering Global Enterprises with Ultra-Fast Systems</h2>
  <p style="font-size: 1.15rem; color: #64748b; max-width: 680px; margin: 0 auto; line-height: 1.6;">We architect high-performance frameworks, automated administration studios, and cloud platforms engineered for zero downtime and exponential developer velocity.</p>
</div>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 1.75rem; margin-bottom: 3.5rem;">
  <div style="background: #ffffff; border: 1px solid #e2e8f0; border-radius: 16px; padding: 2rem; box-shadow: 0 4px 20px -2px rgba(0,0,0,0.05);">
    <div style="width: 48px; height: 48px; border-radius: 12px; background: rgba(14, 165, 233, 0.12); color: #0284c7; display: flex; align-items: center; justify-content: center; font-size: 1.5rem; margin-bottom: 1.25rem;">⚡</div>
    <h3 style="font-size: 1.2rem; font-weight: 700; color: #0f172a; margin-bottom: 0.5rem;">Hyper-Performance</h3>
    <p style="color: #64748b; font-size: 0.95rem; line-height: 1.6;">Engineered with native Go and HTTP/2 cleartext streaming for sub-millisecond response latency under massive concurrency.</p>
  </div>
  <div style="background: #ffffff; border: 1px solid #e2e8f0; border-radius: 16px; padding: 2rem; box-shadow: 0 4px 20px -2px rgba(0,0,0,0.05);">
    <div style="width: 48px; height: 48px; border-radius: 12px; background: rgba(168, 85, 247, 0.12); color: #9333ea; display: flex; align-items: center; justify-content: center; font-size: 1.5rem; margin-bottom: 1.25rem;">🛡️</div>
    <h3 style="font-size: 1.2rem; font-weight: 700; color: #0f172a; margin-bottom: 0.5rem;">Enterprise RBAC</h3>
    <p style="color: #64748b; font-size: 0.95rem; line-height: 1.6;">Granular role-based security matrices, audit trails, and automatic CSRF protection out of the box.</p>
  </div>
  <div style="background: #ffffff; border: 1px solid #e2e8f0; border-radius: 16px; padding: 2rem; box-shadow: 0 4px 20px -2px rgba(0,0,0,0.05);">
    <div style="width: 48px; height: 48px; border-radius: 12px; background: rgba(34, 197, 94, 0.12); color: #16a34a; display: flex; align-items: center; justify-content: center; font-size: 1.5rem; margin-bottom: 1.25rem;">🚀</div>
    <h3 style="font-size: 1.2rem; font-weight: 700; color: #0f172a; margin-bottom: 0.5rem;">Rapid Deployment</h3>
    <p style="color: #64748b; font-size: 0.95rem; line-height: 1.6;">Instant database schema reflection, code generation, and single static binary distribution across any OS.</p>
  </div>
</div>

<div style="background: linear-gradient(135deg, #0f172a 0%, #1e293b 100%); color: #ffffff; border-radius: 20px; padding: 3rem; text-align: center;">
  <h3 style="font-size: 1.85rem; font-weight: 800; margin-bottom: 0.75rem;">Ready to Accelerate Your Architecture?</h3>
  <p style="color: #94a3b8; font-size: 1.05rem; max-width: 600px; margin: 0 auto 2rem auto;">Connect with our enterprise engineering team to review benchmarks, architectural blueprints, and migration paths.</p>
  <a href="/promo-special" style="display: inline-block; background: #38bdf8; color: #0f172a; font-weight: 700; padding: 12px 28px; border-radius: 10px; text-decoration: none; box-shadow: 0 4px 14px rgba(56, 189, 248, 0.4);">Explore Special Offers &rarr;</a>
</div>`,
		},
		{
			ID:          "2",
			Title:       "Special Promo Campaign 2026",
			Slug:        "promo-special",
			RouteURL:    "/promo-special",
			Template:    "landing",
			Status:      "published",
			Author:      "Growth Marketing",
			Views:       3890,
			CreatedAt:   now,
			UpdatedAt:   now,
			Excerpt:     "Exclusive 50% discount on TGo Booster Enterprise licenses. Complete with custom landing pages, pixel conversion tracking, and multi-cloud sync.",
			MetaTitle:   "Special Promo 2026: Save 50% on Enterprise Booster Platform",
			MetaDescription: "Limited time promotional offer. Unlock unlimited CRUD modules, custom landing pages, TikTok/Meta/Google conversion pixels, and 24/7 dedicated SLA.",
			MetaKeywords: "special promo, enterprise discount, flash sale, tgo booster, web framework",
			CanonicalURL: "https://yourdomain.com/promo-special",
			Robots:      "index, follow",
			OGTitle:     "🔥 Exclusive 50% Off: Enterprise Booster Studio Platform",
			OGDescription: "Unlock full-suite enterprise code generation, custom routes, and marketing pixel tracking.",
			OGImage:     "https://images.unsplash.com/photo-1551434678-e076c223a692?w=1200&auto=format&fit=crop&q=80",
			OGType:      "product",
			TwitterCard: "summary_large_image",
			PixelMetaID: "987654321012345",
			PixelGTMID:  "GTM-PROMO26",
			PixelTikTokID: "CTIKTOK2026EXAMPLE",
			HeaderScripts: `<!-- Custom Campaign Verification Script -->
<script>console.log('[Promo Campaign] 2026 Flash Sale Activated');</script>`,
			SchemaJSON: `{
  "@context": "https://schema.org",
  "@type": "Product",
  "name": "TGo Booster Enterprise Suite",
  "image": "https://images.unsplash.com/photo-1551434678-e076c223a692?w=1200&auto=format&fit=crop&q=80",
  "description": "Enterprise code generation, custom routing, and marketing pixel tracking platform.",
  "offers": {
    "@type": "Offer",
    "priceCurrency": "USD",
    "price": "199.00",
    "availability": "https://schema.org/InStock",
    "validThrough": "2026-12-31"
  }
}`,
			Content: `<div style="text-align: center; max-width: 820px; margin: 0 auto 3rem auto;">
  <div style="display: inline-flex; align-items: center; gap: 8px; background: rgba(245, 158, 11, 0.15); border: 1px solid rgba(245, 158, 11, 0.3); padding: 6px 16px; border-radius: 9999px; margin-bottom: 1.5rem;">
    <span style="width: 8px; height: 8px; border-radius: 9999px; background: #f59e0b; animation: pulse 2s infinite;"></span>
    <span style="font-size: 0.85rem; font-weight: 700; color: #d97706; text-transform: uppercase; letter-spacing: 0.05em;">Limited Time Flash Deal &bull; Save 50%</span>
  </div>
  <h1 style="font-size: 3rem; font-weight: 900; line-height: 1.15; color: #0f172a; margin-bottom: 1.25rem;">
    Build Custom Web Apps <span style="background: linear-gradient(135deg, #0284c7 0%, #9333ea 100%); -webkit-background-clip: text; -webkit-text-fill-color: transparent;">10x Faster</span> with Full SEO & Pixels
  </h1>
  <p style="font-size: 1.2rem; color: #64748b; line-height: 1.6; margin-bottom: 2rem;">
    Equip your engineering and marketing squads with custom routes, embedded Meta/TikTok/GTM pixels, OpenGraph previews, and instant CRUD controllers.
  </p>
  <div style="display: flex; gap: 12px; justify-content: center; flex-wrap: wrap;">
    <a href="#claim" style="background: linear-gradient(135deg, #0284c7 0%, #2563eb 100%); color: #ffffff; padding: 14px 32px; border-radius: 12px; font-weight: 700; font-size: 1.05rem; text-decoration: none; box-shadow: 0 10px 25px -5px rgba(37, 99, 235, 0.4);">Claim Offer Now &rarr;</a>
    <a href="/about-us" style="background: #ffffff; color: #334155; border: 1px solid #cbd5e1; padding: 14px 28px; border-radius: 12px; font-weight: 600; font-size: 1.05rem; text-decoration: none;">Explore Platform</a>
  </div>
</div>

<div style="background: #ffffff; border: 2px solid #e2e8f0; border-radius: 24px; padding: 2.5rem; max-width: 920px; margin: 0 auto; box-shadow: 0 20px 40px -15px rgba(0,0,0,0.07);">
  <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 2rem; align-items: center;">
    <div>
      <div style="font-size: 0.9rem; font-weight: 700; color: #0284c7; text-transform: uppercase; margin-bottom: 4px;">What's Included:</div>
      <ul style="list-style: none; padding: 0; margin: 0; display: flex; flex-direction: column; gap: 10px;">
        <li style="display: flex; align-items: center; gap: 10px; font-size: 0.95rem; color: #1e293b;">
          <span style="color: #16a34a; font-weight: bold;">&#10003;</span> Custom Route URL Vanity Paths
        </li>
        <li style="display: flex; align-items: center; gap: 10px; font-size: 0.95rem; color: #1e293b;">
          <span style="color: #16a34a; font-weight: bold;">&#10003;</span> Meta / Facebook Pixel Auto-Injection
        </li>
        <li style="display: flex; align-items: center; gap: 10px; font-size: 0.95rem; color: #1e293b;">
          <span style="color: #16a34a; font-weight: bold;">&#10003;</span> Google Tag Manager & GA4 Events
        </li>
        <li style="display: flex; align-items: center; gap: 10px; font-size: 0.95rem; color: #1e293b;">
          <span style="color: #16a34a; font-weight: bold;">&#10003;</span> TikTok Pixel Base & PageView Tracking
        </li>
        <li style="display: flex; align-items: center; gap: 10px; font-size: 0.95rem; color: #1e293b;">
          <span style="color: #16a34a; font-weight: bold;">&#10003;</span> Live Google SERP & OpenGraph Previews
        </li>
      </ul>
    </div>
    <div id="claim" style="background: #f8fafc; border-radius: 16px; padding: 2rem; text-align: center; border: 1px solid #e2e8f0;">
      <div style="font-size: 0.85rem; color: #64748b; text-decoration: line-through;">Normal Price: $399</div>
      <div style="font-size: 2.75rem; font-weight: 900; color: #0f172a; margin: 4px 0 10px 0;">$199 <span style="font-size: 1rem; font-weight: 500; color: #64748b;">/ lifetime</span></div>
      <button type="button" onclick="alert('Demo: Promo Claimed! Pixel event fbq & ttq fired.');" style="width: 100%; background: #16a34a; color: #ffffff; font-weight: 700; padding: 12px; border-radius: 10px; border: none; cursor: pointer; font-size: 1rem; box-shadow: 0 4px 14px rgba(22, 163, 74, 0.3);">
        Activate Special License
      </button>
      <div style="font-size: 0.78rem; color: #94a3b8; margin-top: 8px;">Instant digital delivery &bull; 30-day money-back guarantee</div>
    </div>
  </div>
</div>`,
		},
		{
			ID:          "3",
			Title:       "Privacy Policy & Compliance",
			Slug:        "privacy",
			RouteURL:    "/privacy",
			Template:    "article",
			Status:      "published",
			Author:      "Legal & Compliance",
			Views:       450,
			CreatedAt:   now,
			UpdatedAt:   now,
			Excerpt:     "Our commitment to privacy, GDPR compliance, transparent data processing, and user data rights.",
			MetaTitle:   "Privacy Policy & GDPR Compliance Guidelines",
			MetaDescription: "Read our comprehensive privacy policy regarding data collection, encryption, cookies, and international data protection standards.",
			MetaKeywords: "privacy policy, gdpr, data compliance, terms, security",
			CanonicalURL: "https://yourdomain.com/privacy",
			Robots:      "index, follow",
			OGTitle:     "Privacy Policy & User Data Rights",
			OGDescription: "Transparent guidelines on how user data is securely processed, encrypted, and respected.",
			OGImage:     "https://images.unsplash.com/photo-1450133064473-71024230f91b?w=1200&auto=format&fit=crop&q=80",
			OGType:      "article",
			TwitterCard: "summary",
			Content: `<p style="font-size: 1.1rem; line-height: 1.8; color: #334155; margin-bottom: 1.5rem;">
  Welcome to our Privacy Policy. Your privacy and the confidentiality of your data are fundamental to how we engineer and operate our software. This document explains what information is collected, how it is safeguarded, and how you can exercise your statutory rights under GDPR, CCPA, and global privacy standards.
</p>

<h3 style="font-size: 1.4rem; font-weight: 700; color: #0f172a; margin: 2rem 0 1rem 0;">1. Information We Collect</h3>
<p style="line-height: 1.8; color: #334155; margin-bottom: 1.25rem;">
  When you register an administrative account or visit our hosted services, we may collect technical metadata such as browser user agent, IP address, and session timestamps. These are strictly processed to enforce role-based access control, prevent unauthorized intrusion, and audit security events.
</p>

<h3 style="font-size: 1.4rem; font-weight: 700; color: #0f172a; margin: 2rem 0 1rem 0;">2. Analytics & Marketing Pixels</h3>
<p style="line-height: 1.8; color: #334155; margin-bottom: 1.25rem;">
  Certain promotional and informational pages utilize anonymized conversion telemetry (such as Meta Pixel, Google Tag Manager, and TikTok Pixel) to measure audience engagement. You can freely opt out of tracking via your browser's Global Privacy Control (GPC) or Do-Not-Track headers.
</p>

<h3 style="font-size: 1.4rem; font-weight: 700; color: #0f172a; margin: 2rem 0 1rem 0;">3. Data Retention & Deletion</h3>
<p style="line-height: 1.8; color: #334155; margin-bottom: 1.25rem;">
  Audit logs and user profiles are stored using enterprise encryption standards. You may request full account deletion and data export at any time by contacting our data protection officer at <code style="background: #f1f5f9; padding: 2px 6px; border-radius: 4px; color: #0f172a;">privacy@yourdomain.com</code>.
</p>`,
		},
	}
}

// FindPage searches for a page by ID, Slug, or exact RouteURL
func (e *Engine) FindPage(idOrSlugOrRoute string) *Page {
	needle := strings.TrimSpace(idOrSlugOrRoute)
	if needle == "" {
		return nil
	}
	normRoute := normalizeRoute(needle)

	for _, p := range e.pages {
		if p.ID == needle || strings.EqualFold(p.Slug, needle) {
			return p
		}
		if normRoute != "" && (normalizeRoute(p.RouteURL) == normRoute || normalizeRoute("/p/"+p.Slug) == normRoute || normalizeRoute("/page/"+p.Slug) == normRoute) {
			return p
		}
	}
	return nil
}

// UpsertPage saves or inserts a Page, normalizing route URLs and updating routes
func (e *Engine) UpsertPage(p *Page) error {
	if p.Title == "" {
		return fmt.Errorf("page title is required")
	}
	if p.RouteURL == "" {
		if p.Slug == "" {
			p.Slug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p.Title), " ", "-"))
		}
		p.Slug = strings.ToLower(p.Slug)
		p.Slug = strings.TrimPrefix(p.Slug, "/")
		p.RouteURL = "/" + p.Slug
	} else {
		p.RouteURL = normalizeRoute(p.RouteURL)
		if p.Slug == "" {
			if p.RouteURL == "/" {
				p.Slug = "home"
			} else {
				p.Slug = strings.TrimPrefix(p.RouteURL, "/")
			}
		} else {
			p.Slug = strings.ToLower(p.Slug)
			p.Slug = strings.TrimPrefix(p.Slug, "/")
		}
	}

	// If this page is designated as root homepage ("/"), revert any other page that currently has RouteURL == "/" to its slug
	if p.RouteURL == "/" {
		for _, existing := range e.pages {
			if existing.ID != p.ID && existing.RouteURL == "/" {
				existing.RouteURL = "/" + existing.Slug
			}
		}
	}

	if p.Template == "" {
		p.Template = "default"
	}
	if p.Status == "" {
		p.Status = "published"
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	p.UpdatedAt = now

	found := false
	for i, existing := range e.pages {
		if (p.ID != "" && existing.ID == p.ID) || (p.ID == "" && existing.Slug == p.Slug) {
			if p.ID == "" {
				p.ID = existing.ID
			}
			p.CreatedAt = existing.CreatedAt
			p.Views = existing.Views
			e.pages[i] = p
			found = true
			break
		}
	}

	if !found {
		if p.ID == "" {
			p.ID = fmt.Sprintf("%d", time.Now().UnixNano()/1e6)
		}
		p.CreatedAt = now
		e.pages = append(e.pages, p)
	}

	// Register dynamically on server if active
	if e.server != nil && p.RouteURL != "" {
		e.registerSinglePageRoute(e.server, p)
	}

	return e.SavePages()
}

// DeletePage removes a page by ID
func (e *Engine) DeletePage(id string) error {
	idx := -1
	for i, p := range e.pages {
		if p.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("page not found")
	}

	e.pages = append(e.pages[:idx], e.pages[idx+1:]...)
	return e.SavePages()
}

// RegisterPageRoutes binds all public page routes to the transport server
func (e *Engine) RegisterPageRoutes(server connect.Server) {
	// 1. Dynamic request middleware so runtime-added pages work instantly without restart
	server.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if e.ServePublicPage(w, r) {
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// 2. Universal slug prefix routes: /p/ and /page/
	server.Register("/p/", http.HandlerFunc(e.handlePublicPagePrefix))
	server.Register("/page/", http.HandlerFunc(e.handlePublicPagePrefix))

	// 3. Individual exact RouteURLs
	for _, p := range e.pages {
		e.registerSinglePageRoute(server, p)
	}
}

func (e *Engine) registerSinglePageRoute(server connect.Server, p *Page) {
	if p.RouteURL == "" || strings.HasPrefix(p.RouteURL, e.AdminPath) {
		return
	}
	norm := normalizeRoute(p.RouteURL)
	if norm == "/" {
		server.Register("/", http.HandlerFunc(e.handleExactPageRoute))
		return
	}
	server.Register(norm, http.HandlerFunc(e.handleExactPageRoute))
	server.Register(norm+"/", http.HandlerFunc(e.handleExactPageRoute))
}

// ServePublicPage allows standalone or middleware-based public page dispatching
func (e *Engine) ServePublicPage(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if strings.HasPrefix(path, e.AdminPath) {
		return false
	}

	var page *Page
	if strings.HasPrefix(path, "/p/") {
		slug := strings.TrimPrefix(path, "/p/")
		slug = strings.Trim(slug, "/")
		page = e.FindPage(slug)
	} else if strings.HasPrefix(path, "/page/") {
		slug := strings.TrimPrefix(path, "/page/")
		slug = strings.Trim(slug, "/")
		page = e.FindPage(slug)
	} else {
		page = e.FindPage(path)
	}

	if page == nil {
		return false
	}

	// Check draft/archive status and authentication
	isAdmin := e.Auth != nil && e.Auth.GetSessionUser(r) != nil
	if page.Status != "published" && !isAdmin {
		// Not published and not admin -> 404
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<!DOCTYPE html><html><head><title>404 Not Found</title></head><body style=\"font-family:sans-serif;text-align:center;padding:4rem;\"><h1>404 - Page Not Found</h1><p>The requested page is not currently published.</p></body></html>"))
		return true
	}

	isPreview := page.Status != "published" || r.URL.Query().Get("preview") == "1"
	if !isPreview {
		page.Views++
	}

	htmlContent := e.RenderPublicPageHTML(page, isPreview, r.Host)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(htmlContent))
	return true
}

func (e *Engine) handleExactPageRoute(w http.ResponseWriter, r *http.Request) {
	if !e.ServePublicPage(w, r) {
		http.NotFound(w, r)
	}
}

func (e *Engine) handlePublicPagePrefix(w http.ResponseWriter, r *http.Request) {
	if !e.ServePublicPage(w, r) {
		http.NotFound(w, r)
	}
}

// ============================================================================
// PUBLIC HTML RENDERER WITH FULL SEO, OPENGRAPH, PIXELS & TEMPLATES
// ============================================================================

// RenderPublicPageHTML builds an SEO-optimized HTML document with pixels & styles via landingTemplate
func (e *Engine) RenderPublicPageHTML(p *Page, isPreview bool, reqHost string) string {
	// Title fallback
	title := p.MetaTitle
	if title == "" {
		title = p.Title
	}

	// Description fallback
	desc := p.MetaDescription
	if desc == "" {
		desc = p.Excerpt
	}

	// Canonical URL
	canonical := p.CanonicalURL
	if canonical == "" && reqHost != "" {
		canonical = fmt.Sprintf("https://%s%s", reqHost, p.RouteURL)
	}

	// Robots
	robots := p.Robots
	if robots == "" {
		robots = "index, follow"
	}

	// OG Title & Desc
	ogTitle := p.OGTitle
	if ogTitle == "" {
		ogTitle = title
	}
	ogDesc := p.OGDescription
	if ogDesc == "" {
		ogDesc = desc
	}
	ogType := p.OGType
	if ogType == "" {
		ogType = "website"
	}
	twitterCard := p.TwitterCard
	if twitterCard == "" {
		twitterCard = "summary_large_image"
	}

	data := map[string]interface{}{
		"Page":            p,
		"AppName":         e.AppName,
		"AdminPath":       e.AdminPath,
		"Settings":        e.GetSettings(),
		"IsPreview":       isPreview,
		"CanonicalURL":    canonical,
		"MetaTitle":       title,
		"MetaDescription": desc,
		"MetaKeywords":    p.MetaKeywords,
		"Robots":          robots,
		"OGTitle":         ogTitle,
		"OGDescription":   ogDesc,
		"OGImage":         p.OGImage,
		"OGType":          ogType,
		"TwitterCard":     twitterCard,
		"SchemaJSON":      p.SchemaJSON,
		"PixelMetaID":     p.PixelMetaID,
		"PixelGTMID":      p.PixelGTMID,
		"PixelTikTokID":   p.PixelTikTokID,
		"HeaderScripts":   template.HTML(p.HeaderScripts),
		"FooterScripts":   template.HTML(p.FooterScripts),
		"CustomCSS":       template.CSS(p.CustomCSS),
		"Content":         template.HTML(p.Content),
		"CurrentYear":     time.Now().Year(),
	}

	renderedBytes, err := RenderLandingLayout(data)
	if err != nil {
		return fmt.Sprintf("<!DOCTYPE html><html><head><title>%s</title></head><body><!-- Render Error: %s -->\n%s</body></html>", html.EscapeString(title), err.Error(), p.Content)
	}
	return string(renderedBytes)
}

// ============================================================================
// BOOSTER ADMIN PAGES STUDIO HANDLERS
// ============================================================================

func (e *Engine) handlePages(w http.ResponseWriter, r *http.Request) {
	if e.Auth != nil {
		if user := e.Auth.GetSessionUser(r); user == nil {
			http.Redirect(w, r, e.AdminPath+"/login", http.StatusSeeOther)
			return
		}
	}

	// Check permission
	currUser := e.Auth.GetSessionUser(r)
	if currUser != nil {
		roleID := currUser.PrivilegeID
		if roleID == "" {
			roleID = currUser.RoleName
		}
		role := e.FindRole(roleID)
		if role != nil && !role.CanAccess("pages", "read") {
			http.Error(w, "Forbidden: insufficient privilege to access Pages module", http.StatusForbidden)
			return
		}
	}

	// Compute statistics
	totalCount := len(e.pages)
	publishedCount := 0
	pixelActiveCount := 0
	seoReadyCount := 0

	for _, p := range e.pages {
		if p.Status == "published" {
			publishedCount++
		}
		if p.PixelMetaID != "" || p.PixelGTMID != "" || p.PixelTikTokID != "" {
			pixelActiveCount++
		}
		if p.MetaTitle != "" && p.MetaDescription != "" && p.CanonicalURL != "" {
			seoReadyCount++
		}
	}

	data := map[string]interface{}{
		"AppName":          e.AppName,
		"AdminPath":        e.AdminPath,
		"CurrentUser":      currUser,
		"Pages":            e.pages,
		"TotalCount":       totalCount,
		"PublishedCount":   publishedCount,
		"PixelActiveCount": pixelActiveCount,
		"SeoReadyCount":    seoReadyCount,
	}

	contentHTML, err := RenderPagesContent(data)
	if err != nil {
		http.Error(w, "Failed to render pages content: "+err.Error(), http.StatusInternalServerError)
		return
	}

	e.RenderLayout(w, r, "Custom Pages & SEO Studio", contentHTML)
}

func (e *Engine) handlePageAPI(w http.ResponseWriter, r *http.Request) {
	if e.Auth != nil {
		if user := e.Auth.GetSessionUser(r); user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(e.pages)
		return
	}

	p := e.FindPage(id)
	if p == nil {
		http.Error(w, `{"error":"page not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

func (e *Engine) handlePageSave(w http.ResponseWriter, r *http.Request) {
	if e.Auth != nil {
		if user := e.Auth.GetSessionUser(r); user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
	}

	var req Page
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"invalid JSON: %s"}`, err.Error()), http.StatusBadRequest)
			return
		}
	} else {
		_ = r.ParseForm()
		req.ID = r.FormValue("id")
		req.Title = r.FormValue("title")
		req.Slug = r.FormValue("slug")
		req.RouteURL = r.FormValue("route_url")
		req.Content = r.FormValue("content")
		req.Excerpt = r.FormValue("excerpt")
		req.Template = r.FormValue("template")
		req.Status = r.FormValue("status")
		req.Author = r.FormValue("author")
		req.MetaTitle = r.FormValue("meta_title")
		req.MetaDescription = r.FormValue("meta_description")
		req.MetaKeywords = r.FormValue("meta_keywords")
		req.CanonicalURL = r.FormValue("canonical_url")
		req.Robots = r.FormValue("robots")
		req.SchemaJSON = r.FormValue("schema_json")
		req.OGTitle = r.FormValue("og_title")
		req.OGDescription = r.FormValue("og_description")
		req.OGImage = r.FormValue("og_image")
		req.OGType = r.FormValue("og_type")
		req.TwitterCard = r.FormValue("twitter_card")
		req.PixelMetaID = r.FormValue("pixel_meta_id")
		req.PixelGTMID = r.FormValue("pixel_gtm_id")
		req.PixelTikTokID = r.FormValue("pixel_tiktok_id")
		req.HeaderScripts = r.FormValue("header_scripts")
		req.FooterScripts = r.FormValue("footer_scripts")
		req.CustomCSS = r.FormValue("custom_css")
	}

	if strings.TrimSpace(req.Title) == "" {
		http.Error(w, `{"error":"Page title is required"}`, http.StatusBadRequest)
		return
	}

	if err := e.UpsertPage(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to save page: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	e.LogAudit(r, "PAGE_UPDATE", "Pages Studio", fmt.Sprintf("Saved custom page '%s' (Route: %s, ID: %s)", req.Title, req.RouteURL, req.ID))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":   true,
		"id":        req.ID,
		"slug":      req.Slug,
		"route_url": req.RouteURL,
	})
}

func (e *Engine) handlePageDelete(w http.ResponseWriter, r *http.Request) {
	if e.Auth != nil {
		if user := e.Auth.GetSessionUser(r); user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		id = req.ID
	}

	if id == "" {
		http.Error(w, `{"error":"page ID is required"}`, http.StatusBadRequest)
		return
	}

	if err := e.DeletePage(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete page: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	e.LogAudit(r, "PAGE_DELETE", "Pages Studio", fmt.Sprintf("Deleted custom page ID: %s", id))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
	})
}

func (e *Engine) handlePagePreview(w http.ResponseWriter, r *http.Request) {
	if e.Auth != nil {
		if user := e.Auth.GetSessionUser(r); user == nil {
			http.Redirect(w, r, e.AdminPath+"/login", http.StatusSeeOther)
			return
		}
	}

	id := r.URL.Query().Get("id")
	page := e.FindPage(id)
	if page == nil {
		http.NotFound(w, r)
		return
	}

	htmlContent := e.RenderPublicPageHTML(page, true, r.Host)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(htmlContent))
}

func (e *Engine) handlePageGenerateAI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if e.Auth != nil {
		if user := e.Auth.GetSessionUser(r); user == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}
	}

	var req AIPageGenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Invalid request JSON: " + err.Error(),
		})
		return
	}

	if strings.TrimSpace(req.Prompt) == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   "Prompt description cannot be empty",
		})
		return
	}

	result, err := e.GeneratePageWithAI(req)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	e.LogAudit(r, "AI_PAGE_GENERATE", "Pages Studio", fmt.Sprintf("Generated custom page with AI: '%s' (Template: %s)", result.Title, result.Template))

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"page":    result,
	})
}

