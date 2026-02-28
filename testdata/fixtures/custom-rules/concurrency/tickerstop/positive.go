package tickerstop

import "time"

func Positive() {
	ticker := time.NewTicker(time.Second) // want concurrency/ticker-not-stopped
	_ = ticker
}
