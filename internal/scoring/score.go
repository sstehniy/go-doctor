package scoring

import (
	"math"
	"sort"
	"strings"

	"github.com/sstehniy/go-doctor/internal/model"
)

type Result struct {
	Enabled bool
	Value   int
	Max     int
	Grade   string
}

const (
	maxScore = 100

	firstErrorPenalty   = 8.0
	firstWarningPenalty = 3.0
	extraErrorPenalty   = 1.0
	extraWarnPenalty    = 0.5

	maxExtraErrorPenalty = 5.0
	maxExtraWarnPenalty  = 3.0

	reachableGovulncheckCap = 49
	modReadonlyFailureCap   = 74
)

type ruleBucket struct {
	plugin   string
	rule     string
	severity string
	category string
	count    int
}

func Score(diagnostics []model.Diagnostic, enabled bool) Result {
	if !enabled {
		return Result{Enabled: false}
	}

	buckets := groupByRuleAndSeverity(diagnostics)
	totalPenalty := 0.0
	keys := sortedKeys(buckets)
	for _, key := range keys {
		bucket := buckets[key]
		totalPenalty += rulePenalty(bucket) * categoryMultiplier(bucket.plugin, bucket.category)
	}

	score := int(math.Round(maxScore - totalPenalty))
	if score < 0 {
		score = 0
	}
	if score > maxScore {
		score = maxScore
	}
	if hasReachableGovulncheck(diagnostics) && score > reachableGovulncheckCap {
		score = reachableGovulncheckCap
	}
	if hasModReadonlyFailure(diagnostics) && score > modReadonlyFailureCap {
		score = modReadonlyFailureCap
	}

	grade := "Critical"
	switch {
	case score >= 90:
		grade = "Excellent"
	case score >= 75:
		grade = "Good"
	case score >= 50:
		grade = "Needs work"
	}

	return Result{
		Enabled: true,
		Value:   score,
		Max:     maxScore,
		Grade:   grade,
	}
}

func groupByRuleAndSeverity(diagnostics []model.Diagnostic) map[string]ruleBucket {
	buckets := map[string]ruleBucket{}
	for _, diagnostic := range diagnostics {
		if diagnostic.Suppressed {
			continue
		}
		severity := strings.ToLower(strings.TrimSpace(diagnostic.Severity))
		key := diagnostic.Plugin + "|" + diagnostic.Rule + "|" + severity
		bucket := buckets[key]
		if bucket.count == 0 {
			bucket.plugin = strings.ToLower(strings.TrimSpace(diagnostic.Plugin))
			bucket.rule = strings.TrimSpace(diagnostic.Rule)
			bucket.severity = severity
			bucket.category = strings.ToLower(strings.TrimSpace(diagnostic.Category))
		}
		bucket.count++
		buckets[key] = bucket
	}
	return buckets
}

func sortedKeys(values map[string]ruleBucket) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func rulePenalty(bucket ruleBucket) float64 {
	if bucket.count == 0 {
		return 0
	}

	switch bucket.severity {
	case "error":
		extraCount := max(0, bucket.count-1)
		extra := min(maxExtraErrorPenalty, float64(extraCount)*extraErrorPenalty)
		return firstErrorPenalty + extra
	case "warning":
		extraCount := max(0, bucket.count-1)
		extra := min(maxExtraWarnPenalty, float64(extraCount)*extraWarnPenalty)
		return firstWarningPenalty + extra
	default:
		return 0
	}
}

func categoryMultiplier(plugin string, category string) float64 {
	if isStyleLikeThirdParty(plugin, category) {
		return 0.5
	}

	switch category {
	case "security":
		return 1.5
	case "correctness", "concurrency", "resource", "reliability":
		return 1.25
	case "net/http", "context", "architecture", "api-surface", "library-safety", "mod", "build", "license":
		return 1.0
	case "performance", "testing":
		return 0.75
	default:
		return 1.0
	}
}

func isStyleLikeThirdParty(plugin string, category string) bool {
	switch plugin {
	case "govet", "staticcheck", "golangci-lint":
	default:
		return false
	}
	switch category {
	case "maintainability", "simplification", "quickfix":
		return true
	default:
		return false
	}
}

func hasReachableGovulncheck(diagnostics []model.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Suppressed {
			continue
		}
		if strings.EqualFold(diagnostic.Plugin, "govulncheck") && strings.EqualFold(diagnostic.Severity, "error") {
			return true
		}
	}
	return false
}

func hasModReadonlyFailure(diagnostics []model.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Suppressed {
			continue
		}
		if strings.EqualFold(diagnostic.Plugin, "repo") && diagnostic.Rule == "build/mod-readonly-failure" {
			return true
		}
	}
	return false
}
