package queue

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/scanner"
)

const (
	cleanupInterval         = 30 * time.Second
	initialAverageDuration  = 15 * time.Second
	averageDurationWindow   = 20
	defaultTombstoneTTL     = 2 * time.Minute
	defaultQueueRetryAfter  = 30 * time.Second
	expiredQueuedJobMessage = "Tu turno expiró por inactividad. Envía un nuevo análisis."
	expiredResultMessage    = "Este resultado expiró. Envía un nuevo análisis."
)

type ScanService interface {
	PrepareTarget(context.Context, string) (scanner.PreparedTarget, error)
	ScanPrepared(context.Context, scanner.PreparedTarget) (scanner.Report, error)
}

type ipEntry struct {
	Count     int
	WindowEnd time.Time
}

type tombstone struct {
	Job       JobResponse
	ExpiresAt time.Time
}

type Queue struct {
	mu              sync.Mutex
	jobsByID        map[string]*Job
	pending         []*Job
	liveJobByIP     map[string]string
	dailyCountsByIP map[string]*ipEntry
	tombstones      map[string]tombstone
	scanService     ScanService
	pool            *scanner.BrowserPool
	logger          *slog.Logger
	requestTimeout  time.Duration
	maxQueueSize    int
	dailyLimit      int
	jobTTL          time.Duration
	resultTTL       time.Duration
	tombstoneTTL    time.Duration
	avgScanDuration time.Duration
	durationSamples int
	shuttingDown    bool
	now             func() time.Time
	idFunc          func() string
}

func New(cfg config.Config, scanService ScanService, pool *scanner.BrowserPool, logger *slog.Logger) *Queue {
	return &Queue{
		jobsByID:        map[string]*Job{},
		pending:         []*Job{},
		liveJobByIP:     map[string]string{},
		dailyCountsByIP: map[string]*ipEntry{},
		tombstones:      map[string]tombstone{},
		scanService:     scanService,
		pool:            pool,
		logger:          logger,
		requestTimeout:  cfg.RequestTimeout,
		maxQueueSize:    cfg.MaxQueueSize,
		dailyLimit:      cfg.DailyIPScanLimit,
		jobTTL:          cfg.JobTTL,
		resultTTL:       cfg.ResultTTL,
		tombstoneTTL:    defaultTombstoneTTL,
		avgScanDuration: initialAverageDuration,
		now: func() time.Time {
			return time.Now().UTC()
		},
		idFunc: newJobID,
	}
}

func (q *Queue) Submit(ctx context.Context, rawURL, clientIP string) (SubmitResult, error) {
	preparedTarget, err := q.scanService.PrepareTarget(ctx, rawURL)
	if err != nil {
		return SubmitResult{}, err
	}

	now := q.now()
	normalizedIP := normalizeClientIP(clientIP)

	q.mu.Lock()
	if q.shuttingDown {
		q.mu.Unlock()
		return SubmitResult{}, ErrQueueFull
	}

	if liveJobID, ok := q.liveJobByIP[normalizedIP]; ok {
		if job, exists := q.jobsByID[liveJobID]; exists && isLiveStatus(job.Status) {
			job.LastPolledAt = now
			jobsToStart := q.takeDispatchableLocked(now)
			snapshot := q.jobResponseLocked(job)
			q.mu.Unlock()
			q.startJobs(jobsToStart)

			if job.NormalizedURL == preparedTarget.NormalizedURL {
				return SubmitResult{Job: snapshot, Deduplicated: true}, nil
			}

			return SubmitResult{}, &ConflictError{Job: snapshot}
		}
		delete(q.liveJobByIP, normalizedIP)
	}

	entry := q.dailyEntryLocked(normalizedIP, now)
	if q.dailyLimit > 0 && entry.Count >= q.dailyLimit {
		retryAfter := time.Until(nextUTCMidnight(now))
		if retryAfter < time.Second {
			retryAfter = time.Second
		}
		q.mu.Unlock()
		return SubmitResult{}, &DailyLimitError{
			Limit:      q.dailyLimit,
			RetryAfter: retryAfter,
		}
	}

	if q.maxQueueSize > 0 && len(q.pending) >= q.maxQueueSize {
		q.mu.Unlock()
		return SubmitResult{}, ErrQueueFull
	}

	entry.Count++
	job := &Job{
		ID:            q.idFunc(),
		ClientIP:      normalizedIP,
		RawURL:        rawURL,
		NormalizedURL: preparedTarget.NormalizedURL,
		Hostname:      preparedTarget.Hostname,
		ResolvedIP:    preparedTarget.ResolvedIP,
		Status:        StatusQueued,
		CreatedAt:     now,
		LastPolledAt:  now,
	}
	q.jobsByID[job.ID] = job
	q.liveJobByIP[normalizedIP] = job.ID
	q.pending = append(q.pending, job)

	jobsToStart := q.takeDispatchableLocked(now)
	snapshot := q.jobResponseLocked(job)
	q.mu.Unlock()

	q.startJobs(jobsToStart)
	return SubmitResult{Job: snapshot}, nil
}

func (q *Queue) Get(_ context.Context, jobID string) (JobResponse, error) {
	now := q.now()

	q.mu.Lock()
	defer q.mu.Unlock()

	if job, ok := q.jobsByID[jobID]; ok {
		job.LastPolledAt = now
		return q.jobResponseLocked(job), nil
	}

	if tombstone, ok := q.tombstones[jobID]; ok && tombstone.ExpiresAt.After(now) {
		return tombstone.Job, &ExpiredError{Job: tombstone.Job}
	}

	return JobResponse{}, ErrJobNotFound
}

func (q *Queue) StartCleanup(ctx context.Context) {
	ticker := time.NewTicker(cleanupInterval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				q.cleanupExpired(q.now())
			}
		}
	}()
}

func (q *Queue) Shutdown(ctx context.Context) error {
	q.mu.Lock()
	q.shuttingDown = true
	q.mu.Unlock()

	return q.pool.Wait(ctx)
}

func (q *Queue) cleanupExpired(now time.Time) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for ip, entry := range q.dailyCountsByIP {
		if !entry.WindowEnd.After(now) {
			delete(q.dailyCountsByIP, ip)
		}
	}

	for jobID, tombstone := range q.tombstones {
		if !tombstone.ExpiresAt.After(now) {
			delete(q.tombstones, jobID)
		}
	}

	for jobID, job := range q.jobsByID {
		switch job.Status {
		case StatusQueued:
			if now.Sub(job.LastPolledAt) >= q.jobTTL {
				q.removePendingLocked(jobID)
				delete(q.jobsByID, jobID)
				if q.liveJobByIP[job.ClientIP] == jobID {
					delete(q.liveJobByIP, job.ClientIP)
				}
				q.addTombstoneLocked(job, expiredQueuedJobMessage, now)
			}
		case StatusCompleted, StatusFailed:
			if !job.CompletedAt.IsZero() && now.Sub(job.CompletedAt) >= q.resultTTL {
				delete(q.jobsByID, jobID)
				q.addTombstoneLocked(job, expiredResultMessage, now)
			}
		}
	}
}

func (q *Queue) dailyEntryLocked(clientIP string, now time.Time) *ipEntry {
	entry, ok := q.dailyCountsByIP[clientIP]
	if ok && entry.WindowEnd.After(now) {
		return entry
	}

	entry = &ipEntry{WindowEnd: nextUTCMidnight(now)}
	q.dailyCountsByIP[clientIP] = entry
	return entry
}

func (q *Queue) takeDispatchableLocked(now time.Time) []*Job {
	if q.shuttingDown {
		return nil
	}

	jobsToStart := make([]*Job, 0, len(q.pending))
	for len(q.pending) > 0 && q.pool.TryAcquire() {
		job := q.pending[0]
		q.pending = q.pending[1:]
		job.Status = StatusScanning
		job.Position = 0
		job.StartedAt = now
		jobsToStart = append(jobsToStart, job)
	}

	return jobsToStart
}

func (q *Queue) startJobs(jobs []*Job) {
	for _, job := range jobs {
		go q.runJob(job.ID)
	}
}

func (q *Queue) runJob(jobID string) {
	q.mu.Lock()
	job, ok := q.jobsByID[jobID]
	if !ok {
		q.mu.Unlock()
		q.pool.Release()
		return
	}

	target := scanner.PreparedTarget{
		RawURL:        job.RawURL,
		NormalizedURL: job.NormalizedURL,
		Hostname:      job.Hostname,
		ResolvedIP:    job.ResolvedIP,
	}
	q.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), q.requestTimeout)
	report, err := q.scanService.ScanPrepared(ctx, target)
	cancel()

	finishedAt := q.now()
	q.pool.Release()

	q.mu.Lock()
	job, ok = q.jobsByID[jobID]
	if !ok {
		jobsToStart := q.takeDispatchableLocked(finishedAt)
		q.mu.Unlock()
		q.startJobs(jobsToStart)
		return
	}

	if q.liveJobByIP[job.ClientIP] == job.ID {
		delete(q.liveJobByIP, job.ClientIP)
	}

	job.CompletedAt = finishedAt
	if err != nil {
		job.Status = StatusFailed
		job.PublicError = publicScanError(err)
		q.logger.Warn("scan_job_failed", "job_id", job.ID, "url", job.NormalizedURL, "error", err)
	} else {
		job.Status = StatusCompleted
		job.PublicError = ""
		reportCopy := report
		job.Report = &reportCopy
	}

	if !job.StartedAt.IsZero() {
		q.observeDurationLocked(finishedAt.Sub(job.StartedAt))
	}

	jobsToStart := q.takeDispatchableLocked(finishedAt)
	q.mu.Unlock()

	q.startJobs(jobsToStart)
}

func (q *Queue) observeDurationLocked(duration time.Duration) {
	if duration <= 0 {
		return
	}

	if q.durationSamples == 0 {
		q.avgScanDuration = duration
		q.durationSamples = 1
		return
	}

	if q.durationSamples < averageDurationWindow {
		total := int64(q.avgScanDuration) * int64(q.durationSamples)
		q.durationSamples++
		q.avgScanDuration = time.Duration((total + int64(duration)) / int64(q.durationSamples))
		return
	}

	q.avgScanDuration = time.Duration(
		((int64(q.avgScanDuration) * int64(averageDurationWindow-1)) + int64(duration)) / int64(averageDurationWindow),
	)
}

func (q *Queue) jobResponseLocked(job *Job) JobResponse {
	response := JobResponse{
		JobID:    job.ID,
		URL:      job.NormalizedURL,
		Status:   job.Status,
		Position: 0,
		Error:    job.PublicError,
		Report:   job.Report,
	}

	if job.Status == StatusQueued {
		jobsAhead := q.jobsAheadLocked(job.ID)
		response.Position = jobsAhead + 1
		response.EstimatedWaitSeconds = q.estimateWaitSecondsLocked(jobsAhead)
	} else {
		response.Position = 0
	}

	return response
}

func (q *Queue) jobsAheadLocked(jobID string) int {
	for index, pendingJob := range q.pending {
		if pendingJob.ID == jobID {
			return index
		}
	}

	return 0
}

func (q *Queue) estimateWaitSecondsLocked(jobsAhead int) int {
	concurrentLimit := q.pool.Limit()
	if concurrentLimit <= 0 {
		concurrentLimit = 1
	}

	activeCount := q.pool.ActiveCount()
	batches := int(math.Ceil(float64(jobsAhead+activeCount) / float64(concurrentLimit)))
	if batches < 1 {
		batches = 1
	}

	wait := time.Duration(batches) * q.avgScanDuration
	seconds := int(math.Ceil(wait.Seconds()))
	if seconds < 1 {
		return 1
	}

	return seconds
}

func (q *Queue) removePendingLocked(jobID string) {
	filtered := q.pending[:0]
	for _, job := range q.pending {
		if job.ID != jobID {
			filtered = append(filtered, job)
		}
	}
	q.pending = filtered
}

func (q *Queue) addTombstoneLocked(job *Job, message string, now time.Time) {
	q.tombstones[job.ID] = tombstone{
		Job: JobResponse{
			JobID:    job.ID,
			URL:      job.NormalizedURL,
			Status:   StatusExpired,
			Position: 0,
			Error:    message,
		},
		ExpiresAt: now.Add(q.tombstoneTTL),
	}
}

func normalizeClientIP(clientIP string) string {
	value := strings.TrimSpace(clientIP)
	if value == "" {
		return "unknown"
	}
	return value
}

func publicScanError(err error) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "El análisis tardó demasiado y fue cancelado. Inténtalo de nuevo."
	case errors.Is(err, context.Canceled):
		return "El análisis fue cancelado antes de completarse. Inténtalo de nuevo."
	default:
		return "El escaneo falló antes de completarse. Inténtalo de nuevo."
	}
}

func newJobID() string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("wl_%d", time.Now().UnixNano())
	}
	return "wl_" + hex.EncodeToString(buffer)
}

func isLiveStatus(status JobStatus) bool {
	return status == StatusQueued || status == StatusScanning
}

func nextUTCMidnight(now time.Time) time.Time {
	utcNow := now.UTC()
	return time.Date(utcNow.Year(), utcNow.Month(), utcNow.Day()+1, 0, 0, 0, 0, time.UTC)
}

func QueueRetryAfter() time.Duration {
	return defaultQueueRetryAfter
}
