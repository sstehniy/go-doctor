package scoring

type Result struct {
	Enabled bool
	Value   int
	Max     int
	Grade   string
}

func Score(diagnosticCount int, enabled bool) Result {
	if !enabled {
		return Result{Enabled: false}
	}

	value := 100 - (diagnosticCount * 10)
	if value < 0 {
		value = 0
	}

	grade := "A"
	switch {
	case value >= 90:
		grade = "A"
	case value >= 80:
		grade = "B"
	case value >= 70:
		grade = "C"
	case value >= 60:
		grade = "D"
	default:
		grade = "F"
	}

	return Result{
		Enabled: true,
		Value:   value,
		Max:     100,
		Grade:   grade,
	}
}
