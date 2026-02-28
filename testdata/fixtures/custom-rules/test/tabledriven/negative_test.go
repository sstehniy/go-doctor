package tabledriven

import "testing"

func TestNegative(t *testing.T) {
	cases := []int{1, 2, 3}
	for _, value := range cases {
		if helper(value) == 0 {
			t.Fatal("unexpected")
		}
	}
}
