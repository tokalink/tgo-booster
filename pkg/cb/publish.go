package cb

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// PublishResult contains summary statistics of the publish operation
type PublishResult struct {
	Published []string
	Skipped   []string
}

// PublishTemplates copies embedded templates from boosterFS into the target directory on disk (e.g. "./templates").
// tag can be "layouts" (default), "pages", or "all".
// If force is true, existing files will be overwritten.
func PublishTemplates(targetDir string, tag string, force bool) (*PublishResult, error) {
	if targetDir == "" {
		targetDir = "templates"
	}

	tag = strings.ToLower(strings.TrimSpace(tag))
	if tag == "" {
		tag = "layouts"
	}

	result := &PublishResult{
		Published: make([]string, 0),
		Skipped:   make([]string, 0),
	}

	var matchPrefixes []string
	switch tag {
	case "layouts", "layout":
		matchPrefixes = []string{"templates/layouts", "templates/components"}
	case "landing", "public":
		matchPrefixes = []string{"templates/layouts/landing.html"}
	case "auth", "login":
		matchPrefixes = []string{"templates/layouts/auth.html"}
	case "pages", "views", "page":
		matchPrefixes = []string{"templates/pages"}
	case "css", "static", "assets":
		matchPrefixes = []string{"static/css"}
	case "all", "vendor", "full":
		matchPrefixes = []string{"templates/layouts", "templates/components", "templates/pages", "static/css"}
	default:
		return nil, fmt.Errorf("unknown publish tag '%s'. Valid tags: layouts, landing, auth, pages, css, all", tag)
	}

	err := fs.WalkDir(boosterFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}

		// Normalize slash
		normPath := filepath.ToSlash(path)

		matched := false
		for _, prefix := range matchPrefixes {
			if strings.HasPrefix(normPath, prefix) {
				matched = true
				break
			}
		}

		if !matched {
			return nil
		}

		// Calculate destination path
		var destFile string
		if strings.HasPrefix(normPath, "templates/") {
			relPath := strings.TrimPrefix(normPath, "templates/")
			destFile = filepath.Join(targetDir, filepath.FromSlash(relPath))
		} else if strings.HasPrefix(normPath, "static/") {
			destFile = filepath.FromSlash(normPath)
		} else {
			destFile = filepath.Join(targetDir, filepath.FromSlash(normPath))
		}

		// Check if destination file already exists
		if _, err := os.Stat(destFile); err == nil && !force {
			result.Skipped = append(result.Skipped, destFile)
			return nil
		}

		content, err := boosterFS.ReadFile(path)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(destFile), 0755); err != nil {
			return err
		}

		if err := os.WriteFile(destFile, content, 0644); err != nil {
			return err
		}

		result.Published = append(result.Published, destFile)
		return nil
	})

	if err != nil {
		return result, err
	}

	if len(result.Published) == 0 && len(result.Skipped) == 0 {
		return result, errors.New("no matching templates found to publish")
	}

	return result, nil
}
