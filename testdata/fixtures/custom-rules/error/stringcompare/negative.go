package stringcompare

import "errors"

var errBoom = errors.New("boom")

func Negative(err error) bool {
	return errors.Is(err, errBoom)
}
