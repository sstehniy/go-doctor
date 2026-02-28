package goroutineleak

func Positive() {
	go func() { // want concurrency/go-routine-leak-risk
		for {
		}
	}()
}
