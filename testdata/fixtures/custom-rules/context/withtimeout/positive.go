package withtimeout

import (
	"context"
	"time"
)

func Positive(ctx context.Context) {
	derived, cancel := context.WithTimeout(ctx, time.Second) // want context/with-timeout-not-canceled
	_ = derived
	_ = cancel
}
