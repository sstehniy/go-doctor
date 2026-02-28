package deferloop

func Negative() {
	defer cleanup()
}

func NegativeNested(values []int) {
	for range values {
		func() {
			defer cleanup()
		}()
	}
}
