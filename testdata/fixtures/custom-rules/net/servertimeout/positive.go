package servertimeout

import "net/http"

func Positive() {
	_ = &http.Server{Addr: ":8080"} // want net/http-server-no-timeouts
}
