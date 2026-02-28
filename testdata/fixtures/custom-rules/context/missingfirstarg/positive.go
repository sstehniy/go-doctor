package missingfirstarg

import "context"

func downstream(ctx context.Context, value string) error { return nil }

func Positive(value string) error { // want context/missing-first-arg
	return downstream(context.Background(), value)
}
