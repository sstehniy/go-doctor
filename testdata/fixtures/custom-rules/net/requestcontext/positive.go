package requestcontext

import "net/http"

func Positive(url string) {
	_, _ = http.NewRequest("GET", url, nil) // want net/http-request-without-context
}
