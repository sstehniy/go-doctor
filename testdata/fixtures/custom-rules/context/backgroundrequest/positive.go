package backgroundrequest

import (
	"context"
	"net/http"
)

func Positive(w http.ResponseWriter, r *http.Request) {
	_ = context.Background() // want context/background-in-request-path
}
