package sarif

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/sstehniy/go-doctor/internal/model"
)

const (
	sarifSchemaURL = "https://json.schemastore.org/sarif-2.1.0.json"
	sarifVersion   = "2.1.0"
)

type RuleMetadata struct {
	Plugin      string
	Rule        string
	Name        string
	Description string
	Help        string
	DocsURL     string
	Severity    string
	Category    string
}

type Input struct {
	ProjectRoot  string
	Diagnostics  []model.Diagnostic
	ToolErrors   []model.ToolError
	RuleMetadata []RuleMetadata
	ScoreEnabled bool
	ScoreValue   int
	ScoreMax     int
	ScoreGrade   string
}

func Render(input Input) ([]byte, error) {
	rules := buildRules(input.RuleMetadata, input.Diagnostics)
	results := make([]sarifResult, 0, len(input.Diagnostics))
	for _, diagnostic := range input.Diagnostics {
		results = append(results, buildResult(input.ProjectRoot, diagnostic))
	}

	run := sarifRun{
		Tool: sarifTool{
			Driver: sarifDriver{
				Name:           "go-doctor",
				InformationURI: "https://github.com/sstehniy/go-doctor",
				Rules:          rules,
			},
		},
		Results:     results,
		Invocations: []sarifInvocation{buildInvocation(input.ToolErrors)},
	}
	if props := scoreProperties(input); len(props) > 0 {
		run.Properties = props
	}

	log := sarifLog{
		Schema:  sarifSchemaURL,
		Version: sarifVersion,
		Runs:    []sarifRun{run},
	}
	body, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal sarif output: %w", err)
	}
	return append(body, '\n'), nil
}

func scoreProperties(input Input) map[string]any {
	if !input.ScoreEnabled {
		return nil
	}
	return map[string]any{
		"goDoctorScore":    input.ScoreValue,
		"goDoctorScoreMax": input.ScoreMax,
		"goDoctorGrade":    input.ScoreGrade,
	}
}

func buildRules(metadata []RuleMetadata, diagnosticsOut []model.Diagnostic) []sarifRule {
	entries := map[string]*ruleEntry{}

	for _, item := range metadata {
		id := ruleID(item.Plugin, item.Rule)
		if id == "" {
			continue
		}
		entry := ensureRule(entries, id)
		entry.Plugin = pickFirst(entry.Plugin, item.Plugin)
		entry.Rule = pickFirst(entry.Rule, item.Rule)
		entry.Name = pickFirst(entry.Name, item.Name, item.Rule)
		entry.Description = pickFirst(entry.Description, item.Description)
		entry.Help = pickFirst(entry.Help, item.Help)
		entry.DocsURL = pickFirst(entry.DocsURL, item.DocsURL)
		entry.Severity = maxSeverity(entry.Severity, item.Severity)
		entry.Category = pickFirst(entry.Category, item.Category)
	}

	for _, diagnostic := range diagnosticsOut {
		id := ruleID(diagnostic.Plugin, diagnostic.Rule)
		if id == "" {
			continue
		}
		entry := ensureRule(entries, id)
		entry.Plugin = pickFirst(entry.Plugin, diagnostic.Plugin)
		entry.Rule = pickFirst(entry.Rule, diagnostic.Rule)
		entry.Name = pickFirst(entry.Name, diagnostic.Rule, id)
		entry.Description = pickFirst(entry.Description, diagnostic.Message)
		entry.Help = pickFirst(entry.Help, diagnostic.Help)
		entry.DocsURL = pickFirst(entry.DocsURL, diagnostic.DocsURL)
		entry.Severity = maxSeverity(entry.Severity, diagnostic.Severity)
		entry.Category = pickFirst(entry.Category, diagnostic.Category)
	}

	out := make([]sarifRule, 0, len(entries))
	for id, entry := range entries {
		rule := sarifRule{
			ID:   id,
			Name: pickFirst(entry.Name, entry.Rule, id),
		}
		if short := strings.TrimSpace(entry.Description); short != "" {
			rule.ShortDescription = &sarifText{Text: short}
		}
		if help := strings.TrimSpace(entry.Help); help != "" {
			rule.Help = &sarifText{Text: help}
		}
		if docs := strings.TrimSpace(entry.DocsURL); docs != "" {
			rule.HelpURI = docs
		}
		if level := levelForSeverity(entry.Severity); level != "" {
			rule.DefaultConfiguration = &sarifReportingConfiguration{Level: level}
		}
		tags := make([]string, 0, 2)
		if plugin := strings.TrimSpace(entry.Plugin); plugin != "" {
			tags = append(tags, "plugin:"+plugin)
		}
		if category := strings.TrimSpace(entry.Category); category != "" {
			tags = append(tags, "category:"+category)
		}
		if len(tags) > 0 {
			rule.Properties = &sarifRuleProperties{Tags: tags}
		}
		out = append(out, rule)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

func ensureRule(entries map[string]*ruleEntry, id string) *ruleEntry {
	entry, ok := entries[id]
	if ok {
		return entry
	}
	entry = &ruleEntry{}
	entries[id] = entry
	return entry
}

func buildResult(projectRoot string, diagnostic model.Diagnostic) sarifResult {
	id := ruleID(diagnostic.Plugin, diagnostic.Rule)
	artifact := normalizeArtifactPath(projectRoot, diagnostic.Path)
	line := max(diagnostic.Line, 1)
	column := max(diagnostic.Column, 1)
	result := sarifResult{
		RuleID:  id,
		Level:   levelForSeverity(diagnostic.Severity),
		Message: sarifText{Text: pickFirst(diagnostic.Message, "diagnostic reported")},
		PartialFingerprints: map[string]string{
			"primaryLocationLineHash": resultFingerprint(id, artifact, line, diagnostic.Message),
		},
		Properties: &sarifResultProperties{
			Plugin:     diagnostic.Plugin,
			Rule:       diagnostic.Rule,
			Category:   diagnostic.Category,
			Module:     diagnostic.Module,
			Package:    diagnostic.Package,
			Symbol:     diagnostic.Symbol,
			Suppressed: diagnostic.Suppressed,
		},
	}
	if artifact != "" {
		location := sarifLocation{
			PhysicalLocation: sarifPhysicalLocation{
				ArtifactLocation: sarifArtifactLocation{URI: artifact},
				Region: sarifRegion{
					StartLine:   line,
					StartColumn: column,
				},
			},
		}
		if diagnostic.EndLine >= line {
			location.PhysicalLocation.Region.EndLine = diagnostic.EndLine
		}
		if diagnostic.EndColumn > 0 {
			location.PhysicalLocation.Region.EndColumn = diagnostic.EndColumn
		}
		result.Locations = []sarifLocation{location}
	}
	if diagnostic.Suppressed {
		result.Suppressions = []sarifSuppression{{
			Kind:          "external",
			Status:        "accepted",
			Justification: "suppressed by go-doctor baseline or inline directive",
		}}
	}
	return result
}

func buildInvocation(toolErrors []model.ToolError) sarifInvocation {
	invocation := sarifInvocation{
		ExecutionSuccessful: true,
	}
	if len(toolErrors) == 0 {
		return invocation
	}
	notifications := make([]sarifNotification, 0, len(toolErrors))
	for _, toolErr := range toolErrors {
		level := "warning"
		if toolErr.Fatal {
			level = "error"
			invocation.ExecutionSuccessful = false
		}
		text := strings.TrimSpace(toolErr.Message)
		if strings.TrimSpace(toolErr.Tool) != "" {
			text = fmt.Sprintf("%s: %s", toolErr.Tool, text)
		}
		notifications = append(notifications, sarifNotification{
			Level:   level,
			Message: sarifText{Text: text},
		})
	}
	invocation.ToolExecutionNotifications = notifications
	return invocation
}

func normalizeArtifactPath(projectRoot string, path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	if filepath.IsAbs(cleaned) {
		root := strings.TrimSpace(projectRoot)
		if root == "" {
			return ""
		}
		relative, err := filepath.Rel(filepath.Clean(root), cleaned)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
			return ""
		}
		cleaned = relative
	}
	normalized := model.NormalizePath(cleaned)
	normalized = strings.TrimPrefix(normalized, "./")
	if normalized == "." || normalized == ".." || strings.HasPrefix(normalized, "../") {
		return ""
	}
	return normalized
}

func resultFingerprint(ruleID string, path string, line int, message string) string {
	hash := fnv.New64a()
	_, _ = io.WriteString(hash, ruleID)
	_, _ = hash.Write([]byte{0})
	_, _ = io.WriteString(hash, path)
	_, _ = hash.Write([]byte{0})
	_, _ = io.WriteString(hash, strconv.Itoa(line))
	_, _ = hash.Write([]byte{0})
	_, _ = io.WriteString(hash, message)
	return strconv.FormatUint(hash.Sum64(), 16)
}

func levelForSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "error":
		return "error"
	case "warning":
		return "warning"
	case "info":
		return "note"
	default:
		return "warning"
	}
}

func maxSeverity(current string, candidate string) string {
	rank := func(value string) int {
		switch strings.ToLower(strings.TrimSpace(value)) {
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
	if rank(candidate) > rank(current) {
		return candidate
	}
	return current
}

func ruleID(plugin string, rule string) string {
	plugin = strings.TrimSpace(plugin)
	rule = strings.TrimSpace(rule)
	switch {
	case plugin == "" && rule == "":
		return ""
	case plugin == "":
		return rule
	case rule == "":
		return plugin
	default:
		return plugin + "/" + rule
	}
}

func pickFirst(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

type ruleEntry struct {
	Plugin      string
	Rule        string
	Name        string
	Description string
	Help        string
	DocsURL     string
	Severity    string
	Category    string
}

type sarifLog struct {
	Schema  string     `json:"$schema,omitempty"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool        sarifTool         `json:"tool"`
	Results     []sarifResult     `json:"results,omitempty"`
	Invocations []sarifInvocation `json:"invocations,omitempty"`
	Properties  map[string]any    `json:"properties,omitempty"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []sarifRule `json:"rules,omitempty"`
}

type sarifRule struct {
	ID                   string                       `json:"id"`
	Name                 string                       `json:"name,omitempty"`
	ShortDescription     *sarifText                   `json:"shortDescription,omitempty"`
	Help                 *sarifText                   `json:"help,omitempty"`
	HelpURI              string                       `json:"helpUri,omitempty"`
	DefaultConfiguration *sarifReportingConfiguration `json:"defaultConfiguration,omitempty"`
	Properties           *sarifRuleProperties         `json:"properties,omitempty"`
}

type sarifRuleProperties struct {
	Tags []string `json:"tags,omitempty"`
}

type sarifReportingConfiguration struct {
	Level string `json:"level,omitempty"`
}

type sarifResult struct {
	RuleID              string                 `json:"ruleId,omitempty"`
	Level               string                 `json:"level,omitempty"`
	Message             sarifText              `json:"message"`
	Locations           []sarifLocation        `json:"locations,omitempty"`
	PartialFingerprints map[string]string      `json:"partialFingerprints,omitempty"`
	Properties          *sarifResultProperties `json:"properties,omitempty"`
	Suppressions        []sarifSuppression     `json:"suppressions,omitempty"`
}

type sarifResultProperties struct {
	Plugin     string `json:"plugin,omitempty"`
	Rule       string `json:"rule,omitempty"`
	Category   string `json:"category,omitempty"`
	Module     string `json:"module,omitempty"`
	Package    string `json:"package,omitempty"`
	Symbol     string `json:"symbol,omitempty"`
	Suppressed bool   `json:"suppressed,omitempty"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
	Region           sarifRegion           `json:"region,omitempty"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri,omitempty"`
}

type sarifRegion struct {
	StartLine   int `json:"startLine,omitempty"`
	StartColumn int `json:"startColumn,omitempty"`
	EndLine     int `json:"endLine,omitempty"`
	EndColumn   int `json:"endColumn,omitempty"`
}

type sarifSuppression struct {
	Kind          string `json:"kind,omitempty"`
	Status        string `json:"status,omitempty"`
	Justification string `json:"justification,omitempty"`
}

type sarifInvocation struct {
	ExecutionSuccessful        bool                `json:"executionSuccessful"`
	ToolExecutionNotifications []sarifNotification `json:"toolExecutionNotifications,omitempty"`
}

type sarifNotification struct {
	Level   string    `json:"level,omitempty"`
	Message sarifText `json:"message"`
}

type sarifText struct {
	Text string `json:"text,omitempty"`
}
