package model

import (
	"path/filepath"
	"strings"
)

type Diagnostic struct {
	Path      string `json:"path,omitempty"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
	EndColumn int    `json:"endColumn,omitempty"`

	Plugin   string `json:"plugin,omitempty"`
	Rule     string `json:"rule,omitempty"`
	Severity string `json:"severity,omitempty"`
	Category string `json:"category,omitempty"`

	Message string `json:"message"`
	Help    string `json:"help,omitempty"`

	Symbol     string `json:"symbol,omitempty"`
	Package    string `json:"package,omitempty"`
	Module     string `json:"module,omitempty"`
	DocsURL    string `json:"docsUrl,omitempty"`
	Weight     int    `json:"weight,omitempty"`
	Suppressed bool   `json:"suppressed,omitempty"`
}

type ToolError struct {
	Tool    string `json:"tool"`
	Message string `json:"message"`
	Fatal   bool   `json:"fatal,omitempty"`
}

func NormalizePath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	return filepath.ToSlash(filepath.Clean(path))
}
