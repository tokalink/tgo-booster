package booster

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/tokalink/tgo/pkg/transport/connect"
)

// Engine manages registered CRUD booster modules and mounts them to the HTTP server
type Engine struct {
	Prefix      string
	controllers []*Controller
}

// NewEngine creates a new Booster engine
func NewEngine() *Engine {
	return &Engine{
		Prefix: "/admin",
	}
}

// Register adds a booster module controller to the engine
func (e *Engine) Register(c *Controller) *Engine {
	e.controllers = append(e.controllers, c)
	return e
}

// Mount attaches all registered booster controllers to the TGo server container
func (e *Engine) Mount(server connect.Server, prefix ...string) {
	if len(prefix) > 0 && prefix[0] != "" {
		e.Prefix = prefix[0]
	}

	for _, ctrl := range e.controllers {
		ctrl.BasePath = fmt.Sprintf("%s/%s", strings.TrimSuffix(e.Prefix, "/"), ctrl.Table)
		// Register exact path and subpaths
		server.Register(ctrl.BasePath, ctrl)
		server.Register(ctrl.BasePath+"/", ctrl)
	}

	// Mount default Admin Index / Dashboard
	server.Register(e.Prefix, http.HandlerFunc(e.handleDashboard))
	server.Register(e.Prefix+"/", http.HandlerFunc(e.handleDashboard))
}

func (e *Engine) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if len(e.controllers) > 0 {
		http.Redirect(w, r, e.controllers[0].BasePath, http.StatusSeeOther)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte("<h1>⚡ TGo Booster Dashboard</h1><p>No modules registered yet.</p>"))
}
