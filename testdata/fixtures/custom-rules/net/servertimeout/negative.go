package servertimeout

import (
	"net/http"
	"time"
)

func Negative() {
	_ = &http.Server{Addr: ":8080", ReadHeaderTimeout: time.Second}
}
