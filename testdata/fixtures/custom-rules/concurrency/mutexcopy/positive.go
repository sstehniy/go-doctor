package mutexcopy

import "sync"

type Counter struct {
	mu sync.Mutex
}

func (c Counter) Inc() { // want concurrency/mutex-copy
	c.mu.Lock()
	defer c.mu.Unlock()
}
