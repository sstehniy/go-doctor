package handlernotest

import (
	"net/http/httptest"
	"testing"
)

func TestPositive(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	Covered(rec, req)
	if rec.Code != 200 {
		t.Fatal("unexpected status")
	}
}
