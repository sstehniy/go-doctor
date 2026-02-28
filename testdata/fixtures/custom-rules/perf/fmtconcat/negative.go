package fmtconcat

import "fmt"

func Negative(a string, b int) string {
	return fmt.Sprintf("%s:%d", a, b)
}
