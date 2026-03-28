package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/queue"
	"github.com/tronchos/wattless/server/pkg/urlutil"
)

type JobQueue interface {
	Submit(context.Context, string, string) (queue.SubmitResult, error)
	Get(context.Context, string) (queue.JobResponse, error)
}

type handler struct {
	cfg    config.Config
	queue  JobQueue
	logger *slog.Logger
}

const maxRequestBodySize = 1 << 20 // 1 MB

func NewRouter(cfg config.Config, jobQueue JobQueue, logger *slog.Logger) http.Handler {
	h := handler{
		cfg:    cfg,
		queue:  jobQueue,
		logger: logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", h.handleHealth)
	mux.HandleFunc("POST /api/v1/scans", h.handleSubmitScan)
	mux.HandleFunc("GET /api/v1/scans/{jobID}", h.handleGetScan)

	return withLogging(logger, withSecurityHeaders(withCORS(cfg.ClientOrigin, mux)))
}

type scanRequest struct {
	URL string `json:"url"`
}

type errorResponse struct {
	Error string `json:"error"`
}

type submitScanResponse struct {
	JobID                string          `json:"job_id"`
	URL                  string          `json:"url"`
	Status               queue.JobStatus `json:"status"`
	Position             int             `json:"position"`
	EstimatedWaitSeconds int             `json:"estimated_wait_seconds,omitempty"`
	Deduplicated         bool            `json:"deduplicated,omitempty"`
}

type conflictResponse struct {
	Error string            `json:"error"`
	Job   queue.JobResponse `json:"job"`
}

func (h handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, h.logger)
}

func (h handler) handleSubmitScan(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	var req scanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid JSON payload"}, h.logger)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.cfg.RequestTimeout)
	defer cancel()

	clientIP := extractClientIP(r)
	result, err := h.queue.Submit(ctx, req.URL, clientIP)
	if err != nil {
		h.logger.Warn("scan_submit_failed", "url", req.URL, "client_ip", clientIP, "error", err)

		switch {
		case isClientError(err):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: clientErrorMessage(err)}, h.logger)
		case errors.Is(err, queue.ErrJobConflict):
			var conflictErr *queue.ConflictError
			if errors.As(err, &conflictErr) {
				writeJSON(w, http.StatusConflict, conflictResponse{
					Error: "Ya tienes un análisis en curso para otra URL. Reanuda ese turno o espera a que termine.",
					Job:   conflictErr.Job,
				}, h.logger)
				return
			}
			writeJSON(w, http.StatusConflict, errorResponse{Error: "Ya existe un análisis en curso para esta IP."}, h.logger)
		case errors.Is(err, queue.ErrDailyLimitExceeded):
			var limitErr *queue.DailyLimitError
			if errors.As(err, &limitErr) {
				writeRetryAfter(w, limitErr.RetryAfter)
				writeJSON(w, http.StatusTooManyRequests, errorResponse{
					Error: dailyLimitMessage(limitErr.Limit, limitErr.RetryAfter),
				}, h.logger)
				return
			}
			writeJSON(w, http.StatusTooManyRequests, errorResponse{Error: "Has alcanzado el límite diario de escaneos."}, h.logger)
		case errors.Is(err, queue.ErrQueueFull):
			writeRetryAfter(w, queue.QueueRetryAfter())
			writeJSON(w, http.StatusServiceUnavailable, errorResponse{
				Error: "La cola está llena en este momento. Inténtalo de nuevo en unos minutos.",
			}, h.logger)
		default:
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "Error interno del servidor"}, h.logger)
		}
		return
	}

	writeJSON(w, http.StatusAccepted, submitScanResponse{
		JobID:                result.Job.JobID,
		URL:                  result.Job.URL,
		Status:               result.Job.Status,
		Position:             result.Job.Position,
		EstimatedWaitSeconds: result.Job.EstimatedWaitSeconds,
		Deduplicated:         result.Deduplicated,
	}, h.logger)
}

func (h handler) handleGetScan(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimSpace(r.PathValue("jobID"))
	if jobID == "" {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "No encontramos ese turno."}, h.logger)
		return
	}

	job, err := h.queue.Get(r.Context(), jobID)
	if err != nil {
		switch {
		case errors.Is(err, queue.ErrJobExpired):
			var expiredErr *queue.ExpiredError
			if errors.As(err, &expiredErr) {
				writeJSON(w, http.StatusGone, expiredErr.Job, h.logger)
				return
			}
			writeJSON(w, http.StatusGone, queue.JobResponse{
				JobID:    jobID,
				Status:   queue.StatusExpired,
				Position: 0,
				Error:    "Tu turno expiró. Envía un nuevo análisis.",
			}, h.logger)
		case errors.Is(err, queue.ErrJobNotFound):
			writeJSON(w, http.StatusNotFound, errorResponse{Error: "No encontramos ese turno."}, h.logger)
		default:
			h.logger.Warn("scan_status_failed", "job_id", jobID, "error", err)
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "Error interno del servidor"}, h.logger)
		}
		return
	}

	writeJSON(w, http.StatusOK, job, h.logger)
}

func isClientError(err error) bool {
	return errors.Is(err, urlutil.ErrInvalidURL) || errors.Is(err, urlutil.ErrBlockedTarget)
}

func clientErrorMessage(err error) string {
	switch {
	case errors.Is(err, urlutil.ErrBlockedTarget):
		return "Solo se permiten URLs públicas. Wattless bloquea localhost, IPs privadas y hosts internos."
	case errors.Is(err, urlutil.ErrInvalidURL):
		return "La URL no es válida o no pudo resolverse correctamente."
	default:
		return "La URL no es válida o no pudo resolverse correctamente."
	}
}

func dailyLimitMessage(limit int, retryAfter time.Duration) string {
	return "Has alcanzado el límite diario de escaneos (" + strconv.Itoa(limit) + "). Podrás volver a escanear en " + humanizeRetryAfter(retryAfter) + "."
}

func humanizeRetryAfter(retryAfter time.Duration) string {
	if retryAfter <= time.Minute {
		return "menos de un minuto"
	}

	hours := int(retryAfter.Hours())
	minutes := int(retryAfter.Minutes()) % 60
	if hours > 0 {
		if minutes == 0 {
			if hours == 1 {
				return "1 hora"
			}
			return strconv.Itoa(hours) + " horas"
		}
		return strconv.Itoa(hours) + "h " + strconv.Itoa(minutes) + "m"
	}

	return strconv.Itoa(int(math.Ceil(retryAfter.Minutes()))) + " minutos"
}

func extractClientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return sanitizeIP(strings.TrimSpace(strings.Split(forwarded, ",")[0]))
	}

	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return sanitizeIP(realIP)
	}

	if clientIdentity := strings.TrimSpace(r.Header.Get("X-Wattless-Client-Id")); clientIdentity != "" {
		return clientIdentity
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return sanitizeIP(host)
	}

	return sanitizeIP(r.RemoteAddr)
}

func sanitizeIP(value string) string {
	trimmed := strings.TrimSpace(value)
	if parsed := net.ParseIP(trimmed); parsed != nil {
		return parsed.String()
	}
	return trimmed
}

func writeRetryAfter(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(math.Ceil(retryAfter.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
}

func writeJSON(w http.ResponseWriter, status int, payload any, logger *slog.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Error("json_encode_failed", "error", err)
	}
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
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

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

func withLogging(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		rec := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rec, r)
		logger.Info("request_completed", "method", r.Method, "path", r.URL.Path, "status", rec.statusCode, "duration_ms", time.Since(startedAt).Milliseconds())
	})
}
