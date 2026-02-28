package custom

import "testing"

func TestSelectRulesDefaultsAndSelectors(t *testing.T) {
	t.Run("default selection keeps default-on and excludes opt-in", func(t *testing.T) {
		selected, err := selectRules(nil, nil)
		if err != nil {
			t.Fatalf("select rules: %v", err)
		}

		got := map[string]struct{}{}
		for _, candidate := range selected {
			got[candidate.desc.Rule] = struct{}{}
		}
		if _, ok := got["error/string-compare"]; !ok {
			t.Fatal("expected default-on rule error/string-compare to be selected")
		}
		if _, ok := got["error/ignored-return"]; ok {
			t.Fatal("expected opt-in rule error/ignored-return to stay out of default set")
		}
	})

	t.Run("group selector expands to matching rule set", func(t *testing.T) {
		selected, err := selectRules([]string{"context"}, nil)
		if err != nil {
			t.Fatalf("select rules: %v", err)
		}

		got := map[string]struct{}{}
		for _, candidate := range selected {
			got[candidate.desc.Rule] = struct{}{}
		}
		if _, ok := got["context/not-propagated"]; !ok {
			t.Fatal("expected context selector to include context/not-propagated")
		}
		if _, ok := got["concurrency/waitgroup-misuse"]; ok {
			t.Fatal("expected context selector to exclude non-context rule")
		}
	})
}

func TestResolveSelectorsRejectsUnknownCustomSelector(t *testing.T) {
	_, err := selectRules([]string{"context/not-real"}, nil)
	if err == nil {
		t.Fatal("expected unknown custom selector to fail")
	}
}
