package goroutineleak

import "context"

func Negative(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
}
