package baseline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sstehniy/go-doctor/internal/model"
)

func TestFingerprintNormalizesWhitespaceAndRange(t *testing.T) {
	left := model.Diagnostic{
		Rule:    "mod/not-tidy",
		Path:    "pkg\\main.go",
		Line:    10,
		Column:  2,
		EndLine: 10,
		Message: "message   with\nextra\tspace",
	}
	right := model.Diagnostic{
		Rule:      "mod/not-tidy",
		Path:      "pkg/main.go",
		Line:      10,
		Column:    2,
		EndLine:   10,
		EndColumn: 2,
		Message:   "message with extra space",
	}

	if got, want := Fingerprint(left), Fingerprint(right); got != want {
		t.Fatalf("expected stable fingerprint, got %q want %q", got, want)
	}
}

func TestApplyMarksMatchingDiagnosticsSuppressed(t *testing.T) {
	diagnostics := []model.Diagnostic{
		{Rule: "license/missing", Path: "go.mod", Line: 1, Column: 1, Message: "missing license"},
		{Rule: "fmt/not-gofmt", Path: "main.go", Line: 2, Column: 1, Message: "file is not gofmt formatted"},
	}

	file := Build(diagnostics[:1])
	set := Set{fingerprints: map[string]struct{}{file.Entries[0].Fingerprint: {}}}
	applied := Apply(diagnostics, set)

	if !applied[0].Suppressed {
		t.Fatal("expected first diagnostic to be suppressed")
	}
	if applied[1].Suppressed {
		t.Fatal("expected second diagnostic to stay active")
	}
}

func TestWriteProducesDeterministicJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "baseline.json")
	diagnostics := []model.Diagnostic{
		{Rule: "fmt/not-gofmt", Path: "main.go", Line: 2, Column: 1, Message: "file is not gofmt formatted"},
		{Rule: "license/missing", Path: "go.mod", Line: 1, Column: 1, Message: "missing license"},
	}

	if err := Write(path, diagnostics); err != nil {
		t.Fatalf("write baseline: %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read baseline: %v", err)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Fatal("expected trailing newline")
	}

	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatalf("unmarshal baseline: %v", err)
	}
	if file.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema version %d, got %d", SchemaVersion, file.SchemaVersion)
	}
	if len(file.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(file.Entries))
	}
	if file.Entries[0].Fingerprint > file.Entries[1].Fingerprint {
		t.Fatalf("expected entries sorted by fingerprint: %#v", file.Entries)
	}
}

func TestBuildSkipsSuppressedDiagnostics(t *testing.T) {
	file := Build([]model.Diagnostic{
		{
			Rule:       "rule/one",
			Path:       "main.go",
			Line:       3,
			Column:     1,
			EndLine:    3,
			EndColumn:  10,
			Message:    "keep me out",
			Suppressed: true,
		},
		{
			Rule:      "rule/two",
			Path:      "main.go",
			Line:      5,
			Column:    1,
			EndLine:   5,
			EndColumn: 10,
			Message:   "still active",
		},
	})

	if len(file.Entries) != 1 {
		t.Fatalf("expected only unsuppressed entries, got %#v", file.Entries)
	}
	if file.Entries[0].Rule != "rule/two" {
		t.Fatalf("unexpected baseline entry: %#v", file.Entries[0])
	}
}
