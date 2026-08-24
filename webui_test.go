package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithWebUIForwardsWebSocketPath(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Via", "api")
		w.WriteHeader(http.StatusTeapot)
	})
	h := withWebUI(api)

	for _, path := range []string{"/ws/term", "/api/system"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", path, nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusTeapot || rec.Header().Get("X-Via") != "api" {
			t.Fatalf("%s went to SPA (code=%d via=%s)", path, rec.Code, rec.Header().Get("X-Via"))
		}
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Header().Get("X-Via") == "api" {
		t.Fatal("SPA / should not hit the API mux")
	}
}
