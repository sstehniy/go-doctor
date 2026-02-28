package waitgroup

import "sync"

func Positive() {
	var wg sync.WaitGroup
	go func() {
		wg.Add(1) // want concurrency/waitgroup-misuse
		defer wg.Done()
	}()
}
