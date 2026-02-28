package bytescopy

import "bytes"

func Positive(value string) *bytes.Buffer {
	return bytes.NewBuffer([]byte(value)) // want perf/bytes-buffer-copy
}
