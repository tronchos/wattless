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
	"github.com/tronchos/wattless/server/internal/insights"
	"github.com/tronchos/wattless/server/internal/scanner"
	"log/slog"
)

type fakeScanService struct {
	mu              sync.Mutex
	release         map[string]chan struct{}
	insightsRelease map[string]chan struct{}
	started         chan string
	insightsStarted chan string
	scanErr         map[string]error
	insightsErr     map[string]error
	reports         map[string]scanner.Report
	insightsResults map[string]insights.ProviderResult
	hasAI           bool
}

func newFakeScanService() *fakeScanService {
	return &fakeScanService{
		release:         make(map[string]chan struct{}),
		insightsRelease: make(map[string]chan struct{}),
		started:         make(chan string, 16),
		insightsStarted: make(chan string, 16),
		scanErr:         make(map[string]error),
		insightsErr:     make(map[string]error),
		reports:         make(map[string]scanner.Report),
		insightsResults: make(map[string]insights.ProviderResult),
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
		report = scanner.Report{
			URL:   target.NormalizedURL,
			Score: "A",
			Insights: insights.ScanInsights{
				Provider:         "rule_based",
				ExecutiveSummary: "Resumen base",
			},
		}
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

func (s *fakeScanService) GenerateInsights(ctx context.Context, report scanner.Report) (insights.ProviderResult, error) {
	s.insightsStarted <- report.URL

	s.mu.Lock()
	release := s.insightsRelease[report.URL]
	result, ok := s.insightsResults[report.URL]
	if !ok {
		result = insights.ProviderResult{
			Insights: insights.ScanInsights{
				Provider:         "gemini",
				ExecutiveSummary: "Resumen Gemini",
			},
		}
	}
	err := s.insightsErr[report.URL]
	s.mu.Unlock()

	if release != nil {
		select {
		case <-release:
		case <-ctx.Done():
			return insights.ProviderResult{}, ctx.Err()
		}
	}

	if err != nil {
		return insights.ProviderResult{}, err
	}

	return result, nil
}

func (s *fakeScanService) ApplyInsights(report *scanner.Report, result insights.ProviderResult) {
	if report == nil {
		return
	}

	report.Insights = result.Insights
}

func (s *fakeScanService) HasAIProvider() bool {
	return s.hasAI
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

func TestQueueMarksInsightsProcessingWithoutBlockingCompletedReport(t *testing.T) {
	service := newFakeScanService()
	service.hasAI = true
	service.insightsRelease["https://ai.test"] = make(chan struct{})
	service.insightsResults["https://ai.test"] = insights.ProviderResult{
		Insights: insights.ScanInsights{
			Provider:         "gemini",
			ExecutiveSummary: "Resumen enriquecido",
		},
	}

	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		LLMTimeout:          5 * time.Second,
		MaxQueueSize:        5,
		DailyIPScanLimit:    10,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	submitted, err := q.Submit(context.Background(), "https://ai.test", "203.0.113.10")
	if err != nil {
		t.Fatalf("expected submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://ai.test")
	waitForStatus(t, q, submitted.Job.JobID, StatusCompleted)
	waitForInsightsStatus(t, q, submitted.Job.JobID, InsightsStatusProcessing)

	job, err := q.Get(context.Background(), submitted.Job.JobID)
	if err != nil {
		t.Fatalf("expected completed job, got %v", err)
	}
	if job.Report == nil || job.Report.Insights.Provider != "rule_based" {
		t.Fatalf("expected base report before async insights, got %#v", job.Report)
	}

	insightsResponse, err := q.GetInsights(context.Background(), submitted.Job.JobID)
	if err != nil {
		t.Fatalf("expected insights status, got %v", err)
	}
	if insightsResponse.Status != InsightsStatusProcessing {
		t.Fatalf("expected processing insights, got %#v", insightsResponse)
	}

	close(service.insightsRelease["https://ai.test"])
	waitForInsightsStatus(t, q, submitted.Job.JobID, InsightsStatusReady)

	ready, err := q.GetInsights(context.Background(), submitted.Job.JobID)
	if err != nil {
		t.Fatalf("expected ready insights, got %v", err)
	}
	if ready.Status != InsightsStatusReady {
		t.Fatalf("expected ready insights status, got %#v", ready)
	}
	if ready.Insights == nil || ready.Insights.Provider != "gemini" {
		t.Fatalf("expected gemini insights payload, got %#v", ready.Insights)
	}
}

func TestQueueMarksInsightsFailedWhenAsyncProviderFails(t *testing.T) {
	service := newFakeScanService()
	service.hasAI = true
	service.insightsErr["https://boom.test"] = errors.New("boom")

	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		LLMTimeout:          5 * time.Second,
		MaxQueueSize:        5,
		DailyIPScanLimit:    10,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	submitted, err := q.Submit(context.Background(), "https://boom.test", "203.0.113.20")
	if err != nil {
		t.Fatalf("expected submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://boom.test")
	waitForStatus(t, q, submitted.Job.JobID, StatusCompleted)
	waitForInsightsStatus(t, q, submitted.Job.JobID, InsightsStatusFailed)

	insightsResponse, err := q.GetInsights(context.Background(), submitted.Job.JobID)
	if err != nil {
		t.Fatalf("expected failed insights status, got %v", err)
	}
	if insightsResponse.Status != InsightsStatusFailed {
		t.Fatalf("expected failed insights status, got %#v", insightsResponse)
	}
}

func TestQueueInsightsUnavailableWithoutAIProvider(t *testing.T) {
	service := newFakeScanService()

	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		LLMTimeout:          5 * time.Second,
		MaxQueueSize:        5,
		DailyIPScanLimit:    10,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	submitted, err := q.Submit(context.Background(), "https://rule-based.test", "203.0.113.30")
	if err != nil {
		t.Fatalf("expected submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://rule-based.test")
	waitForStatus(t, q, submitted.Job.JobID, StatusCompleted)

	_, err = q.GetInsights(context.Background(), submitted.Job.JobID)
	if !errors.Is(err, ErrInsightsUnavailable) {
		t.Fatalf("expected insights unavailable, got %v", err)
	}
}

func TestQueueGetReturnsDeepClonedReportSnapshot(t *testing.T) {
	service := newFakeScanService()
	service.reports["https://clone.test"] = scanner.Report{
		URL: "https://clone.test",
		Insights: insights.ScanInsights{
			Provider:         "rule_based",
			ExecutiveSummary: "Resumen base",
			TopActions: []insights.TopAction{
				{
					ID:                 "act-1",
					Evidence:           []string{"evidence-1"},
					RelatedResourceIDs: []string{"asset-1"},
					VisibleRelatedResourceIDs: []string{
						"asset-1",
					},
					RecommendedFix: &insights.RecommendedFix{
						Summary: "Fix",
						Changes: []string{"change-1"},
					},
				},
			},
		},
		VampireElements: []scanner.ResourceSummary{
			{
				ID: "asset-1",
				AssetInsight: scanner.AssetInsight{
					Evidence: []string{"asset-evidence"},
					RecommendedFix: &scanner.FixSuggestion{
						Summary: "Fix",
						Changes: []string{"change-1"},
					},
				},
				BoundingBox: &scanner.BoundingBox{X: 1, Y: 2, Width: 3, Height: 4},
			},
		},
		Analysis: scanner.Analysis{
			Findings: []scanner.AnalysisFinding{
				{
					ID:                 "finding-1",
					Evidence:           []string{"finding-evidence"},
					RelatedResourceIDs: []string{"asset-1"},
				},
			},
			ResourceGroups: []scanner.ResourceGroup{
				{
					ID:                 "group-1",
					RelatedResourceIDs: []string{"asset-1"},
				},
			},
		},
		Methodology: scanner.Methodology{
			Assumptions: []string{"assumption-1"},
		},
		Warnings: []string{"warning-1"},
	}

	q := New(config.Config{
		RequestTimeout:      5 * time.Second,
		LLMTimeout:          5 * time.Second,
		MaxQueueSize:        5,
		DailyIPScanLimit:    10,
		JobTTL:              5 * time.Minute,
		ResultTTL:           3 * time.Minute,
		ConcurrentScanLimit: 1,
	}, service, scanner.NewBrowserPool(1), nilLogger())

	submitted, err := q.Submit(context.Background(), "https://clone.test", "203.0.113.40")
	if err != nil {
		t.Fatalf("expected submit to succeed: %v", err)
	}
	waitForStarted(t, service.started, "https://clone.test")
	waitForStatus(t, q, submitted.Job.JobID, StatusCompleted)

	first, err := q.Get(context.Background(), submitted.Job.JobID)
	if err != nil {
		t.Fatalf("expected cloned report snapshot, got %v", err)
	}
	if first.Report == nil {
		t.Fatal("expected report snapshot")
	}

	first.Report.Insights.TopActions[0].Evidence[0] = "mutated"
	first.Report.Insights.TopActions[0].RecommendedFix.Changes[0] = "mutated"
	first.Report.VampireElements[0].AssetInsight.Evidence[0] = "mutated"
	first.Report.VampireElements[0].AssetInsight.RecommendedFix.Changes[0] = "mutated"
	first.Report.VampireElements[0].BoundingBox.X = 99
	first.Report.Analysis.Findings[0].Evidence[0] = "mutated"
	first.Report.Analysis.ResourceGroups[0].RelatedResourceIDs[0] = "mutated"
	first.Report.Methodology.Assumptions[0] = "mutated"
	first.Report.Warnings[0] = "mutated"

	second, err := q.Get(context.Background(), submitted.Job.JobID)
	if err != nil {
		t.Fatalf("expected second report snapshot, got %v", err)
	}
	if second.Report == nil {
		t.Fatal("expected report snapshot")
	}
	if second.Report.Insights.TopActions[0].Evidence[0] != "evidence-1" {
		t.Fatalf("expected deep clone for top action evidence, got %#v", second.Report.Insights.TopActions[0].Evidence)
	}
	if second.Report.VampireElements[0].AssetInsight.Evidence[0] != "asset-evidence" {
		t.Fatalf("expected deep clone for asset evidence, got %#v", second.Report.VampireElements[0].AssetInsight.Evidence)
	}
	if second.Report.VampireElements[0].BoundingBox.X != 1 {
		t.Fatalf("expected deep clone for bounding box, got %#v", second.Report.VampireElements[0].BoundingBox)
	}
	if second.Report.Methodology.Assumptions[0] != "assumption-1" {
		t.Fatalf("expected deep clone for methodology assumptions, got %#v", second.Report.Methodology.Assumptions)
	}
	if second.Report.Warnings[0] != "warning-1" {
		t.Fatalf("expected deep clone for warnings, got %#v", second.Report.Warnings)
	}
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

func waitForInsightsStatus(t *testing.T, q *Queue, jobID string, expected InsightsStatus) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		q.mu.Lock()
		job, ok := q.jobsByID[jobID]
		current := InsightsStatusNone
		if ok {
			current = job.InsightsStatus
		}
		q.mu.Unlock()

		if ok && current == expected {
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	q.mu.Lock()
	job, ok := q.jobsByID[jobID]
	q.mu.Unlock()
	if !ok {
		t.Fatalf("timed out waiting for insights status %q, job %s not found", expected, jobID)
	}

	t.Fatalf(
		"timed out waiting for job %s to reach insights %q, last status %q",
		jobID,
		expected,
		job.InsightsStatus,
	)
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

func TestCloneReportPreservesEmptySlices(t *testing.T) {
	report := &scanner.Report{
		BreakdownByType:  []scanner.ResourceBreakdown{},
		BreakdownByParty: []scanner.ResourceBreakdown{},
		Insights: insights.ScanInsights{
			TopActions: []insights.TopAction{
				{
					ID:                        "act-1",
					Evidence:                  []string{},
					RelatedResourceIDs:        []string{},
					VisibleRelatedResourceIDs: []string{},
					RecommendedFix: &scanner.FixSuggestion{
						Changes: []string{},
					},
				},
			},
		},
		VampireElements: []scanner.ResourceSummary{},
		Analysis: scanner.Analysis{
			Findings: []scanner.AnalysisFinding{
				{
					ID:                 "finding-1",
					Evidence:           []string{},
					RelatedResourceIDs: []string{},
				},
			},
			ResourceGroups: []scanner.ResourceGroup{
				{
					ID:                 "group-1",
					RelatedResourceIDs: []string{},
				},
			},
		},
		Screenshot: scanner.Screenshot{
			Tiles: []scanner.ScreenshotTile{},
		},
		Methodology: scanner.Methodology{
			Assumptions: []string{},
		},
		Warnings: []string{},
	}

	cloned := cloneReport(report)
	if cloned == nil {
		t.Fatal("expected cloned report")
	}
	if cloned.BreakdownByType == nil {
		t.Fatal("expected empty breakdown_by_type slice to stay non-nil")
	}
	if cloned.BreakdownByParty == nil {
		t.Fatal("expected empty breakdown_by_party slice to stay non-nil")
	}
	if cloned.Insights.TopActions[0].Evidence == nil {
		t.Fatal("expected empty top action evidence slice to stay non-nil")
	}
	if cloned.Insights.TopActions[0].RelatedResourceIDs == nil {
		t.Fatal("expected empty related ids slice to stay non-nil")
	}
	if cloned.Insights.TopActions[0].VisibleRelatedResourceIDs == nil {
		t.Fatal("expected empty visible related ids slice to stay non-nil")
	}
	if cloned.Insights.TopActions[0].RecommendedFix == nil || cloned.Insights.TopActions[0].RecommendedFix.Changes == nil {
		t.Fatalf("expected empty recommended fix changes slice to stay non-nil, got %#v", cloned.Insights.TopActions[0].RecommendedFix)
	}
	if cloned.VampireElements == nil {
		t.Fatal("expected empty vampire elements slice to stay non-nil")
	}
	if cloned.Analysis.Findings[0].Evidence == nil {
		t.Fatal("expected empty finding evidence slice to stay non-nil")
	}
	if cloned.Analysis.Findings[0].RelatedResourceIDs == nil {
		t.Fatal("expected empty finding related ids slice to stay non-nil")
	}
	if cloned.Analysis.ResourceGroups[0].RelatedResourceIDs == nil {
		t.Fatal("expected empty resource group related ids slice to stay non-nil")
	}
	if cloned.Screenshot.Tiles == nil {
		t.Fatal("expected empty screenshot tiles slice to stay non-nil")
	}
	if cloned.Methodology.Assumptions == nil {
		t.Fatal("expected empty methodology assumptions slice to stay non-nil")
	}
	if cloned.Warnings == nil {
		t.Fatal("expected empty warnings slice to stay non-nil")
	}
}
