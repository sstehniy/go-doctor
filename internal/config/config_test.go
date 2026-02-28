package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrecedence(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, ".go-doctor.yaml")
	ymlPath := filepath.Join(dir, ".go-doctor.yml")
	jsonPath := filepath.Join(dir, ".go-doctor.json")

	if err := os.WriteFile(yamlPath, []byte("output:\n  format: json\n"), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
	if err := os.WriteFile(ymlPath, []byte("output:\n  format: sarif\n"), 0o644); err != nil {
		t.Fatalf("write yml: %v", err)
	}
	if err := os.WriteFile(jsonPath, []byte(`{"output":{"format":"text"}}`), 0o644); err != nil {
		t.Fatalf("write json: %v", err)
	}

	cfg, path, err := Load(dir, "", nil)
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if path != yamlPath {
		t.Fatalf("expected yaml precedence, got %q", path)
	}
	if cfg.Output.Format != "json" {
		t.Fatalf("expected yaml config to win, got %q", cfg.Output.Format)
	}

	cfg, path, err = Load(dir, jsonPath, nil)
	if err != nil {
		t.Fatalf("load explicit config: %v", err)
	}
	if path != jsonPath {
		t.Fatalf("expected explicit path, got %q", path)
	}
	if cfg.Output.Format != "text" {
		t.Fatalf("expected explicit config to win, got %q", cfg.Output.Format)
	}
}

func TestLoadRejectsUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".go-doctor.yaml")
	if err := os.WriteFile(path, []byte("bogus: true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(dir, "", nil)
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
	if !strings.Contains(err.Error(), "field bogus not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadRejectsUnknownRules(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".go-doctor.yaml")
	if err := os.WriteFile(path, []byte("rules:\n  enable: [missing-rule]\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := Load(dir, "", nil)
	if err == nil {
		t.Fatal("expected error for unknown rule")
	}
	if !strings.Contains(err.Error(), `unknown rule "missing-rule"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyAnalyzerToggles(t *testing.T) {
	cfg := File{
		Analyzers: AnalyzersConfig{
			ThirdParty: boolPtr(false),
			Custom:     boolPtr(false),
		},
	}

	opts := DefaultOptions()
	if err := cfg.Apply(&opts); err != nil {
		t.Fatalf("apply config: %v", err)
	}
	if opts.ThirdParty {
		t.Fatal("expected third-party analyzers disabled")
	}
	if opts.Custom {
		t.Fatal("expected custom analyzers disabled")
	}
}

func boolPtr(value bool) *bool {
	return &value
}
