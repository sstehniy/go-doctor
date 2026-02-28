package readall

import "io"

func Negative(resp struct{ Body io.Reader }) {
	_, _ = io.ReadAll(io.LimitReader(resp.Body, 64))
}
