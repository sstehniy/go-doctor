package thirdparty

import (
	"strconv"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/diagnostics"
)

var supportedRules = []string{
	"govet",
	"staticcheck",
	"govulncheck",
	"golangci-lint",
	"errcheck",
	"ineffassign",
	"bodyclose",
	"rowserrcheck",
	"sqlclosecheck",
	"exportloopref",
	"prealloc",
}

func SupportedRules() []string {
	out := make([]string, len(supportedRules))
	copy(out, supportedRules)
	return out
}

func DefaultAnalyzers(target diagnostics.Target, enableRules []string, disableRules []string) []diagnostics.Analyzer {
	enableSet := toRuleSet(enableRules)
	disableSet := toRuleSet(disableRules)

	var analyzers []diagnostics.Analyzer
	if includeAnalyzer("govet", enableSet, disableSet) {
		analyzers = append(analyzers, newGoVetAdapter())
	}
	if includeAnalyzer("staticcheck", enableSet, disableSet) {
		analyzers = append(analyzers, newStaticcheckAdapter())
	}
	if includeAnalyzer("govulncheck", enableSet, disableSet) {
		analyzers = append(analyzers, newGovulncheckAdapter())
	}

	linters := defaultGolangCILinters(target.GoVersion)
	if len(enableSet) > 0 {
		var requested []string
		for _, linter := range supportedGolangCILinters(target.GoVersion) {
			if _, ok := enableSet[linter]; ok {
				requested = append(requested, linter)
			}
		}
		if len(requested) > 0 {
			linters = requested
		} else if _, ok := enableSet["golangci-lint"]; !ok {
			linters = nil
		}
	}
	filtered := linters[:0]
	for _, linter := range linters {
		if _, disabled := disableSet[linter]; disabled {
			continue
		}
		filtered = append(filtered, linter)
	}
	linters = filtered
	if _, disabled := disableSet["golangci-lint"]; disabled {
		linters = nil
	}
	if len(linters) > 0 {
		analyzers = append(analyzers, newGolangCIAdapter(linters))
	}

	return analyzers
}

func includeAnalyzer(name string, enableSet map[string]struct{}, disableSet map[string]struct{}) bool {
	if _, disabled := disableSet[name]; disabled {
		return false
	}
	if len(enableSet) == 0 {
		return true
	}
	_, enabled := enableSet[name]
	return enabled
}

func toRuleSet(values []string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		set[trimmed] = struct{}{}
	}
	return set
}

func defaultGolangCILinters(goVersion string) []string {
	linters := []string{"errcheck", "ineffassign", "bodyclose", "rowserrcheck", "sqlclosecheck"}
	if beforeGo122(goVersion) {
		linters = append(linters, "exportloopref")
	}
	return linters
}

func supportedGolangCILinters(goVersion string) []string {
	linters := []string{"errcheck", "ineffassign", "bodyclose", "rowserrcheck", "sqlclosecheck", "prealloc"}
	if beforeGo122(goVersion) {
		linters = append(linters, "exportloopref")
	}
	return linters
}

func beforeGo122(version string) bool {
	if version == "" {
		return true
	}
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return true
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return true
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return true
	}
	if major != 1 {
		return major < 1
	}
	return minor < 22
}
