package errorbranch

func Positive(err error) bool {
	switch err.Error() { // want api/error-string-branching
	case "boom":
		return true
	default:
		return false
	}
}
