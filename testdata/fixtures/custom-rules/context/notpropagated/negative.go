package notpropagated

import "context"

func Negative(ctx context.Context) {
	downstream(ctx)
}
