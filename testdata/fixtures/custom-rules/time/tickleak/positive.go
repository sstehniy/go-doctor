package tickleak

import "time"

func Positive() {
	_ = time.Tick(time.Second) // want time/tick-leak
}
