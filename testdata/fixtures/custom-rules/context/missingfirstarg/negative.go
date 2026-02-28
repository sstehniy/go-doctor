package missingfirstarg

import "context"

func Negative(ctx context.Context, value string) error {
	return downstream(ctx, value)
}
