package fmterror

import "fmt"

func Positive(err error) error {
	return fmt.Errorf("wrap: %v", err) // want error/fmt-error-without-wrap
}
