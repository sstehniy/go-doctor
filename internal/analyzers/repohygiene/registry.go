package repohygiene

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/stanislavstehniy/go-doctor/internal/model"
)

type Descriptor struct {
	Plugin      string
	Rule        string
	Category    string
	DefaultOn   bool
	Severity    string
	Description string
	Help        string
}

type rule struct {
	desc Descriptor
	run  func(context.Context, *analysisContext) ([]model.Diagnostic, []model.ToolError)
}

func RuleDescriptors() []Descriptor {
	out := make([]Descriptor, 0, len(registry))
	for _, candidate := range registry {
		out = append(out, candidate.desc)
	}
	slices.SortFunc(out, func(left, right Descriptor) int {
		return strings.Compare(left.Rule, right.Rule)
	})
	return out
}

// SupportedRules returns the repo-hygiene rule names.
func SupportedRules() []string {
	out := make([]string, 0, len(registry))
	for _, candidate := range registry {
		out = append(out, candidate.desc.Rule)
	}
	slices.Sort(out)
	return out
}

// SupportedSelectors returns the repo-hygiene rule and group selectors.
func SupportedSelectors() []string {
	groups := selectorGroups()
	out := make([]string, 0, len(registry)+len(groups))
	out = append(out, SupportedRules()...)
	for name := range groups {
		out = append(out, name)
	}
	slices.Sort(out)
	return compactSorted(out)
}

func selectRules(enableRules []string, disableRules []string) ([]rule, error) {
	enabled, err := resolveSelectors(enableRules)
	if err != nil {
		return nil, err
	}
	disabled, err := resolveSelectors(disableRules)
	if err != nil {
		return nil, err
	}

	selected := map[string]rule{}
	if len(enableRules) == 0 {
		for _, candidate := range registry {
			if candidate.desc.DefaultOn {
				selected[candidate.desc.Rule] = candidate
			}
		}
	} else {
		for _, candidate := range enabled {
			selected[candidate.desc.Rule] = candidate
		}
	}

	for _, candidate := range disabled {
		delete(selected, candidate.desc.Rule)
	}

	out := make([]rule, 0, len(selected))
	for _, candidate := range selected {
		out = append(out, candidate)
	}
	slices.SortFunc(out, func(left, right rule) int {
		return strings.Compare(left.desc.Rule, right.desc.Rule)
	})
	return out, nil
}

func resolveSelectors(selectors []string) ([]rule, error) {
	if len(selectors) == 0 {
		return nil, nil
	}

	rulesByName := map[string]rule{}
	for _, candidate := range registry {
		rulesByName[candidate.desc.Rule] = candidate
	}

	groups := selectorGroups()
	resolved := map[string]rule{}
	for _, selector := range selectors {
		selector = strings.TrimSpace(selector)
		if selector == "" {
			continue
		}
		if candidate, ok := rulesByName[selector]; ok {
			resolved[candidate.desc.Rule] = candidate
			continue
		}
		group, ok := groups[selector]
		if !ok {
			if !looksLikeRepoSelector(selector, groups) {
				continue
			}
			return nil, fmt.Errorf("unknown repo hygiene rule selector %q", selector)
		}
		for _, candidate := range group {
			resolved[candidate.desc.Rule] = candidate
		}
	}

	out := make([]rule, 0, len(resolved))
	for _, candidate := range resolved {
		out = append(out, candidate)
	}
	slices.SortFunc(out, func(left, right rule) int {
		return strings.Compare(left.desc.Rule, right.desc.Rule)
	})
	return out, nil
}

func selectorGroups() map[string][]rule {
	groups := map[string][]rule{
		"repo": nil,
	}
	for _, candidate := range registry {
		groups["repo"] = append(groups["repo"], candidate)
		prefix, _, ok := strings.Cut(candidate.desc.Rule, "/")
		if ok {
			groups[prefix] = append(groups[prefix], candidate)
		}
		if candidate.desc.Category != "" {
			groups[candidate.desc.Category] = append(groups[candidate.desc.Category], candidate)
		}
	}
	return groups
}

func looksLikeRepoSelector(selector string, groups map[string][]rule) bool {
	if selector == "repo" {
		return true
	}
	if _, ok := groups[selector]; ok {
		return true
	}
	prefix, _, ok := strings.Cut(selector, "/")
	if !ok {
		return false
	}
	_, ok = groups[prefix]
	return ok
}

func compactSorted(values []string) []string {
	slices.Sort(values)
	return slices.Compact(values)
}
