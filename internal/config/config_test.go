package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sstehniy/go-doctor/pkg/godoctor"
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
	if !errors.Is(err, ErrUnknownConfigField) {
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
	if !errors.Is(err, ErrUnknownRuleReference) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyAnalyzerToggles(t *testing.T) {
	cfg := File{
		Analyzers: AnalyzersConfig{
			Repo:       boolPtr(false),
			ThirdParty: boolPtr(false),
			Custom:     boolPtr(false),
		},
	}

	opts := DefaultOptions()
	if err := cfg.Apply(&opts); err != nil {
		t.Fatalf("apply config: %v", err)
	}
	if opts.RepoHygiene {
		t.Fatal("expected repo hygiene analyzers disabled")
	}
	if opts.ThirdParty {
		t.Fatal("expected third-party analyzers disabled")
	}
	if opts.Custom {
		t.Fatal("expected custom analyzers disabled")
	}
}

func TestValidateRuleSelectionsAllowsCustomGroups(t *testing.T) {
	selectors := godoctor.ListRuleSelectors()
	if err := ValidateRuleSelections([]string{"context"}, []string{"context/not-propagated"}, selectors); err != nil {
		t.Fatalf("expected custom group selection to validate, got %v", err)
	}
}

func TestValidateDiffGovulncheck(t *testing.T) {
	valid := []string{"", "skip", "changed-modules-only", "  changed-modules-only  "}
	for _, value := range valid {
		if err := ValidateDiffGovulncheck(value); err != nil {
			t.Fatalf("expected %q to be valid, got %v", value, err)
		}
	}
	if err := ValidateDiffGovulncheck("full"); err == nil {
		t.Fatal("expected invalid mode to fail")
	}
}

func TestApplyDiffGovulncheckMode(t *testing.T) {
	cfg := File{
		Scan: ScanConfig{
			DiffGovulncheck: "changed-modules-only",
		},
	}
	opts := DefaultOptions()
	if err := cfg.Apply(&opts); err != nil {
		t.Fatalf("apply config: %v", err)
	}
	if opts.DiffGovulncheck != "changed-modules-only" {
		t.Fatalf("expected changed-modules-only mode, got %q", opts.DiffGovulncheck)
	}
}

func boolPtr(value bool) *bool {
	return &value
}
