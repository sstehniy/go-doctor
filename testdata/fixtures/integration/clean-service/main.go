package main

import (
	"context"
	"fmt"
)

func run(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Println(err)
	}
}
