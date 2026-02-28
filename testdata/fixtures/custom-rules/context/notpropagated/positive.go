package notpropagated

import "context"

func downstream(ctx context.Context) {}

func Positive(ctx context.Context) {
	downstream(context.Background()) // want context/not-propagated
}
