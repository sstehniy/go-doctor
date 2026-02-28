package mutexcopy

func (c *Counter) Dec() {
	c.mu.Lock()
	defer c.mu.Unlock()
}

func NegativeAssign() {
	counter := &Counter{}
	_ = counter
}
