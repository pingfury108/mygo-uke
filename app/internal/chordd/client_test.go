package chordd

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"analyzer":"mock","models":[]}`))
	}))
	defer srv.Close()

	health, err := New(srv.URL).Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !health.OK || health.Analyzer != "mock" {
		t.Fatalf("health = %+v", health)
	}
}

func TestCheckReportsHTTPFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "starting", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if _, err := New(srv.URL).Check(context.Background()); err == nil {
		t.Fatal("expected an error for a 503 response")
	}
}

func TestCheckReportsUnhealthyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":false}`))
	}))
	defer srv.Close()

	if _, err := New(srv.URL).Check(context.Background()); err == nil {
		t.Fatal("expected an error for ok=false")
	}
}
