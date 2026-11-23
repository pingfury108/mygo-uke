// Package chordd is the HTTP client for the Python analysis service.
package chordd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Client talks to one chordd deployment.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	Poll    time.Duration
}

// Health is GET /healthz.
type Health struct {
	OK       bool     `json:"ok"`
	Analyzer string   `json:"analyzer"`
	Models   []string `json:"models"`
}

// New returns a client with a short timeout, suitable for status checks.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 60 * time.Second},
		Poll:    700 * time.Millisecond,
	}
}

// Result is GET /v1/jobs/{id}/result.
type Result struct {
	Tempo         float64   `json:"tempo"`
	TimeSignature [2]int    `json:"timeSignature"`
	Key           string    `json:"key"`
	Beats         []float64 `json:"beats"`
	Downbeats     []float64 `json:"downbeats"`
	Chords        []Chord   `json:"chords"`
}

// Chord is one recognised chord segment.
type Chord struct {
	Start      float64  `json:"start"`
	End        float64  `json:"end"`
	Label      string   `json:"label"`
	Conf       float64  `json:"conf"`
	Candidates []string `json:"candidates,omitempty"`
}

type accepted struct {
	JobID string `json:"jobId"`
	EtaMS int    `json:"etaMs"`
}

type jobStatus struct {
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Error    string  `json:"error"`
}

// Check calls GET /healthz.
func (c *Client) Check(ctx context.Context) (Health, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/healthz", nil)
	if err != nil {
		return Health{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Health{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Health{}, fmt.Errorf("chordd health: %s", resp.Status)
	}
	var health Health
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return Health{}, err
	}
	if !health.OK {
		return Health{}, fmt.Errorf("chordd health: not ok")
	}
	return health, nil
}

// Analyze uploads one audio file, waits for the job, and returns its result.
func (c *Client) Analyze(
	ctx context.Context,
	audioPath string,
	duration float64,
	progress func(float64),
) (Result, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("audio", filepath.Base(audioPath))
	if err != nil {
		return Result{}, err
	}
	file, err := os.Open(audioPath)
	if err != nil {
		return Result{}, err
	}
	if _, err := io.Copy(part, file); err != nil {
		_ = file.Close()
		return Result{}, err
	}
	_ = file.Close()
	if err := writer.WriteField("duration", strconv.FormatFloat(duration, 'f', 3, 64)); err != nil {
		return Result{}, err
	}
	if err := writer.Close(); err != nil {
		return Result{}, err
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.BaseURL+"/v1/analyze", &body,
	)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		return Result{}, responseError("analyze", resp)
	}
	var job accepted
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return Result{}, err
	}
	if job.JobID == "" {
		return Result{}, fmt.Errorf("chordd analyze: missing jobId")
	}
	if progress != nil {
		progress(0)
	}

	ticker := time.NewTicker(c.Poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		case <-ticker.C:
			status, err := c.status(ctx, job.JobID)
			if err != nil {
				return Result{}, err
			}
			switch status.Status {
			case "failed":
				if status.Error == "" {
					status.Error = "analysis failed"
				}
				return Result{}, fmt.Errorf("chordd: %s", status.Error)
			case "done":
				return c.result(ctx, job.JobID)
			}
			if progress != nil {
				progress(status.Progress)
			}
		}
	}
}

func (c *Client) status(ctx context.Context, id string) (jobStatus, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.BaseURL+"/v1/jobs/"+id, nil,
	)
	if err != nil {
		return jobStatus{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return jobStatus{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return jobStatus{}, responseError("job status", resp)
	}
	var status jobStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return jobStatus{}, err
	}
	return status, nil
}

func (c *Client) result(ctx context.Context, id string) (Result, error) {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, c.BaseURL+"/v1/jobs/"+id+"/result", nil,
	)
	if err != nil {
		return Result{}, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Result{}, responseError("job result", resp)
	}
	var result Result
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func responseError(prefix string, resp *http.Response) error {
	detail, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	message := strings.TrimSpace(string(detail))
	if message == "" {
		message = resp.Status
	}
	return fmt.Errorf("chordd %s: %s", prefix, message)
}
