package cb

import (
	"bytes"
	"embed"
	"html/template"
	"strings"
)

//go:embed templates/* static/*
var boosterFS embed.FS

var (
	ThemeCSS                template.CSS
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
)

func init() {
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"hasPrefix": strings.HasPrefix,
	}

	// 1. Load CSS from static/css/admin.css
	cssData, err := boosterFS.ReadFile("static/css/admin.css")
	if err != nil {
		panic("failed to read admin.css: " + err.Error())
	}
	ThemeCSS = template.CSS(cssData)

	// 2. Parse master layout and components
	var parseErr error
	masterTemplates, parseErr = template.New("main.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/layouts/main.html",
		"templates/components/sidebar.html",
		"templates/components/topbar.html",
		"templates/components/modals.html",
	)
	if parseErr != nil {
		panic("failed to parse masterTemplates: " + parseErr.Error())
	}

	// 3. Parse login auth layout
	loginTemplate, parseErr = template.New("auth.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/layouts/auth.html",
	)
	if parseErr != nil {
		panic("failed to parse loginTemplate: " + parseErr.Error())
	}

	// 4. Parse table page template
	tableTemplate, parseErr = template.New("table.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/table.html",
	)
	if parseErr != nil {
		panic("failed to parse tableTemplate: " + parseErr.Error())
	}

	// 5. Parse form page template
	formTemplate, parseErr = template.New("form.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/form.html",
	)
	if parseErr != nil {
		panic("failed to parse formTemplate: " + parseErr.Error())
	}

	// 6. Parse module generator studio template
	moduleGeneratorTemplate, parseErr = template.New("module_generator.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/module_generator.html",
	)
	if parseErr != nil {
		panic("failed to parse moduleGeneratorTemplate: " + parseErr.Error())
	}

	// 7. Parse privileges & roles matrix template
	privilegesTemplate, parseErr = template.New("privileges.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/privileges.html",
	)
	if parseErr != nil {
		panic("failed to parse privilegesTemplate: " + parseErr.Error())
	}

	// 8. Parse menus studio template
	menusTemplate, parseErr = template.New("menus.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/menus.html",
	)
	if parseErr != nil {
		panic("failed to parse menusTemplate: " + parseErr.Error())
	}

	// 9. Parse custom pages & SEO studio template
	pagesTemplate, parseErr = template.New("pages.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/pages.html",
	)
	if parseErr != nil {
		panic("failed to parse pagesTemplate: " + parseErr.Error())
	}

	// 10. Parse platform settings studio template
	settingsTemplate, parseErr = template.New("settings.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/settings.html",
	)
	if parseErr != nil {
		panic("failed to parse settingsTemplate: " + parseErr.Error())
	}

	// 11. Parse API & tokens studio template
	apiGeneratorTemplate, parseErr = template.New("api_generator.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/api_generator.html",
	)
	if parseErr != nil {
		panic("failed to parse apiGeneratorTemplate: " + parseErr.Error())
	}

	// 12. Parse executive dashboard template
	dashboardTemplate, parseErr = template.New("dashboard.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/pages/dashboard.html",
	)
	// 13. Parse landing and public pages layout template
	landingTemplate, parseErr = template.New("landing.html").Funcs(funcMap).ParseFS(boosterFS,
		"templates/layouts/landing.html",
	)
	if parseErr != nil {
		panic("failed to parse landingTemplate: " + parseErr.Error())
	}
}

// RenderLandingLayout executes the landing and public pages layout template
func RenderLandingLayout(data map[string]interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := landingTemplate.Execute(&buf, data)
	return buf.Bytes(), err
}

// RenderMasterLayout executes the master layout with components and injected page content
func RenderMasterLayout(data map[string]interface{}) ([]byte, error) {
	data["ThemeCSS"] = ThemeCSS
	var buf bytes.Buffer
	err := masterTemplates.Execute(&buf, data)
	return buf.Bytes(), err
}

// RenderLoginTemplate executes the dedicated login screen template
func RenderLoginTemplate(data map[string]interface{}) ([]byte, error) {
	data["ThemeCSS"] = ThemeCSS
	var buf bytes.Buffer
	err := loginTemplate.ExecuteTemplate(&buf, "auth.html", data)
	return buf.Bytes(), err
}

// RenderTableContent executes the responsive CRUD data grid template
func RenderTableContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := tableTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}

// RenderFormContent executes the form builder template
func RenderFormContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := formTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}

// RenderModuleGeneratorContent executes the CRUDBooster Module Generator Studio template
func RenderModuleGeneratorContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := moduleGeneratorTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}

// RenderPrivilegesContent executes the CRUDBooster Privileges & Roles RBAC Matrix template
func RenderPrivilegesContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := privilegesTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}

// RenderMenusContent executes the CRUDBooster Menu Management Studio template
func RenderMenusContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := menusTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}

// RenderPagesContent executes the Custom Pages & SEO Studio template
func RenderPagesContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := pagesTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}

// RenderSettingsContent executes the Platform Settings Studio template
func RenderSettingsContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := settingsTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}

// RenderAPIGeneratorContent executes the API & Tokens Studio template
func RenderAPIGeneratorContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := apiGeneratorTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}

// RenderDashboardContent executes the Executive Dashboard template
func RenderDashboardContent(data map[string]interface{}) (template.HTML, error) {
	var buf bytes.Buffer
	err := dashboardTemplate.Execute(&buf, data)
	return template.HTML(buf.String()), err
}


