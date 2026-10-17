package cb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishTemplates(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "booster_publish_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Publish layouts
	res, err := PublishTemplates(tempDir, "layouts", false)
	if err != nil {
		t.Fatalf("PublishTemplates layouts failed: %v", err)
	}
	if len(res.Published) == 0 {
		t.Fatalf("expected published files, got 0")
	}

	mainLayout := filepath.Join(tempDir, "layouts", "main.html")
	if _, err := os.Stat(mainLayout); os.IsNotExist(err) {
		t.Fatalf("expected main.html to exist at %s", mainLayout)
	}

	// 2. Publish again without force should skip
	res2, err := PublishTemplates(tempDir, "layouts", false)
	if err != nil {
		t.Fatalf("PublishTemplates second run failed: %v", err)
	}
	if len(res2.Published) != 0 {
		t.Fatalf("expected 0 published on second run, got %d", len(res2.Published))
	}
	if len(res2.Skipped) == 0 {
		t.Fatalf("expected skipped files on second run")
	}

	// 3. Publish with force should overwrite
	res3, err := PublishTemplates(tempDir, "layouts", true)
	if err != nil {
		t.Fatalf("PublishTemplates with force failed: %v", err)
	}
	if len(res3.Published) == 0 {
		t.Fatalf("expected published files with force, got 0")
	}

	// 4. Invalid tag should return error
	_, err = PublishTemplates(tempDir, "invalid-tag", false)
	if err == nil {
		t.Fatalf("expected error for invalid tag, got nil")
	}
}

func TestDynamicTemplateOverride(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "booster_override_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Publish layouts into tempDir
	_, err = PublishTemplates(tempDir, "layouts", false)
	if err != nil {
		t.Fatalf("PublishTemplates failed: %v", err)
	}

	// Set custom template directory
	oldDir := CustomTemplateDir
	SetCustomTemplateDir(tempDir)
	defer SetCustomTemplateDir(oldDir)

	// Modify the local main.html
	mainLayout := filepath.Join(tempDir, "layouts", "main.html")
	customMarker := `<div id="custom-developer-override-marker">OVERRIDE_ACTIVE</div>`
	content, err := os.ReadFile(mainLayout)
	if err != nil {
		t.Fatalf("failed to read local main.html: %v", err)
	}
	newContent := string(content) + "\n" + customMarker
	if err := os.WriteFile(mainLayout, []byte(newContent), 0644); err != nil {
		t.Fatalf("failed to write local main.html: %v", err)
	}

	// RenderMasterLayout should render with our custom override
	data := map[string]interface{}{
		"PageTitle": "Test Title",
		"AppName":   "Test App",
	}

	out, err := RenderMasterLayout(data)
	if err != nil {
		t.Fatalf("RenderMasterLayout failed: %v", err)
	}

	if !testing.Short() {
		outStr := string(out)
		if len(outStr) == 0 {
			t.Fatalf("expected non-empty output from RenderMasterLayout")
		}
		if !strings.Contains(outStr, customMarker) {
			t.Fatalf("expected rendered output to contain custom override marker")
		}
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || (len(s) > 0 && len(substr) > 0 && stringContains(s, substr)))
}

func stringContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
