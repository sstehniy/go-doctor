package afterloop

import "time"

func Positive(values []int) {
	for range values { // want time/after-in-loop
		<-time.After(time.Second)
	}
}
