package godoctor

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkHardeningFixtures(b *testing.B) {
	testCases := []struct {
		name string
		path string
		opts Options
	}{
		{
			name: "basic-api-smells",
			path: hardeningFixturePath("basic-api-smells"),
			opts: Options{
				Timeout:      20 * time.Second,
				Concurrency:  2,
				EnableRules:  []string{"error/string-compare", "api/exported-mutable-global"},
				RepoHygiene:  false,
				ThirdParty:   false,
				Custom:       true,
				Score:        false,
				BaselinePath: "",
			},
		},
		{
			name: "mod-hygiene-demo",
			path: hardeningFixturePath("mod-hygiene-demo"),
			opts: Options{
				Timeout:      20 * time.Second,
				Concurrency:  2,
				EnableRules:  []string{"mod/replace-local-path"},
				RepoHygiene:  true,
				ThirdParty:   false,
				Custom:       false,
				Score:        false,
				BaselinePath: "",
			},
		},
		{
			name: "workspace-multi-module",
			path: hardeningFixturePath("workspace-multi-module"),
			opts: Options{
				Timeout:      20 * time.Second,
				Concurrency:  2,
				RepoHygiene:  false,
				ThirdParty:   false,
				Custom:       false,
				Score:        false,
				BaselinePath: "",
			},
		},
	}

	for _, testCase := range testCases {
		b.Run(testCase.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := Diagnose(context.Background(), testCase.path, testCase.opts); err != nil {
					b.Fatalf("diagnose %s: %v", testCase.name, err)
				}
			}
		})
	}
}

func hardeningFixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", "fixtures", "integration", name)
}
