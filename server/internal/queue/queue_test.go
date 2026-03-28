package queue

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/scanner"
	"log/slog"
)

type fakeScanService struct {
	mu      sync.Mutex
	release map[string]chan struct{}
	started chan string
	scanErr map[string]error
	reports map[string]scanner.Report
}

func newFakeScanService() *fakeScanService {
	return &fakeScanService{
		release: make(map[string]chan struct{}),
		started: make(chan string, 16),
		scanErr: make(map[string]error),
		reports: make(map[string]scanner.Report),
	}
}

func (s *fakeScanService) PrepareTarget(ctx context.Context, rawURL string) (scanner.PreparedTarget, error) {
	return scanner.PreparedTarget{
		RawURL:        rawURL,
		NormalizedURL: rawURL,
		Hostname:      rawURL,
		ResolvedIP:    "1.1.1.1",
	}, nil
}

func (s *fakeScanService) ScanPrepared(ctx context.Context, target scanner.PreparedTarget) (scanner.Report, error) {
	s.started <- target.NormalizedURL

	s.mu.Lock()
	release := s.release[target.NormalizedURL]
	report, ok := s.reports[target.NormalizedURL]
	if !ok {
		report = scanner.Report{URL: target.NormalizedURL, Score: "A"}
	}
	err := s.scanErr[target.NormalizedURL]
	s.mu.Unlock()

	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return scanner.Report{}, ctx.Err()
		}
	}

	if err != nil {
		return scanner.Report{}, err
	}

	return report, nil
}

func TestQueueDispatchesFIFOAndUpdatesPosition(t *testing.T) {
	service := newFakeScanService()
	service.release["https://one.test"] = make(chan struct{})
	service.release["https://two.test"] = make(chan struct{})

	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		MaxQueueSize:        5,
		DailyIPScanLimit:    10,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	first, err := q.Submit(context.Background(), "https://one.test", "203.0.113.1")
	if err != nil {
		t.Fatalf("expected first submit to succeed: %v", err)
	}
	if first.Job.Status != StatusScanning {
		t.Fatalf("expected first job to start scanning, got %q", first.Job.Status)
	}

	<-service.started

	second, err := q.Submit(context.Background(), "https://two.test", "203.0.113.2")
	if err != nil {
		t.Fatalf("expected second submit to succeed: %v", err)
	}
	if second.Job.Status != StatusQueued || second.Job.Position != 1 {
		t.Fatalf("expected second job to be queued at position 1, got %#v", second.Job)
	}

	close(service.release["https://one.test"])
	waitForStarted(t, service.started, "https://two.test")
	waitForStatus(t, q, second.Job.JobID, StatusScanning)

	close(service.release["https://two.test"])
	waitForStatus(t, q, second.Job.JobID, StatusCompleted)
}

func TestQueueDeduplicatesSameIPAndConflictsOnDifferentURL(t *testing.T) {
	service := newFakeScanService()
	service.release["https://same.test"] = make(chan struct{})

	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		MaxQueueSize:        5,
		DailyIPScanLimit:    10,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	first, err := q.Submit(context.Background(), "https://same.test", "198.51.100.10")
	if err != nil {
		t.Fatalf("expected first submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://same.test")

	deduped, err := q.Submit(context.Background(), "https://same.test", "198.51.100.10")
	if err != nil {
		t.Fatalf("expected deduplicated submit to succeed: %v", err)
	}
	if !deduped.Deduplicated || deduped.Job.JobID != first.Job.JobID {
		t.Fatalf("expected deduplicated job response, got %#v", deduped)
	}

	_, err = q.Submit(context.Background(), "https://other.test", "198.51.100.10")
	var conflictErr *ConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("expected conflict error, got %v", err)
	}
	if conflictErr.Job.JobID != first.Job.JobID {
		t.Fatalf("expected conflict to point at first job, got %#v", conflictErr.Job)
	}

	close(service.release["https://same.test"])
	waitForStatus(t, q, first.Job.JobID, StatusCompleted)
}

func TestQueueAppliesDailyLimitOnlyToAcceptedJobs(t *testing.T) {
	service := newFakeScanService()
	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		MaxQueueSize:        5,
		DailyIPScanLimit:    1,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	first, err := q.Submit(context.Background(), "https://one.test", "192.0.2.10")
	if err != nil {
		t.Fatalf("expected first submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://one.test")
	waitForStatus(t, q, first.Job.JobID, StatusCompleted)

	_, err = q.Submit(context.Background(), "https://two.test", "192.0.2.10")
	var limitErr *DailyLimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected daily limit error, got %v", err)
	}
	if limitErr.Limit != 1 || limitErr.RetryAfter <= 0 {
		t.Fatalf("unexpected daily limit payload: %#v", limitErr)
	}
}

func TestQueueRejectsWhenPendingQueueIsFull(t *testing.T) {
	service := newFakeScanService()
	service.release["https://one.test"] = make(chan struct{})
	service.release["https://two.test"] = make(chan struct{})

	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		MaxQueueSize:        1,
		DailyIPScanLimit:    10,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	_, _ = q.Submit(context.Background(), "https://one.test", "192.0.2.1")
	waitForStarted(t, service.started, "https://one.test")
	_, _ = q.Submit(context.Background(), "https://two.test", "192.0.2.2")

	_, err := q.Submit(context.Background(), "https://three.test", "192.0.2.3")
	if !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected queue full error, got %v", err)
	}

	close(service.release["https://one.test"])
	waitForStarted(t, service.started, "https://two.test")
	close(service.release["https://two.test"])
}

func TestQueuePrioritizesDailyLimitOverQueueFull(t *testing.T) {
	service := newFakeScanService()
	service.release["https://busy.test"] = make(chan struct{})
	service.release["https://queued.test"] = make(chan struct{})

	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		MaxQueueSize:        1,
		DailyIPScanLimit:    1,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	first, err := q.Submit(context.Background(), "https://limit.test", "192.0.2.10")
	if err != nil {
		t.Fatalf("expected first submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://limit.test")
	waitForStatus(t, q, first.Job.JobID, StatusCompleted)

	_, err = q.Submit(context.Background(), "https://busy.test", "192.0.2.20")
	if err != nil {
		t.Fatalf("expected busy submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://busy.test")

	_, err = q.Submit(context.Background(), "https://queued.test", "192.0.2.30")
	if err != nil {
		t.Fatalf("expected queued submit to succeed: %v", err)
	}

	_, err = q.Submit(context.Background(), "https://limit-again.test", "192.0.2.10")
	var limitErr *DailyLimitError
	if !errors.As(err, &limitErr) {
		t.Fatalf("expected daily limit error, got %v", err)
	}

	close(service.release["https://busy.test"])
	waitForStarted(t, service.started, "https://queued.test")
	close(service.release["https://queued.test"])
}

func TestQueueExpiresQueuedJobsAfterTTL(t *testing.T) {
	service := newFakeScanService()
	service.release["https://one.test"] = make(chan struct{})

	currentTime := time.Date(2026, time.March, 28, 12, 0, 0, 0, time.UTC)
	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		MaxQueueSize:        5,
		DailyIPScanLimit:    10,
		JobTTL:              2 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())
	q.now = func() time.Time { return currentTime }
	q.idFunc = sequentialID()

	first, err := q.Submit(context.Background(), "https://one.test", "198.51.100.1")
	if err != nil {
		t.Fatalf("expected first submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://one.test")

	second, err := q.Submit(context.Background(), "https://two.test", "198.51.100.2")
	if err != nil {
		t.Fatalf("expected second submit to succeed: %v", err)
	}
	if second.Job.Status != StatusQueued {
		t.Fatalf("expected second job to stay queued, got %#v", second.Job)
	}

	currentTime = currentTime.Add(3 * time.Minute)
	q.cleanupExpired(currentTime)

	_, err = q.Get(context.Background(), second.Job.JobID)
	var expiredErr *ExpiredError
	if !errors.As(err, &expiredErr) {
		t.Fatalf("expected expired error, got %v", err)
	}
	if expiredErr.Job.Status != StatusExpired {
		t.Fatalf("expected expired job snapshot, got %#v", expiredErr.Job)
	}

	close(service.release["https://one.test"])
	waitForStatus(t, q, first.Job.JobID, StatusCompleted)
}

func nilLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func waitForStarted(t *testing.T, started <-chan string, expected string) {
	t.Helper()

	select {
	case value := <-started:
		if value != expected {
			t.Fatalf("expected scan %q to start, got %q", expected, value)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %q to start", expected)
	}
}

func waitForStatus(t *testing.T, q *Queue, jobID string, expected JobStatus) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := q.Get(context.Background(), jobID)
		if err == nil && job.Status == expected {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	job, err := q.Get(context.Background(), jobID)
	t.Fatalf("timed out waiting for job %s to reach %q, last state %#v, err=%v", jobID, expected, job, err)
}

func sequentialID() func() string {
	var mu sync.Mutex
	var nextID int

	return func() string {
		mu.Lock()
		defer mu.Unlock()
		nextID++
		return fmt.Sprintf("wl_test_%d", nextID)
	}
}
