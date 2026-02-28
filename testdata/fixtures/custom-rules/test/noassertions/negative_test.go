package noassertions

import "testing"

func TestNegative(t *testing.T) {
	t.Fatal("boom")
}
