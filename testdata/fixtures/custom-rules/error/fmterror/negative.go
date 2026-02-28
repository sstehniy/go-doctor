package fmterror

import "fmt"

func Negative(err error) error {
	return fmt.Errorf("wrap: %w", err)
}
