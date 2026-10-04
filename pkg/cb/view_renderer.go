package cb

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// DefaultFuncMap returns standard template helper functions available across all custom views
func DefaultFuncMap() template.FuncMap {
	return template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"hasPrefix": strings.HasPrefix,
		"hasSuffix": strings.HasSuffix,
		"contains":  strings.Contains,
		"toUpper":   strings.ToUpper,
		"toLower":   strings.ToLower,
	}
}

type cachedTemplate struct {
	modTime time.Time
	tmpl    *template.Template
}

var (
	viewCacheMu sync.RWMutex
	viewCache   = make(map[string]*cachedTemplate)
)

// ResolveFilePath locates an HTML template file across candidate directories
func ResolveFilePath(path string) (string, error) {
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		if stat, err := os.Stat(cleanPath); err == nil && !stat.IsDir() {
			return cleanPath, nil
		}
		return "", fmt.Errorf("file not found: %s", cleanPath)
	}

	candidates := []string{
		cleanPath,
		filepath.Join(".", cleanPath),
		filepath.Join("starter", cleanPath),
		filepath.Join("..", cleanPath),
		filepath.Join("../starter", cleanPath),
	}

	for _, cand := range candidates {
		if stat, err := os.Stat(cand); err == nil && !stat.IsDir() {
			return cand, nil
		}
	}

	return "", fmt.Errorf("template file not found: %s", path)
}

// ParseFileToHTML loads, compiles, caches, and executes an external HTML template file with data
func ParseFileToHTML(filePath string, data interface{}) (template.HTML, error) {
	resolved, err := ResolveFilePath(filePath)
	if err != nil {
		return "", err
	}

	stat, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	modTime := stat.ModTime()

	viewCacheMu.RLock()
	cached, found := viewCache[resolved]
	viewCacheMu.RUnlock()

	var tmpl *template.Template
	if found && cached.modTime.Equal(modTime) {
		tmpl = cached.tmpl
	} else {
		contentBytes, err := os.ReadFile(resolved)
		if err != nil {
			return "", err
		}

		t, err := template.New(filepath.Base(resolved)).Funcs(DefaultFuncMap()).Parse(string(contentBytes))
		if err != nil {
			return "", fmt.Errorf("failed to parse template file %s: %w", resolved, err)
		}

		viewCacheMu.Lock()
		viewCache[resolved] = &cachedTemplate{
			modTime: modTime,
			tmpl:    t,
		}
		viewCacheMu.Unlock()
		tmpl = t
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", resolved, err)
	}

	return template.HTML(buf.String()), nil
}

// ParseFSToHTML executes a template from an embedded fs.FS
func ParseFSToHTML(fsys fs.FS, pattern string, data interface{}) (template.HTML, error) {
	t, err := template.New(filepath.Base(pattern)).Funcs(DefaultFuncMap()).ParseFS(fsys, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to parse fs template %s: %w", pattern, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute fs template %s: %w", pattern, err)
	}
	return template.HTML(buf.String()), nil
}

// RenderViewFile renders an external HTML template file inside the full Booster admin layout
func (c *Controller) RenderViewFile(w http.ResponseWriter, r *http.Request, pageTitle, filePath string, data interface{}) error {
	contentHTML, err := ParseFileToHTML(filePath, data)
	if err != nil {
		http.Error(w, "Template Error: "+err.Error(), http.StatusInternalServerError)
		return err
	}
	c.RenderView(w, r, pageTitle, contentHTML)
	return nil
}

// RenderViewFS renders an embedded HTML template file inside the full Booster admin layout
func (c *Controller) RenderViewFS(w http.ResponseWriter, r *http.Request, pageTitle string, fsys fs.FS, pattern string, data interface{}) error {
	contentHTML, err := ParseFSToHTML(fsys, pattern, data)
	if err != nil {
		http.Error(w, "Template Error: "+err.Error(), http.StatusInternalServerError)
		return err
	}
	c.RenderView(w, r, pageTitle, contentHTML)
	return nil
}

// RenderViewFile renders an external HTML template file inside the full Booster admin layout
func (e *Engine) RenderViewFile(w http.ResponseWriter, r *http.Request, pageTitle, filePath string, data interface{}) error {
	contentHTML, err := ParseFileToHTML(filePath, data)
	if err != nil {
		http.Error(w, "Template Error: "+err.Error(), http.StatusInternalServerError)
		return err
	}
	e.RenderLayout(w, r, pageTitle, contentHTML)
	return nil
}

// RenderViewFS renders an embedded HTML template file inside the full Booster admin layout
func (e *Engine) RenderViewFS(w http.ResponseWriter, r *http.Request, pageTitle string, fsys fs.FS, pattern string, data interface{}) error {
	contentHTML, err := ParseFSToHTML(fsys, pattern, data)
	if err != nil {
		http.Error(w, "Template Error: "+err.Error(), http.StatusInternalServerError)
		return err
	}
	e.RenderLayout(w, r, pageTitle, contentHTML)
	return nil
}

// AddCustomPageWithFile registers a custom admin page directly backed by an external HTML template file
func (e *Engine) AddCustomPageWithFile(path, title, icon, filePath string, dataProvider func(r *http.Request) interface{}, addToMenu ...bool) *Engine {
	return e.AddCustomPage(path, title, icon, func(w http.ResponseWriter, r *http.Request) template.HTML {
		var data interface{}
		if dataProvider != nil {
			data = dataProvider(r)
		}
		contentHTML, err := ParseFileToHTML(filePath, data)
		if err != nil {
			return template.HTML(fmt.Sprintf("<div class='cb-alert cb-alert-danger'><strong>Template Error:</strong> %s</div>", err.Error()))
		}
		return contentHTML
	}, addToMenu...)
}
