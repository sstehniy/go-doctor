package readall

import "io"

func Positive(resp struct{ Body io.Reader }) {
	_, _ = io.ReadAll(resp.Body) // want io/readall-unbounded
}
