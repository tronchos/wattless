package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/queue"
	"github.com/tronchos/wattless/server/pkg/urlutil"
)

type stubQueue struct {
	submitResult queue.SubmitResult
	submitErr    error
	getResult    queue.JobResponse
	getErr       error
}

func (s stubQueue) Submit(ctx context.Context, rawURL, clientIP string) (queue.SubmitResult, error) {
	return s.submitResult, s.submitErr
}

func (s stubQueue) Get(ctx context.Context, jobID string) (queue.JobResponse, error) {
	return s.getResult, s.getErr
}

func TestSubmitScanReturnsBadRequestForInvalidURL(t *testing.T) {
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000", RequestTimeout: time.Second}, stubQueue{submitErr: urlutil.ErrInvalidURL}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/scans", bytes.NewBufferString(`{"url":"notaurl"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestSubmitScanReturnsAcceptedJob(t *testing.T) {
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000", RequestTimeout: time.Second}, stubQueue{
		submitResult: queue.SubmitResult{
			Job: queue.JobResponse{
				JobID:                "wl_123",
				URL:                  "https://example.com",
				Status:               queue.StatusQueued,
				Position:             2,
				EstimatedWaitSeconds: 30,
			},
		},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/scans", bytes.NewBufferString(`{"url":"https://example.com"}`))
	req.Header.Set("X-Forwarded-For", "203.0.113.10")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusAccepted {
		t.Fatalf("expected status 202, got %d", recorder.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid JSON response: %v", err)
	}
	if payload["job_id"] != "wl_123" {
		t.Fatalf("unexpected job payload: %#v", payload)
	}
}

func TestSubmitScanReturnsConflictJob(t *testing.T) {
	conflictJob := queue.JobResponse{
		JobID:    "wl_conflict",
		URL:      "https://example.com",
		Status:   queue.StatusScanning,
		Position: 0,
	}
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000", RequestTimeout: time.Second}, stubQueue{
		submitErr: &queue.ConflictError{Job: conflictJob},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/scans", bytes.NewBufferString(`{"url":"https://other.com"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusConflict {
		t.Fatalf("expected status 409, got %d", recorder.Code)
	}
}

func TestGetScanReturnsGoneForExpiredJob(t *testing.T) {
	expiredJob := queue.JobResponse{
		JobID:    "wl_expired",
		URL:      "https://example.com",
		Status:   queue.StatusExpired,
		Position: 0,
		Error:    "Tu turno expiró por inactividad. Envía un nuevo análisis.",
	}
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000"}, stubQueue{
		getErr: &queue.ExpiredError{Job: expiredJob},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_expired", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusGone {
		t.Fatalf("expected status 410, got %d", recorder.Code)
	}
}

func TestGetScanReturnsNotFound(t *testing.T) {
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000"}, stubQueue{
		getErr: queue.ErrJobNotFound,
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_missing", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

func TestSubmitScanReturnsRetryAfterForRateLimit(t *testing.T) {
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000", RequestTimeout: time.Second}, stubQueue{
		submitErr: &queue.DailyLimitError{Limit: 20, RetryAfter: 2 * time.Hour},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/scans", bytes.NewBufferString(`{"url":"https://example.com"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusTooManyRequests {
		t.Fatalf("expected status 429, got %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestClientErrorClassifier(t *testing.T) {
	if !isClientError(urlutil.ErrBlockedTarget) {
		t.Fatal("expected blocked target to be classified as client error")
	}
	if isClientError(errors.New("boom")) {
		t.Fatal("unexpected client error classification")
	}
}

func TestExtractClientIPFallsBackToStableClientIdentity(t *testing.T) {
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/scans", nil)
	req.Header.Set("X-Wattless-Client-Id", "wlc_test_client")

	clientIP := extractClientIP(req)
	if clientIP != "wlc_test_client" {
		t.Fatalf("expected fallback client identity, got %q", clientIP)
	}
}
