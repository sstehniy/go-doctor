package backgroundrequest

import (
	"context"
	"net/http"
)

func Negative(w http.ResponseWriter, r *http.Request) {
	_ = r.Context()
	_, cancel := context.WithCancel(r.Context())
	defer cancel()
}
