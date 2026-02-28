package requestcontext

import (
	"context"
	"net/http"
)

func Negative(ctx context.Context, url string) {
	_, _ = http.NewRequestWithContext(ctx, "GET", url, nil)
}
