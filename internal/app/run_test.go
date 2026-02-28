package app

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunTextOutputSingleModule(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "single-module")
	configPath := writeNoAnalyzerConfig(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--config", configPath, fixture}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "healthy: no findings") {
		t.Fatalf("expected healthy output, got %q", output)
	}
	if !strings.Contains(output, "mode: module") {
		t.Fatalf("expected module mode, got %q", output)
	}
	if !strings.Contains(output, "100/100 (A)") {
		t.Fatalf("expected score, got %q", output)
	}
}

func TestRunJSONOutputWorkspace(t *testing.T) {
	fixture := filepath.Join("..", "..", "testdata", "fixtures", "workspace")
	configPath := writeNoAnalyzerConfig(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	code := Run(context.Background(), []string{"--config", configPath, "--format=json", fixture}, &stdout, &stderr)
	if code != ExitSuccess {
		t.Fatalf("expected success, got %d: %s", code, stderr.String())
	}

	var payload struct {
		SchemaVersion int `json:"schemaVersion"`
		Project       struct {
			Mode         string   `json:"mode"`
			PackageCount int      `json:"packageCount"`
			ModuleRoots  []string `json:"moduleRoots"`
		} `json:"project"`
		Diagnostics []any `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal json: %v", err)
	}
	if payload.SchemaVersion != 1 {
		t.Fatalf("expected schema version 1, got %d", payload.SchemaVersion)
	}
	if payload.Project.Mode != "workspace" {
		t.Fatalf("expected workspace mode, got %q", payload.Project.Mode)
	}
	if payload.Project.PackageCount != 2 {
		t.Fatalf("expected 2 packages, got %d", payload.Project.PackageCount)
	}
	if len(payload.Project.ModuleRoots) != 2 {
		t.Fatalf("expected 2 module roots, got %d", len(payload.Project.ModuleRoots))
	}
	if len(payload.Diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %d", len(payload.Diagnostics))
	}
}

func writeNoAnalyzerConfig(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), ".go-doctor.yaml")
	if err := os.WriteFile(path, []byte("analyzers:\n  thirdParty: false\n  custom: false\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
