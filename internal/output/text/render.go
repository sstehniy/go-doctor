package text

import (
	"fmt"
	"sort"
	"strings"

	"github.com/stanislavstehniy/go-doctor/pkg/godoctor"
)

func Render(result godoctor.DiagnoseResult, verbose bool, quiet bool) []byte {
	if quiet && len(result.Diagnostics) == 0 && len(result.ToolErrors) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("go-doctor\n")
	b.WriteString("=========\n\n")

	fmt.Fprintf(&b, "Project\n")
	fmt.Fprintf(&b, "root: %s\n", result.Project.Root)
	fmt.Fprintf(&b, "mode: %s\n", result.Project.Mode)
	fmt.Fprintf(&b, "go: %s\n", valueOrNA(result.Project.GoVersion))
	fmt.Fprintf(&b, "modules: %d\n", len(result.Project.ModuleRoots))
	fmt.Fprintf(&b, "packages: %d\n\n", result.Project.PackageCount)

	fmt.Fprintf(&b, "Score\n")
	if result.Score != nil && result.Score.Enabled {
		fmt.Fprintf(&b, "%d/%d (%s)\n\n", result.Score.Value, result.Score.Max, valueOrNA(result.Score.Grade))
	} else {
		fmt.Fprintf(&b, "disabled\n\n")
	}

	fmt.Fprintf(&b, "Categories\n")
	categoryCounts := map[string]int{}
	for _, diagnostic := range result.Diagnostics {
		categoryCounts[diagnostic.Category]++
	}
	if len(categoryCounts) == 0 {
		b.WriteString("none\n\n")
	} else {
		categories := make([]string, 0, len(categoryCounts))
		for category := range categoryCounts {
			categories = append(categories, category)
		}
		sort.Strings(categories)
		for _, category := range categories {
			fmt.Fprintf(&b, "%s: %d\n", category, categoryCounts[category])
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Findings\n")
	if len(result.Diagnostics) == 0 {
		b.WriteString("healthy: no findings\n\n")
	} else {
		for _, diagnostic := range result.Diagnostics {
			suffix := ""
			if diagnostic.Suppressed {
				suffix = " (suppressed)"
			}
			fmt.Fprintf(&b, "%s [%s/%s] %s%s\n", diagnostic.Severity, diagnostic.Plugin, diagnostic.Rule, diagnostic.Message, suffix)
			if diagnostic.Path != "" {
				fmt.Fprintf(&b, "path: %s:%d:%d\n", diagnostic.Path, diagnostic.Line, diagnostic.Column)
			}
		}
		b.WriteString("\n")
	}

	if verbose {
		fmt.Fprintf(&b, "Verbose\n")
		if len(result.Diagnostics) == 0 {
			b.WriteString("no diagnostic file lines\n\n")
		} else {
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Path == "" {
					continue
				}
				fmt.Fprintf(&b, "%s:%d:%d\n", diagnostic.Path, diagnostic.Line, diagnostic.Column)
			}
			b.WriteString("\n")
		}
	}

	fmt.Fprintf(&b, "Warnings\n")
	if len(result.SkippedTools) == 0 && len(result.ToolErrors) == 0 {
		b.WriteString("none\n\n")
	} else {
		for _, tool := range result.SkippedTools {
			fmt.Fprintf(&b, "skipped: %s\n", tool)
		}
		for _, toolErr := range result.ToolErrors {
			fmt.Fprintf(&b, "tool %s: %s\n", toolErr.Tool, toolErr.Message)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "Remediation\n")
	b.WriteString("address reported findings, then rerun go-doctor\n")

	return []byte(b.String())
}

func valueOrNA(value string) string {
	if value == "" {
		return "n/a"
	}
	return value
}
