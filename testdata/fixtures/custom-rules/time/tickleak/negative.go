package tickleak

import "time"

func Negative() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
}
