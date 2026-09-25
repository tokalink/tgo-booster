package cb

import (
	"bytes"
	"embed"
	"html/template"
)

//go:embed templates/* static/*
var boosterFS embed.FS

var (
	ThemeCSS        template.CSS
	masterTemplates *template.Template
	loginTemplate   *template.Template
	tableTemplate   *template.Template
	formTemplate    *template.Template
)

func init() {
	// 1. Load CSS from static/css/admin.css
	cssData, err := boosterFS.ReadFile("static/css/admin.css")
	if err != nil {
		panic("failed to read admin.css: " + err.Error())
	}
	ThemeCSS = template.CSS(cssData)

	// 2. Parse master layout and components
	var parseErr error
	masterTemplates, parseErr = template.New("layout.html").ParseFS(boosterFS,
		"templates/layout.html",
		"templates/sidebar.html",
		"templates/topbar.html",
	)
	if parseErr != nil {
		panic("failed to parse masterTemplates: " + parseErr.Error())
	}

	// 3. Parse login template
	loginTemplate, parseErr = template.New("login.html").ParseFS(boosterFS,
		"templates/login.html",
	)
	if parseErr != nil {
		panic("failed to parse loginTemplate: " + parseErr.Error())
	}

	// 4. Parse table template
	tableTemplate, parseErr = template.New("table.html").ParseFS(boosterFS,
		"templates/table.html",
	)
	if parseErr != nil {
		panic("failed to parse tableTemplate: " + parseErr.Error())
	}

	// 5. Parse form template
	formTemplate, parseErr = template.New("form.html").ParseFS(boosterFS,
		"templates/form.html",
	)
	if parseErr != nil {
		panic("failed to parse formTemplate: " + parseErr.Error())
	}
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
	err := loginTemplate.ExecuteTemplate(&buf, "login.html", data)
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
