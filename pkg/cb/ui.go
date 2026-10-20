package cb

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed templates/* static/*
var boosterFS embed.FS

var (
	ThemeCSS template.CSS

	// Configurable directories for user template and static file overrides
	CustomTemplateDir = "templates"
	CustomStaticDir   = "static"

	defaultFuncMap template.FuncMap

	// Embedded fallback templates
	masterTemplates         *template.Template
	loginTemplate           *template.Template
	tableTemplate           *template.Template
	formTemplate            *template.Template
	moduleGeneratorTemplate *template.Template
	privilegesTemplate      *template.Template
	menusTemplate           *template.Template
	pagesTemplate           *template.Template
	settingsTemplate        *template.Template
	apiGeneratorTemplate    *template.Template
	dashboardTemplate       *template.Template
	landingTemplate         *template.Template

	// Dynamic template caching for hot-reloading
	dynamicMu      sync.RWMutex
	templateCaches = make(map[string]*templateCacheEntry)
)

type templateCacheEntry struct {
	tmpl        *template.Template
	lastModTime time.Time
}

type templateFileDef struct {
	embedPath string
	localRel  string
}

var (
	masterFilesDef = []templateFileDef{
		{embedPath: "templates/layouts/main.html", localRel: "layouts/main.html"},
		{embedPath: "templates/components/sidebar.html", localRel: "components/sidebar.html"},
		{embedPath: "templates/components/topbar.html", localRel: "components/topbar.html"},
		{embedPath: "templates/components/modals.html", localRel: "components/modals.html"},
	}
	landingFilesDef = []templateFileDef{
		{embedPath: "templates/layouts/landing.html", localRel: "layouts/landing.html"},
	}
	authFilesDef = []templateFileDef{
		{embedPath: "templates/layouts/auth.html", localRel: "layouts/auth.html"},
	}
	tableFilesDef = []templateFileDef{
		{embedPath: "templates/pages/table.html", localRel: "pages/table.html"},
	}
	formFilesDef = []templateFileDef{
		{embedPath: "templates/pages/form.html", localRel: "pages/form.html"},
	}
	moduleGenFilesDef = []templateFileDef{
		{embedPath: "templates/pages/module_generator.html", localRel: "pages/module_generator.html"},
	}
	privilegesFilesDef = []templateFileDef{
		{embedPath: "templates/pages/privileges.html", localRel: "pages/privileges.html"},
	}
	menusFilesDef = []templateFileDef{
		{embedPath: "templates/pages/menus.html", localRel: "pages/menus.html"},
	}
	pagesFilesDef = []templateFileDef{
		{embedPath: "templates/pages/pages.html", localRel: "pages/pages.html"},
	}
	settingsFilesDef = []templateFileDef{
		{embedPath: "templates/pages/settings.html", localRel: "pages/settings.html"},
	}
	apiGenFilesDef = []templateFileDef{
		{embedPath: "templates/pages/api_generator.html", localRel: "pages/api_generator.html"},
	}
	dashboardFilesDef = []templateFileDef{
		{embedPath: "templates/pages/dashboard.html", localRel: "pages/dashboard.html"},
	}
)

func init() {
	defaultFuncMap = template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"safeSchemaJSON": func(s string) template.JS {
			return template.JS(sanitizeSchemaJSON(s))
		},
		"safeIcon":     SanitizeIcon,
		"safeRichHTML": SanitizeRichHTML,
		"isChecked":    isChecked,
		"hasPrefix":    strings.HasPrefix,
	}

	// 1. Load CSS from static/css/admin.css
	cssData, err := boosterFS.ReadFile("static/css/admin.css")
	if err != nil {
		panic("failed to read admin.css: " + err.Error())
	}
	ThemeCSS = template.CSS(cssData)

	// 2. Parse master layout and components
	var parseErr error
	masterTemplates, parseErr = template.New("main.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/layouts/main.html",
		"templates/components/sidebar.html",
		"templates/components/topbar.html",
		"templates/components/modals.html",
	)
	if parseErr != nil {
		panic("failed to parse masterTemplates: " + parseErr.Error())
	}

	// 3. Parse login auth layout
	loginTemplate, parseErr = template.New("auth.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/layouts/auth.html",
	)
	if parseErr != nil {
		panic("failed to parse loginTemplate: " + parseErr.Error())
	}

	// 4. Parse table page template
	tableTemplate, parseErr = template.New("table.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/table.html",
	)
	if parseErr != nil {
		panic("failed to parse tableTemplate: " + parseErr.Error())
	}

	// 5. Parse form page template
	formTemplate, parseErr = template.New("form.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/form.html",
	)
	if parseErr != nil {
		panic("failed to parse formTemplate: " + parseErr.Error())
	}

	// 6. Parse module generator studio template
	moduleGeneratorTemplate, parseErr = template.New("module_generator.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/module_generator.html",
	)
	if parseErr != nil {
		panic("failed to parse moduleGeneratorTemplate: " + parseErr.Error())
	}

	// 7. Parse privileges & roles matrix template
	privilegesTemplate, parseErr = template.New("privileges.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/privileges.html",
	)
	if parseErr != nil {
		panic("failed to parse privilegesTemplate: " + parseErr.Error())
	}

	// 8. Parse menus studio template
	menusTemplate, parseErr = template.New("menus.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/menus.html",
	)
	if parseErr != nil {
		panic("failed to parse menusTemplate: " + parseErr.Error())
	}

	// 9. Parse custom pages & SEO studio template
	pagesTemplate, parseErr = template.New("pages.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/pages.html",
	)
	if parseErr != nil {
		panic("failed to parse pagesTemplate: " + parseErr.Error())
	}

	// 10. Parse platform settings studio template
	settingsTemplate, parseErr = template.New("settings.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/settings.html",
	)
	if parseErr != nil {
		panic("failed to parse settingsTemplate: " + parseErr.Error())
	}

	// 11. Parse API & tokens studio template
	apiGeneratorTemplate, parseErr = template.New("api_generator.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/api_generator.html",
	)
	if parseErr != nil {
		panic("failed to parse apiGeneratorTemplate: " + parseErr.Error())
	}

	// 12. Parse executive dashboard template
	dashboardTemplate, parseErr = template.New("dashboard.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/pages/dashboard.html",
	)
	if parseErr != nil {
		panic("failed to parse dashboardTemplate: " + parseErr.Error())
	}

	// 13. Parse landing and public pages layout template
	landingTemplate, parseErr = template.New("landing.html").Funcs(defaultFuncMap).ParseFS(boosterFS,
		"templates/layouts/landing.html",
	)
	if parseErr != nil {
		panic("failed to parse landingTemplate: " + parseErr.Error())
	}
}

// SetCustomTemplateDir sets the directory where custom template overrides are located (default: "templates")
func SetCustomTemplateDir(dir string) {
	CustomTemplateDir = dir
}

// SetCustomStaticDir sets the directory where custom static assets are located (default: "static")
func SetCustomStaticDir(dir string) {
	CustomStaticDir = dir
}

// GetThemeCSS returns custom CSS if found on disk at static/css/admin.css, otherwise fallback to embedded ThemeCSS
func GetThemeCSS() template.CSS {
	localCSSPath := filepath.Join(CustomStaticDir, "css", "admin.css")
	if data, err := os.ReadFile(localCSSPath); err == nil && len(data) > 0 {
		return template.CSS(data)
	}
	return ThemeCSS
}

// getDynamicTemplate resolves templates with priority given to local published/custom files on disk.
// If any local files exist, it compiles them and caches them by max ModTime (enabling hot reload during editing).
// If no local files exist, it immediately returns the embedded precompiled fallback template.
func getDynamicTemplate(groupKey string, rootName string, fallback *template.Template, files []templateFileDef) (*template.Template, error) {
	hasLocal := false
	var maxMod time.Time

	for _, f := range files {
		p := filepath.Join(CustomTemplateDir, filepath.FromSlash(f.localRel))
		if stat, err := os.Stat(p); err == nil && !stat.IsDir() {
			hasLocal = true
			if stat.ModTime().After(maxMod) {
				maxMod = stat.ModTime()
			}
		} else {
			baseP := filepath.Join(CustomTemplateDir, filepath.Base(f.localRel))
			if stat2, err2 := os.Stat(baseP); err2 == nil && !stat2.IsDir() {
				hasLocal = true
				if stat2.ModTime().After(maxMod) {
					maxMod = stat2.ModTime()
				}
			}
		}
	}

	if !hasLocal {
		return fallback, nil
	}

	dynamicMu.RLock()
	cached, ok := templateCaches[groupKey]
	if ok && cached != nil && cached.tmpl != nil && cached.lastModTime.Equal(maxMod) {
		dynamicMu.RUnlock()
		return cached.tmpl, nil
	}
	dynamicMu.RUnlock()

	dynamicMu.Lock()
	defer dynamicMu.Unlock()

	// Double check under write lock
	if cached, ok := templateCaches[groupKey]; ok && cached != nil && cached.tmpl != nil && cached.lastModTime.Equal(maxMod) {
		return cached.tmpl, nil
	}

	t := template.New(rootName).Funcs(defaultFuncMap)

	for _, f := range files {
		var content []byte
		var readErr error

		p := filepath.Join(CustomTemplateDir, filepath.FromSlash(f.localRel))
		if data, err := os.ReadFile(p); err == nil {
			content = data
		} else {
			baseP := filepath.Join(CustomTemplateDir, filepath.Base(f.localRel))
			if data, err := os.ReadFile(baseP); err == nil {
				content = data
			} else {
				content, readErr = boosterFS.ReadFile(f.embedPath)
				if readErr != nil {
					return fallback, readErr
				}
			}
		}

		baseName := filepath.Base(f.embedPath)
		var targetTmpl *template.Template
		if baseName == rootName {
			targetTmpl = t
		} else {
			targetTmpl = t.New(baseName)
		}

		if _, err := targetTmpl.Parse(string(content)); err != nil {
			return fallback, fmt.Errorf("error parsing %s: %w", f.localRel, err)
		}
	}

	templateCaches[groupKey] = &templateCacheEntry{
		tmpl:        t,
		lastModTime: maxMod,
	}

	return t, nil
}

// RenderLandingLayout executes the landing and public pages layout template
func RenderLandingLayout(data map[string]interface{}) ([]byte, error) {
	tmpl, err := getDynamicTemplate("landing", "landing.html", landingTemplate, landingFilesDef)
	if err != nil {
		tmpl = landingTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return buf.Bytes(), execErr
}

// RenderMasterLayout executes the master layout with components and injected page content
func RenderMasterLayout(data map[string]interface{}) ([]byte, error) {
	data["ThemeCSS"] = GetThemeCSS()
	tmpl, err := getDynamicTemplate("master", "main.html", masterTemplates, masterFilesDef)
	if err != nil {
		tmpl = masterTemplates
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return buf.Bytes(), execErr
}

// RenderLoginTemplate executes the dedicated login screen template
func RenderLoginTemplate(data map[string]interface{}) ([]byte, error) {
	data["ThemeCSS"] = GetThemeCSS()
	tmpl, err := getDynamicTemplate("auth", "auth.html", loginTemplate, authFilesDef)
	if err != nil {
		tmpl = loginTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.ExecuteTemplate(&buf, "auth.html", data)
	if execErr != nil {
		buf.Reset()
		execErr = tmpl.Execute(&buf, data)
	}
	return buf.Bytes(), execErr
}

// RenderTableContent executes the responsive CRUD data grid template
func RenderTableContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("table", "table.html", tableTemplate, tableFilesDef)
	if err != nil {
		tmpl = tableTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}

// RenderFormContent executes the form builder template
func RenderFormContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("form", "form.html", formTemplate, formFilesDef)
	if err != nil {
		tmpl = formTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}

// RenderModuleGeneratorContent executes the CRUDBooster Module Generator Studio template
func RenderModuleGeneratorContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("module_gen", "module_generator.html", moduleGeneratorTemplate, moduleGenFilesDef)
	if err != nil {
		tmpl = moduleGeneratorTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}

// RenderPrivilegesContent executes the CRUDBooster Privileges & Roles RBAC Matrix template
func RenderPrivilegesContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("privileges", "privileges.html", privilegesTemplate, privilegesFilesDef)
	if err != nil {
		tmpl = privilegesTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}

// RenderMenusContent executes the CRUDBooster Menu Management Studio template
func RenderMenusContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("menus", "menus.html", menusTemplate, menusFilesDef)
	if err != nil {
		tmpl = menusTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}

// RenderPagesContent executes the Custom Pages & SEO Studio template
func RenderPagesContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("pages", "pages.html", pagesTemplate, pagesFilesDef)
	if err != nil {
		tmpl = pagesTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}

// RenderSettingsContent executes the Platform Settings Studio template
func RenderSettingsContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("settings", "settings.html", settingsTemplate, settingsFilesDef)
	if err != nil {
		tmpl = settingsTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}

// RenderAPIGeneratorContent executes the API & Tokens Studio template
func RenderAPIGeneratorContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("api_gen", "api_generator.html", apiGeneratorTemplate, apiGenFilesDef)
	if err != nil {
		tmpl = apiGeneratorTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}

// RenderDashboardContent executes the Executive Dashboard template
func RenderDashboardContent(data map[string]interface{}) (template.HTML, error) {
	tmpl, err := getDynamicTemplate("dashboard", "dashboard.html", dashboardTemplate, dashboardFilesDef)
	if err != nil {
		tmpl = dashboardTemplate
	}
	var buf bytes.Buffer
	execErr := tmpl.Execute(&buf, data)
	return template.HTML(buf.String()), execErr
}
