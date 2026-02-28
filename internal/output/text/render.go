package text

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/stanislavstehniy/go-doctor/pkg/godoctor"
)

const defaultMaxCompactFiles = 3

type Options struct {
	Verbose         bool
	Quiet           bool
	UseColor        bool
	UseUnicode      bool
	MaxCompactFiles int
}

type diagnosticGroup struct {
	Plugin      string
	Rule        string
	Message     string
	Help        string
	Severity    string
	Diagnostics []godoctor.Diagnostic
}

func Render(result godoctor.DiagnoseResult, opts Options) []byte {
	if opts.Quiet && len(result.Diagnostics) == 0 && len(result.ToolErrors) == 0 {
		return nil
	}
	if opts.MaxCompactFiles <= 0 {
		opts.MaxCompactFiles = defaultMaxCompactFiles
	}

	colors := palette{enabled: opts.UseColor}
	var b strings.Builder

	active := make([]godoctor.Diagnostic, 0, len(result.Diagnostics))
	suppressed := make([]godoctor.Diagnostic, 0)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Suppressed {
			suppressed = append(suppressed, diagnostic)
			continue
		}
		active = append(active, diagnostic)
	}

	groups := groupDiagnostics(active)
	suppressedGroups := groupDiagnostics(suppressed)
	categoryCounts := countCategories(active)
	severityCounts := countSeverities(active)
	affectedFiles := countAffectedFiles(active)

	b.WriteString("go-doctor\n")

	fmt.Fprintf(
		&b,
		"project: %s (mode: %s, go: %s, modules: %d, packages: %d)\n\n",
		result.Project.Root,
		valueOrNA(result.Project.Mode),
		valueOrNA(result.Project.GoVersion),
		len(result.Project.ModuleRoots),
		result.Project.PackageCount,
	)

	renderSummary(&b, result, severityCounts, affectedFiles, opts.UseUnicode, colors)
	b.WriteString("\n")

	b.WriteString("Categories\n")
	if len(categoryCounts) == 0 {
		b.WriteString("none\n\n")
	} else {
		for _, item := range sortedCategoryCounts(categoryCounts) {
			fmt.Fprintf(&b, "%s: %d\n", item.name, item.count)
		}
		b.WriteString("\n")
	}

	b.WriteString("Active Findings\n")
	if len(groups) == 0 {
		b.WriteString("healthy: no active findings\n\n")
	} else {
		for _, group := range groups {
			renderGroup(&b, group, opts, colors)
		}
		b.WriteString("\n")
	}

	if len(suppressedGroups) > 0 {
		b.WriteString("Suppressed Findings\n")
		for _, group := range suppressedGroups {
			renderGroup(&b, group, opts, colors)
		}
		b.WriteString("\n")
	}

	b.WriteString("Tool Status\n")
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

	b.WriteString("Next Steps\n")
	b.WriteString("fix highest-severity findings first, then rerun go-doctor\n")

	return []byte(b.String())
}

func renderSummary(
	b *strings.Builder,
	result godoctor.DiagnoseResult,
	severityCounts map[string]int,
	affectedFiles int,
	unicode bool,
	colors palette,
) {
	scoreLine := "Score: disabled"
	scoreBar := ""

	if result.Score != nil && result.Score.Enabled {
		scoreLine = fmt.Sprintf("%d / %d  %s", result.Score.Value, result.Score.Max, valueOrNA(result.Score.Grade))
		scoreBar = buildScoreBar(result.Score.Value, result.Score.Max, 30, unicode, colors)
	}

	errorCount := severityCounts["error"]
	warningCount := severityCounts["warning"]

	elapsed := formatElapsed(result.ElapsedMillis)

	errorLabel := "errors"
	if errorCount == 1 {
		errorLabel = "error"
	}
	warningLabel := "warnings"
	if warningCount == 1 {
		warningLabel = "warning"
	}

	errorSymbol, warningSymbol := "x", "!"
	if unicode {
		errorSymbol, warningSymbol = "×", "▲"
	}

	var footerParts []string
	if errorCount > 0 {
		footerParts = append(footerParts, colors.red(errorSymbol+" "+strconv.Itoa(errorCount)+" "+errorLabel))
	}
	if warningCount > 0 {
		footerParts = append(footerParts, colors.yellow(warningSymbol+" "+strconv.Itoa(warningCount)+" "+warningLabel))
	}
	if errorCount == 0 && warningCount == 0 {
		footerParts = append(footerParts, "no issues")
	}

	filesLabel := "files"
	if affectedFiles == 1 {
		filesLabel = "file"
	}
	filesText := fmt.Sprintf("across %d %s", affectedFiles, filesLabel)
	footerParts = append(footerParts, filesText)
	footerParts = append(footerParts, "in "+elapsed)

	footerLine := strings.Join(footerParts, "  ")

	lines := []string{
		"Golang Doctor",
		"",
		scoreLine,
		"",
		scoreBar,
		"",
		footerLine,
	}
	if scoreBar == "" {
		lines = []string{
			"go-doctor",
			"",
			scoreLine,
			"",
			footerLine,
		}
	}
	writeSummaryBox(b, lines, unicode, colors)
}

func renderGroup(b *strings.Builder, group diagnosticGroup, opts Options, colors palette) {
	count := len(group.Diagnostics)
	header := fmt.Sprintf("%s %s/%s (%d)", severityLabel(group.Severity, opts.UseUnicode), group.Plugin, group.Rule, count)
	fmt.Fprintf(b, "%s\n", colors.colorizeSeverity(header, group.Severity))
	fmt.Fprintf(b, "  %s\n", group.Message)
	if strings.TrimSpace(group.Help) != "" {
		fmt.Fprintf(b, "  %s\n", group.Help)
	}

	if opts.Verbose {
		locations := groupLocations(group.Diagnostics)
		for _, location := range locations {
			fmt.Fprintf(b, "  %s\n", location)
		}
		b.WriteString("\n")
		return
	}

	fileSummaries := compactFileSummaries(group.Diagnostics)
	limit := opts.MaxCompactFiles
	if limit > len(fileSummaries) {
		limit = len(fileSummaries)
	}
	for index := 0; index < limit; index++ {
		fmt.Fprintf(b, "  %s\n", fileSummaries[index])
	}
	if len(fileSummaries) > limit {
		fmt.Fprintf(b, "  +%d more files\n", len(fileSummaries)-limit)
	}
	b.WriteString("\n")
}

func compactFileSummaries(diagnostics []godoctor.Diagnostic) []string {
	type fileSummary struct {
		path  string
		lines []int
	}

	byPath := map[string]map[int]struct{}{}
	for _, diagnostic := range diagnostics {
		if strings.TrimSpace(diagnostic.Path) == "" {
			continue
		}
		lineSet, ok := byPath[diagnostic.Path]
		if !ok {
			lineSet = map[int]struct{}{}
			byPath[diagnostic.Path] = lineSet
		}
		if diagnostic.Line > 0 {
			lineSet[diagnostic.Line] = struct{}{}
		}
	}

	summaries := make([]fileSummary, 0, len(byPath))
	for path, lineSet := range byPath {
		lines := make([]int, 0, len(lineSet))
		for line := range lineSet {
			lines = append(lines, line)
		}
		sort.Ints(lines)
		summaries = append(summaries, fileSummary{path: path, lines: lines})
	}

	sort.SliceStable(summaries, func(i, j int) bool {
		if len(summaries[i].lines) != len(summaries[j].lines) {
			return len(summaries[i].lines) > len(summaries[j].lines)
		}
		return summaries[i].path < summaries[j].path
	})

	out := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		if len(summary.lines) == 0 {
			out = append(out, summary.path)
			continue
		}
		lineParts := make([]string, 0, len(summary.lines))
		for _, line := range summary.lines {
			lineParts = append(lineParts, strconv.Itoa(line))
		}
		out = append(out, fmt.Sprintf("%s: %s", summary.path, strings.Join(lineParts, ", ")))
	}
	return out
}

func groupLocations(diagnostics []godoctor.Diagnostic) []string {
	seen := map[string]struct{}{}
	locations := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if strings.TrimSpace(diagnostic.Path) == "" {
			continue
		}
		location := diagnostic.Path
		if diagnostic.Line > 0 {
			location += ":" + strconv.Itoa(diagnostic.Line)
		}
		if diagnostic.Column > 0 {
			location += ":" + strconv.Itoa(diagnostic.Column)
		}
		if _, ok := seen[location]; ok {
			continue
		}
		seen[location] = struct{}{}
		locations = append(locations, location)
	}
	sort.Strings(locations)
	return locations
}

func countAffectedFiles(diagnostics []godoctor.Diagnostic) int {
	files := map[string]struct{}{}
	for _, diagnostic := range diagnostics {
		if strings.TrimSpace(diagnostic.Path) == "" {
			continue
		}
		files[diagnostic.Path] = struct{}{}
	}
	return len(files)
}

func countSeverities(diagnostics []godoctor.Diagnostic) map[string]int {
	counts := map[string]int{}
	for _, diagnostic := range diagnostics {
		counts[strings.ToLower(strings.TrimSpace(diagnostic.Severity))]++
	}
	return counts
}

func severityLabel(severity string, unicode bool) string {
	severity = strings.ToLower(strings.TrimSpace(severity))
	if !unicode {
		switch severity {
		case "error":
			return "E"
		case "warning":
			return "W"
		case "info":
			return "I"
		default:
			return "-"
		}
	}
	switch severity {
	case "error":
		return "✗"
	case "warning":
		return "⚠"
	case "info":
		return "ℹ"
	default:
		return "•"
	}
}

type categoryCount struct {
	name  string
	count int
}

func sortedCategoryCounts(categoryCounts map[string]int) []categoryCount {
	out := make([]categoryCount, 0, len(categoryCounts))
	for name, count := range categoryCounts {
		out = append(out, categoryCount{name: name, count: count})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].count != out[j].count {
			return out[i].count > out[j].count
		}
		return out[i].name < out[j].name
	})
	return out
}

func countCategories(diagnostics []godoctor.Diagnostic) map[string]int {
	categoryCounts := map[string]int{}
	for _, diagnostic := range diagnostics {
		category := strings.TrimSpace(diagnostic.Category)
		if category == "" {
			category = "uncategorized"
		}
		categoryCounts[category]++
	}
	return categoryCounts
}

func groupDiagnostics(diagnostics []godoctor.Diagnostic) []diagnosticGroup {
	groups := map[string]*diagnosticGroup{}
	for _, diagnostic := range diagnostics {
		severity := strings.ToLower(strings.TrimSpace(diagnostic.Severity))
		key := strings.Join([]string{
			strings.TrimSpace(diagnostic.Plugin),
			strings.TrimSpace(diagnostic.Rule),
			severity,
			strings.TrimSpace(diagnostic.Message),
			strings.TrimSpace(diagnostic.Help),
		}, "\x1f")
		group, ok := groups[key]
		if !ok {
			group = &diagnosticGroup{
				Plugin:   strings.TrimSpace(diagnostic.Plugin),
				Rule:     strings.TrimSpace(diagnostic.Rule),
				Message:  strings.TrimSpace(diagnostic.Message),
				Help:     strings.TrimSpace(diagnostic.Help),
				Severity: severity,
			}
			groups[key] = group
		}
		group.Diagnostics = append(group.Diagnostics, diagnostic)
	}

	out := make([]diagnosticGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, *group)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := out[i]
		right := out[j]
		if severityRank(left.Severity) != severityRank(right.Severity) {
			return severityRank(left.Severity) > severityRank(right.Severity)
		}
		if len(left.Diagnostics) != len(right.Diagnostics) {
			return len(left.Diagnostics) > len(right.Diagnostics)
		}
		leftRule := left.Plugin + "/" + left.Rule
		rightRule := right.Plugin + "/" + right.Rule
		if leftRule != rightRule {
			return leftRule < rightRule
		}
		return left.Message < right.Message
	})
	return out
}

func severityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "error":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func formatElapsed(elapsedMillis int64) string {
	if elapsedMillis < 1000 {
		return fmt.Sprintf("%dms", elapsedMillis)
	}
	seconds := float64(elapsedMillis) / 1000
	return fmt.Sprintf("%.1fs", seconds)
}

func buildScoreBar(score int, max int, width int, unicode bool, colors palette) string {
	if max <= 0 || width <= 0 {
		return ""
	}
	filled := int(float64(score) / float64(max) * float64(width))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	empty := width - filled
	filledChar := "#"
	emptyChar := "-"
	if unicode {
		filledChar = "█"
		emptyChar = "░"
	}
	return colors.green(strings.Repeat(filledChar, filled)) + colors.dim(strings.Repeat(emptyChar, empty))
}

func valueOrNA(value string) string {
	if strings.TrimSpace(value) == "" {
		return "n/a"
	}
	return value
}

type palette struct {
	enabled bool
}

func (p palette) red(value string) string    { return p.color("31", value) }
func (p palette) green(value string) string  { return p.color("32", value) }
func (p palette) yellow(value string) string { return p.color("33", value) }
func (p palette) cyan(value string) string   { return p.color("36", value) }
func (p palette) dim(value string) string    { return p.color("2", value) }

func (p palette) colorizeSeverity(value string, severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "error":
		return p.red(value)
	case "warning":
		return p.yellow(value)
	case "info":
		return p.cyan(value)
	default:
		return value
	}
}

func (p palette) color(code string, value string) string {
	if !p.enabled || value == "" {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func visibleWidth(s string) int {
	stripped := stripAnsi(s)
	return utf8.RuneCountInString(stripped)
}

func stripAnsi(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && s[j] != 'm' {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}

func writeSummaryBox(b *strings.Builder, lines []string, unicode bool, colors palette) {
	maxWidth := 0
	for _, line := range lines {
		width := visibleWidth(line)
		if width > maxWidth {
			maxWidth = width
		}
	}

	leftTop, rightTop, leftBottom, rightBottom, horizontal, vertical := boxChars(unicode)
	border := strings.Repeat(horizontal, maxWidth+2)

	b.WriteString(colors.dim(leftTop + border + rightTop))
	b.WriteString("\n")

	for _, line := range lines {
		padding := strings.Repeat(" ", maxWidth-visibleWidth(line))
		b.WriteString(colors.dim(vertical))
		b.WriteString(" ")
		b.WriteString(line)
		b.WriteString(padding)
		b.WriteString(" ")
		b.WriteString(colors.dim(vertical))
		b.WriteString("\n")
	}

	b.WriteString(colors.dim(leftBottom + border + rightBottom))
	b.WriteString("\n")
}

func boxChars(unicode bool) (leftTop, rightTop, leftBottom, rightBottom, horizontal, vertical string) {
	if unicode {
		return "┌", "┐", "└", "┘", "─", "│"
	}
	return "+", "+", "+", "+", "-", "|"
}
