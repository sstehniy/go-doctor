package handlernotest

import "net/http"

func Missing(w http.ResponseWriter, r *http.Request) { // want test/http-handler-no-test
	_, _ = w.Write([]byte("ok"))
}
