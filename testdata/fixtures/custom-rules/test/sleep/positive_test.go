package sleep

import (
	"testing"
	"time"
)

func TestPositive(t *testing.T) {
	time.Sleep(time.Millisecond) // want test/sleep-in-test
	t.Fatal("finished")
}
