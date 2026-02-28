package stringcompare

func Positive(err error) bool {
	return err != nil && err.Error() == "boom" // want error/string-compare
}
