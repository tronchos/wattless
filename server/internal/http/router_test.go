package http

import (
	"bytes"
	"context"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/scanner"
	"github.com/tronchos/wattless/server/pkg/urlutil"
)

type stubScanner struct {
	report         scanner.Report
	err            error
}

func (s stubScanner) Scan(ctx context.Context, rawURL string) (scanner.Report, error) {
	return s.report, s.err
}

func TestScanReturnsBadRequestForInvalidURL(t *testing.T) {
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000", RequestTimeout: time.Second}, stubScanner{err: urlutil.ErrInvalidURL}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/scans", bytes.NewBufferString(`{"url":"notaurl"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestScanReturnsBadRequestForBlockedTarget(t *testing.T) {
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000", RequestTimeout: time.Second}, stubScanner{err: urlutil.ErrBlockedTarget}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/scans", bytes.NewBufferString(`{"url":"http://127.0.0.1"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", recorder.Code)
	}
}

func TestScanReturnsReport(t *testing.T) {
	router := NewRouter(config.Config{ClientOrigin: "http://localhost:3000", RequestTimeout: time.Second}, stubScanner{report: scanner.Report{URL: "https://example.com", Score: "A"}}, slog.Default())
	req := httptest.NewRequest(nethttp.MethodPost, "/api/v1/scans", bytes.NewBufferString(`{"url":"https://example.com"}`))
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, req)

	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}
