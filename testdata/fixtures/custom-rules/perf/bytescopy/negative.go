package bytescopy

import "bytes"

func Negative(value string) *bytes.Buffer {
	return bytes.NewBufferString(value)
}
