package defaultclient

import "net/http"

func Positive(url string) {
	_, _ = http.Get(url) // want net/http-default-client
}
