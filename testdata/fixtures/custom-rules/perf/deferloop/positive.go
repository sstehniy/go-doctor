package deferloop

func cleanup() {}

func Positive(values []int) {
	for range values {
		defer cleanup() // want perf/defer-in-hot-loop
	}
}
