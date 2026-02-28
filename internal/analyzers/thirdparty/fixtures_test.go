package thirdparty_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stanislavstehniy/go-doctor/internal/analyzers/thirdparty"
	"github.com/stanislavstehniy/go-doctor/internal/model"
)

func TestNormalizationFixtures(t *testing.T) {
	cases := []struct {
		name      string
		output    string
		golden    string
		parseFunc func(string, string) []model.Diagnostic
	}{
		{
			name:      "govet",
			output:    "testdata/govet/output.txt",
			golden:    "testdata/govet/golden.json",
			parseFunc: thirdparty.ExportParseGoVetOutput,
		},
		{
			name:      "staticcheck-json",
			output:    "testdata/staticcheck/output.jsonl",
			golden:    "testdata/staticcheck/golden.json",
			parseFunc: thirdparty.ExportParseStaticcheckJSON,
		},
		{
			name:      "staticcheck-text",
			output:    "testdata/staticcheck/output.txt",
			golden:    "testdata/staticcheck/golden-text.json",
			parseFunc: thirdparty.ExportParseStaticcheckText,
		},
		{
			name:      "govulncheck",
			output:    "testdata/govulncheck/output.jsonl",
			golden:    "testdata/govulncheck/golden.json",
			parseFunc: thirdparty.ExportParseGovulncheckJSON,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := readFixtureFile(t, tc.output)
			want := readGoldenDiagnostics(t, tc.golden)
			got := tc.parseFunc(output, "/repo")
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("unexpected normalized diagnostics\nwant: %#v\ngot: %#v", want, got)
			}
		})
	}
}

func TestGolangCINormalizationFixture(t *testing.T) {
	output := readFixtureFile(t, "testdata/golangci/output.json")
	want := readGoldenDiagnostics(t, "testdata/golangci/golden.json")
	got, err := thirdparty.ExportParseGolangCIJSON(output, "/repo")
	if err != nil {
		t.Fatalf("parse golangci fixture: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected normalized diagnostics\nwant: %#v\ngot: %#v", want, got)
	}
}

func readFixtureFile(t *testing.T, relativePath string) string {
	t.Helper()

	path := filepath.Join(relativePath)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", relativePath, err)
	}
	return string(raw)
}

func readGoldenDiagnostics(t *testing.T, relativePath string) []model.Diagnostic {
	t.Helper()

	raw := readFixtureFile(t, relativePath)
	var diagnosticsOut []model.Diagnostic
	if err := json.Unmarshal([]byte(raw), &diagnosticsOut); err != nil {
		t.Fatalf("unmarshal golden %s: %v", relativePath, err)
	}
	return diagnosticsOut
}
