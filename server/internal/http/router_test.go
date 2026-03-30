package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/queue"
	"github.com/tronchos/wattless/server/internal/scanner"
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

func testRouterConfig() config.Config {
	return config.Config{
		ClientOrigin:   "http://localhost:3000,http://localhost:5173",
		RequestTimeout: time.Second,
		ResultTTL:      90 * time.Second,
	}
}

func newScreenshotReport(mimeType string, tilePayloads ...string) *scanner.Report {
	tiles := make([]scanner.ScreenshotTile, 0, len(tilePayloads))
	for index, payload := range tilePayloads {
		tiles = append(tiles, scanner.ScreenshotTile{
			ID:         "tile-" + strconv.Itoa(index),
			DataBase64: base64.StdEncoding.EncodeToString([]byte(payload)),
		})
	}

	return &scanner.Report{
		Screenshot: scanner.Screenshot{
			MimeType: mimeType,
			Tiles:    tiles,
		},
	}
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

func TestGetScreenshotReturnsBinaryTileWithPrivateTTLCache(t *testing.T) {
	router := NewRouter(testRouterConfig(), stubQueue{
		getResult: queue.JobResponse{
			JobID:  "wl_123",
			Status: queue.StatusCompleted,
			Report: newScreenshotReport("image/png", "first-tile", "second-tile"),
		},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_123/screenshot?tile=1", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "image/png" {
		t.Fatalf("expected image/png content type, got %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "private, max-age=90" {
		t.Fatalf("expected private ttl cache, got %q", got)
	}
	if got := recorder.Body.String(); got != "second-tile" {
		t.Fatalf("unexpected screenshot payload %q", got)
	}
}

func TestGetScreenshotReturnsGoneForExpiredJob(t *testing.T) {
	expiredJob := queue.JobResponse{
		JobID:    "wl_expired",
		URL:      "https://example.com",
		Status:   queue.StatusExpired,
		Position: 0,
		Error:    "Tu turno expiró por inactividad. Envía un nuevo análisis.",
	}
	router := NewRouter(testRouterConfig(), stubQueue{
		getErr: &queue.ExpiredError{Job: expiredJob},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_expired/screenshot", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusGone {
		t.Fatalf("expected status 410, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache on errors, got %q", got)
	}

	var payload queue.JobResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("expected valid job payload: %v", err)
	}
	if payload.Status != queue.StatusExpired {
		t.Fatalf("expected expired job payload, got %#v", payload)
	}
}

func TestGetScreenshotReturnsNotFoundForMissingJob(t *testing.T) {
	router := NewRouter(testRouterConfig(), stubQueue{
		getErr: queue.ErrJobNotFound,
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_missing/screenshot", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache on errors, got %q", got)
	}
}

func TestGetScreenshotReturnsNotFoundWhenTilesAreMissing(t *testing.T) {
	router := NewRouter(testRouterConfig(), stubQueue{
		getResult: queue.JobResponse{
			JobID:  "wl_empty",
			Status: queue.StatusCompleted,
			Report: &scanner.Report{},
		},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_empty/screenshot", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
}

func TestGetScreenshotReturnsNotFoundWhenReportIsMissing(t *testing.T) {
	router := NewRouter(testRouterConfig(), stubQueue{
		getResult: queue.JobResponse{
			JobID:  "wl_missing_report",
			Status: queue.StatusCompleted,
		},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_missing_report/screenshot", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusNotFound {
		t.Fatalf("expected status 404, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache on errors, got %q", got)
	}
}

func TestGetScreenshotReturnsInternalServerErrorForQueueFailure(t *testing.T) {
	router := NewRouter(testRouterConfig(), stubQueue{
		getErr: errors.New("boom"),
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_500/screenshot", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache on errors, got %q", got)
	}
}

func TestGetScreenshotRejectsInvalidTileQuery(t *testing.T) {
	router := NewRouter(testRouterConfig(), stubQueue{
		getResult: queue.JobResponse{
			JobID:  "wl_tiles",
			Status: queue.StatusCompleted,
			Report: newScreenshotReport("image/webp", "tile-zero"),
		},
	}, slog.Default())

	for _, rawTile := range []string{"abc", "-1", "9"} {
		req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_tiles/screenshot?tile="+rawTile, nil)
		recorder := httptest.NewRecorder()

		router.ServeHTTP(recorder, req)

		if recorder.Code != nethttp.StatusBadRequest {
			t.Fatalf("expected status 400 for tile %q, got %d", rawTile, recorder.Code)
		}
		if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
			t.Fatalf("expected no-store cache for tile %q, got %q", rawTile, got)
		}
	}
}

func TestGetScreenshotReturnsInternalServerErrorForCorruptBase64(t *testing.T) {
	router := NewRouter(testRouterConfig(), stubQueue{
		getResult: queue.JobResponse{
			JobID:  "wl_corrupt",
			Status: queue.StatusCompleted,
			Report: &scanner.Report{
				Screenshot: scanner.Screenshot{
					MimeType: "image/webp",
					Tiles: []scanner.ScreenshotTile{
						{ID: "tile-0", DataBase64: "%%%"},
					},
				},
			},
		},
	}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodGet, "/api/v1/scans/wl_corrupt/screenshot", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("expected no-store cache on errors, got %q", got)
	}
}

func TestCORSReflectsOnlyAllowedOrigins(t *testing.T) {
	testCases := []struct {
		name          string
		allowed       string
		requestOrigin string
		wantOrigin    string
	}{
		{
			name:          "matches second configured origin",
			allowed:       "http://localhost:3000,http://localhost:5173",
			requestOrigin: "http://localhost:5173",
			wantOrigin:    "http://localhost:5173",
		},
		{
			name:          "rejects unknown origin",
			allowed:       "http://localhost:3000,http://localhost:5173",
			requestOrigin: "http://evil.example",
			wantOrigin:    "",
		},
		{
			name:          "allows wildcard",
			allowed:       "*",
			requestOrigin: "http://evil.example",
			wantOrigin:    "*",
		},
		{
			name:          "keeps same-origin requests headerless when origin missing",
			allowed:       "http://localhost:3000,http://localhost:5173",
			requestOrigin: "",
			wantOrigin:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouter(config.Config{ClientOrigin: tc.allowed}, stubQueue{}, slog.Default())
			req := httptest.NewRequest(nethttp.MethodGet, "/healthz", nil)
			if tc.requestOrigin != "" {
				req.Header.Set("Origin", tc.requestOrigin)
			}
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != tc.wantOrigin {
				t.Fatalf("expected allow-origin %q, got %q", tc.wantOrigin, got)
			}
			if got := recorder.Header().Get("Vary"); got != "Origin" {
				t.Fatalf("expected Vary Origin, got %q", got)
			}
		})
	}
}

func TestCORSPreflightReturnsAllowedHeadersForConfiguredOrigin(t *testing.T) {
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000,http://localhost:5173"}, stubQueue{}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodOptions, "/healthz", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusNoContent {
		t.Fatalf("expected status 204, got %d", recorder.Code)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected reflected origin, got %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, X-Wattless-Client-Id" {
		t.Fatalf("unexpected allow-headers %q", got)
	}
}
