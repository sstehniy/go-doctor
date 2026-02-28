package fmtconcat

import "fmt"

func Positive(a string, b string) string {
	return fmt.Sprintf("%s:%s", a, b) // want perf/fmt-sprint-simple-concat
}
