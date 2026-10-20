package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestMain(m *testing.M) {
	origDir, err := os.Getwd()
	if err != nil {
		os.Exit(1)
	}
	tmpDir, err := os.MkdirTemp("", "tgo-booster-starter-test-*")
	if err != nil {
		os.Exit(1)
	}
	srcData := filepath.Join(origDir, "data")
	if info, err := os.Stat(srcData); err == nil && info.IsDir() {
		dstData := filepath.Join(tmpDir, "data")
		_ = filepath.WalkDir(srcData, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(srcData, path)
			if err != nil {
				return err
			}
			target := filepath.Join(dstData, rel)
			if d.IsDir() {
				return os.MkdirAll(target, 0755)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, b, 0644)
		})
	}
	if err := os.Chdir(tmpDir); err != nil {
		_ = os.RemoveAll(tmpDir)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.Chdir(origDir)
	_ = os.RemoveAll(tmpDir)
	os.Exit(code)
}
