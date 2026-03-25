package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/insights"
	"github.com/tronchos/wattless/server/internal/scanner"
	"github.com/tronchos/wattless/server/pkg/urlutil"
)

type ScanService interface {
	Scan(context.Context, string) (scanner.Report, error)
	RefactorCode(context.Context, insights.RefactorRequest) (insights.RefactorResult, error)
}

type handler struct {
	cfg     config.Config
	scanner ScanService
	logger  *slog.Logger
}

func NewRouter(cfg config.Config, scanService ScanService, logger *slog.Logger) http.Handler {
	h := handler{
		cfg:     cfg,
		scanner: scanService,
		logger:  logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealth)
	mux.HandleFunc("POST /api/v1/scans", h.handleScan)
	mux.HandleFunc("POST /api/v1/green-fix", h.handleGreenFix)

	return withLogging(logger, withCORS(cfg.ClientOrigin, mux))
}

type scanRequest struct {
	URL string `json:"url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

const minimumGreenFixCodeLength = 20

func (h handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h handler) handleScan(w http.ResponseWriter, r *http.Request) {
	var req scanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON payload"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.RequestTimeout)
	defer cancel()

	report, err := h.scanner.Scan(ctx, req.URL)
	if err != nil {
		status := http.StatusInternalServerError
		if isClientError(err) {
			status = http.StatusBadRequest
		}
		h.logger.Warn("scan_failed", "url", req.URL, "error", err)
		writeJSON(w, status, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, report)
}

func (h handler) handleGreenFix(w http.ResponseWriter, r *http.Request) {
	var req insights.RefactorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON payload"})
		return
	}

	req.Code = strings.TrimSpace(req.Code)
	if req.Code == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "Debes pegar un snippet antes de generar el Green Fix."})
		return
	}
	if len(req.Code) < minimumGreenFixCodeLength {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "El snippet es demasiado corto para proponer un refactor útil."})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.RequestTimeout)
	defer cancel()

	result, err := h.scanner.RefactorCode(ctx, req)
	if err != nil {
		h.logger.Warn("green_fix_failed", "error", err)
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func isClientError(err error) bool {
	return errors.Is(err, urlutil.ErrInvalidURL)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func withCORS(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		next.ServeHTTP(w, r)
		logger.Info("request_completed", "method", r.Method, "path", r.URL.Path, "duration_ms", time.Since(startedAt).Milliseconds())
	})
}
