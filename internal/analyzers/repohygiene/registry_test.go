package repohygiene

import "testing"

func TestSelectRulesDefaultsAndSelectors(t *testing.T) {
	t.Run("default selection keeps default-on and excludes optional", func(t *testing.T) {
		selected, err := selectRules(nil, nil)
		if err != nil {
			t.Fatalf("select rules: %v", err)
		}

		got := map[string]struct{}{}
		for _, candidate := range selected {
			got[candidate.desc.Rule] = struct{}{}
		}
		if _, ok := got["mod/not-tidy"]; !ok {
			t.Fatal("expected mod/not-tidy to be selected by default")
		}
		if _, ok := got["fmt/not-gofmt"]; ok {
			t.Fatal("expected default-off rule fmt/not-gofmt to stay disabled")
		}
	})

	t.Run("group selector expands to matching rule set", func(t *testing.T) {
		selected, err := selectRules([]string{"mod"}, nil)
		if err != nil {
			t.Fatalf("select rules: %v", err)
		}

		got := map[string]struct{}{}
		for _, candidate := range selected {
			got[candidate.desc.Rule] = struct{}{}
		}
		if _, ok := got["mod/not-tidy"]; !ok {
			t.Fatal("expected mod selector to include mod/not-tidy")
		}
		if _, ok := got["build/mod-readonly-failure"]; ok {
			t.Fatal("expected mod selector to exclude build/mod-readonly-failure")
		}
	})
}

func TestResolveSelectorsRejectsUnknownRepoSelector(t *testing.T) {
	_, err := selectRules([]string{"repo/not-real"}, nil)
	if err == nil {
		t.Fatal("expected unknown repo selector to fail")
	}
}
