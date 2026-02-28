package withtimeout

import (
	"context"
	"time"
)

func Negative(ctx context.Context) {
	derived, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = derived
}
