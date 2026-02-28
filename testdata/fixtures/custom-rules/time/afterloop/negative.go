package afterloop

import "time"

func Negative() {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	<-timer.C
}
