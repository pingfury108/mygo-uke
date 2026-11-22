package chordd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
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

func TestAnalyzeUploadsPollsAndReadsResult(t *testing.T) {
	var polls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/analyze":
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				t.Error(err)
			}
			file, _, err := r.FormFile("audio")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			data, _ := io.ReadAll(file)
			if string(data) != "fake audio" {
				t.Errorf("audio = %q", data)
			}
			if got := r.FormValue("duration"); got != "12.500" {
				t.Errorf("duration = %q", got)
			}
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"jobId":"job1","etaMs":10}`))
		case "/v1/jobs/job1":
			polls++
			if polls < 2 {
				_, _ = w.Write([]byte(`{"status":"running","progress":0.5,"error":null}`))
				return
			}
			_, _ = w.Write([]byte(`{"status":"done","progress":1,"error":null}`))
		case "/v1/jobs/job1/result":
			_, _ = w.Write([]byte(`{
				"tempo": 68,
				"timeSignature": [4,4],
				"key": "C",
				"beats": [0, 0.5],
				"downbeats": [0],
				"chords": [{"start":0,"end":2,"label":"C","conf":0.9}]
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "audio.mp3")
	if err := os.WriteFile(path, []byte("fake audio"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := New(srv.URL)
	client.Poll = time.Millisecond
	var progress []float64
	result, err := client.Analyze(
		context.Background(), path, 12.5, func(value float64) { progress = append(progress, value) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Tempo != 68 || result.Key != "C" || len(result.Chords) != 1 {
		t.Fatalf("result = %+v", result)
	}
	if result.TimeSignature != [2]int{4, 4} {
		t.Fatalf("time signature = %v", result.TimeSignature)
	}
	if len(progress) < 2 || progress[0] != 0 {
		t.Fatalf("progress = %v", progress)
	}
}

func TestAnalyzeReportsServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "audio too large", http.StatusRequestEntityTooLarge)
	}))
	defer srv.Close()

	path := filepath.Join(t.TempDir(), "audio.mp3")
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := New(srv.URL).Analyze(context.Background(), path, 1, nil)
	if err == nil {
		t.Fatal("expected an error for a 413 response")
	}
}
