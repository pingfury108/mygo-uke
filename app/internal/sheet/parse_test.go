package sheet

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseImageUsesOpenAICompatibleEndpoint(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization = %q", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var request map[string]any
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
			return
		}
		requests = append(requests, request)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{
			"id":"chatcmpl-test","object":"chat.completion","created":0,
			"model":"vision-test",
			"choices":[{"index":0,"finish_reason":"stop","message":{
				"role":"assistant",
				"content":"{\"key\":\"C\",\"capo\":0,\"lines\":[{\"chords\":[\"F\",\"G\"],\"lyrics\":\"好想能看到 你嘴角微笑\"}]}"
			}}]
		}`)
	}))
	defer server.Close()

	path := filepath.Join(t.TempDir(), "chart.png")
	if err := os.WriteFile(path, []byte("not-a-real-png"), 0o600); err != nil {
		t.Fatal(err)
	}
	client, err := New(context.Background(), Config{
		BaseURL: server.URL,
		APIKey:  "test-key",
		Model:   "vision-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := client.ParseImage(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Key != "C" || len(parsed.Lines) != 1 || parsed.Lines[0].Lyrics != "好想能看到 你嘴角微笑" {
		t.Fatalf("parsed = %+v", parsed)
	}
	if len(requests) != 1 {
		t.Fatalf("requests = %d", len(requests))
	}
	if requests[0]["model"] != "vision-test" {
		t.Errorf("model = %v", requests[0]["model"])
	}
	if url := requestImageURL(t, requests[0]); !strings.HasPrefix(url, "data:image/png;base64,") {
		t.Errorf("image url = %q", url)
	}

	if _, err := client.ParseImageData(context.Background(), []byte("clipboard"), "image/webp"); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d", len(requests))
	}
	if url := requestImageURL(t, requests[1]); !strings.HasPrefix(url, "data:image/webp;base64,") {
		t.Errorf("clipboard image url = %q", url)
	}
}

func requestImageURL(t *testing.T, request map[string]any) string {
	t.Helper()
	messages, _ := request["messages"].([]any)
	if len(messages) != 1 {
		t.Fatalf("messages = %#v", request["messages"])
	}
	message, _ := messages[0].(map[string]any)
	parts, _ := message["content"].([]any)
	if len(parts) != 2 {
		t.Fatalf("content parts = %#v", message["content"])
	}
	image, _ := parts[1].(map[string]any)
	imageURL, _ := image["image_url"].(map[string]any)
	url, _ := imageURL["url"].(string)
	return url
}
