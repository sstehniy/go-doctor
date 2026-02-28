package handlernotest

import "net/http"

func Covered(w http.ResponseWriter, r *http.Request) {
	Missing(w, r)
}
