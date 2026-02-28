package baseline

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sstehniy/go-doctor/internal/model"
)

const SchemaVersion = 1

type File struct {
	SchemaVersion int     `json:"schemaVersion"`
	Entries       []Entry `json:"entries"`
}

type Entry struct {
	Fingerprint string `json:"fingerprint"`
	Rule        string `json:"rule"`
	Path        string `json:"path,omitempty"`
	Line        int    `json:"line,omitempty"`
	Column      int    `json:"column,omitempty"`
	EndLine     int    `json:"endLine,omitempty"`
	EndColumn   int    `json:"endColumn,omitempty"`
	Message     string `json:"message"`
}

type Set struct {
	fingerprints map[string]struct{}
}

func Exists(path string) (bool, error) {
	if path == "" {
		return false, nil
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("stat baseline %q: %w", path, err)
}

func Load(path string) (File, Set, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return File{}, Set{}, fmt.Errorf("read baseline %q: %w", path, err)
	}

	var file File
	if err := json.Unmarshal(raw, &file); err != nil {
		return File{}, Set{}, fmt.Errorf("parse baseline %q: %w", path, err)
	}
	if file.SchemaVersion != SchemaVersion {
		return File{}, Set{}, fmt.Errorf("unsupported baseline schema version %d", file.SchemaVersion)
	}

	set := Set{fingerprints: make(map[string]struct{}, len(file.Entries))}
	for _, entry := range file.Entries {
		if entry.Fingerprint == "" {
			return File{}, Set{}, fmt.Errorf("baseline %q contains an entry without a fingerprint", path)
		}
		set.fingerprints[entry.Fingerprint] = struct{}{}
	}

	return file, set, nil
}

func Write(path string, diagnostics []model.Diagnostic) error {
	file := Build(diagnostics)
	body, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal baseline %q: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create baseline directory for %q: %w", path, err)
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return fmt.Errorf("write baseline %q: %w", path, err)
	}
	return nil
}

func Build(diagnostics []model.Diagnostic) File {
	entries := make([]Entry, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if diagnostic.Suppressed {
			continue
		}
		line, column, endLine, endColumn := normalizedRange(diagnostic)
		entries = append(entries, Entry{
			Fingerprint: Fingerprint(diagnostic),
			Rule:        diagnostic.Rule,
			Path:        normalizePath(diagnostic.Path),
			Line:        line,
			Column:      column,
			EndLine:     endLine,
			EndColumn:   endColumn,
			Message:     normalizeMessage(diagnostic.Message),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		left := entries[i]
		right := entries[j]
		if left.Fingerprint != right.Fingerprint {
			return left.Fingerprint < right.Fingerprint
		}
		if left.Rule != right.Rule {
			return left.Rule < right.Rule
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		if left.Column != right.Column {
			return left.Column < right.Column
		}
		if left.EndLine != right.EndLine {
			return left.EndLine < right.EndLine
		}
		if left.EndColumn != right.EndColumn {
			return left.EndColumn < right.EndColumn
		}
		return left.Message < right.Message
	})

	return File{
		SchemaVersion: SchemaVersion,
		Entries:       entries,
	}
}

func Apply(diagnostics []model.Diagnostic, set Set) []model.Diagnostic {
	if len(diagnostics) == 0 || len(set.fingerprints) == 0 {
		return diagnostics
	}

	out := make([]model.Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if set.Has(Fingerprint(diagnostic)) {
			diagnostic.Suppressed = true
		}
		out = append(out, diagnostic)
	}
	return out
}

func (s Set) Has(fingerprint string) bool {
	if len(s.fingerprints) == 0 {
		return false
	}
	_, ok := s.fingerprints[fingerprint]
	return ok
}

func Fingerprint(diagnostic model.Diagnostic) string {
	line, column, endLine, endColumn := normalizedRange(diagnostic)
	payload := strings.Join([]string{
		diagnostic.Rule,
		normalizePath(diagnostic.Path),
		fmt.Sprintf("%d:%d-%d:%d", line, column, endLine, endColumn),
		normalizeMessage(diagnostic.Message),
	}, "\n")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func normalizeMessage(message string) string {
	return strings.Join(strings.Fields(message), " ")
}

func normalizedRange(diagnostic model.Diagnostic) (int, int, int, int) {
	line := diagnostic.Line
	column := diagnostic.Column
	endLine := diagnostic.EndLine
	endColumn := diagnostic.EndColumn
	if endLine == 0 {
		endLine = line
	}
	if endColumn == 0 {
		endColumn = column
	}
	return line, column, endLine, endColumn
}

func normalizePath(path string) string {
	return model.NormalizePath(strings.ReplaceAll(path, "\\", "/"))
}
