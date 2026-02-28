package godoctor

import (
	"testing"

	"github.com/stanislavstehniy/go-doctor/internal/analyzers/custom"
)

func TestReleaseGateCustomRuleCoverage(t *testing.T) {
	descriptors := custom.RuleDescriptors()
	if len(descriptors) < 18 {
		t.Fatalf("expected at least 18 custom rules, got %d", len(descriptors))
	}

	categories := map[string]struct{}{}
	for _, descriptor := range descriptors {
		if descriptor.Category == "" {
			continue
		}
		categories[descriptor.Category] = struct{}{}
	}
	if len(categories) < 8 {
		t.Fatalf("expected at least 8 custom rule categories, got %d", len(categories))
	}
}
