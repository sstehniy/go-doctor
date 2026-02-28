package tabledriven

import "testing"

func helper(v int) int { return v }

func TestPositive(t *testing.T) { // want test/missing-table-driven
	if helper(1) == 0 {
		t.Fatal("unexpected")
	}
	if helper(2) == 0 {
		t.Fatal("unexpected")
	}
	if helper(3) == 0 {
		t.Fatal("unexpected")
	}
}
