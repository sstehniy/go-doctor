package main

import (
	"net/http"
)

func main() {
	req, _ := http.NewRequest(http.MethodGet, "https://example.com", nil)
	_, _ = http.DefaultClient.Do(req)
	_ = http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
}
