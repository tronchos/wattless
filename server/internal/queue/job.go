package queue

import (
	"errors"
	"fmt"
	"time"

	"github.com/tronchos/wattless/server/internal/scanner"
)

type JobStatus string

const (
	StatusQueued    JobStatus = "queued"
	StatusScanning  JobStatus = "scanning"
	StatusCompleted JobStatus = "completed"
	StatusFailed    JobStatus = "failed"
	StatusExpired   JobStatus = "expired"
)

type Job struct {
	ID            string
	ClientIP      string
	RawURL        string
	NormalizedURL string
	Hostname      string
	ResolvedIP    string
	Status        JobStatus
	Position      int
	Report        *scanner.Report
	PublicError   string
	CreatedAt     time.Time
	StartedAt     time.Time
	CompletedAt   time.Time
	LastPolledAt  time.Time
}

type JobResponse struct {
	JobID                string          `json:"job_id"`
	URL                  string          `json:"url"`
	Status               JobStatus       `json:"status"`
	Position             int             `json:"position"`
	EstimatedWaitSeconds int             `json:"estimated_wait_seconds,omitempty"`
	Report               *scanner.Report `json:"report,omitempty"`
	Error                string          `json:"error,omitempty"`
}

type SubmitResult struct {
	Job          JobResponse
	Deduplicated bool
}

var (
	ErrQueueFull          = errors.New("scan queue is full")
	ErrDailyLimitExceeded = errors.New("daily scan limit exceeded")
	ErrJobConflict        = errors.New("job conflict")
	ErrJobExpired         = errors.New("job expired")
	ErrJobNotFound        = errors.New("job not found")
)

type DailyLimitError struct {
	Limit      int
	RetryAfter time.Duration
}

func (e *DailyLimitError) Error() string {
	return fmt.Sprintf("daily scan limit %d exceeded", e.Limit)
}

func (e *DailyLimitError) Unwrap() error {
	return ErrDailyLimitExceeded
}

type ConflictError struct {
	Job JobResponse
}

func (e *ConflictError) Error() string {
	return "there is already a live job for this client IP"
}

func (e *ConflictError) Unwrap() error {
	return ErrJobConflict
}

type ExpiredError struct {
	Job JobResponse
}

func (e *ExpiredError) Error() string {
	return "job expired"
}

func (e *ExpiredError) Unwrap() error {
	return ErrJobExpired
}
