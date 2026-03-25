package scanner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/hosting"
	"github.com/tronchos/wattless/server/internal/insights"
	"github.com/tronchos/wattless/server/pkg/co2"
	"github.com/tronchos/wattless/server/pkg/score"
	"github.com/tronchos/wattless/server/pkg/urlutil"
)

type HostingChecker interface {
	Check(context.Context, string) (hosting.Result, error)
}

type Service struct {
	cfg            config.Config
	hostingChecker HostingChecker
	insights       insights.Provider
	logger         *slog.Logger
}

func NewService(cfg config.Config, hostingChecker HostingChecker, insightsProvider insights.Provider, logger *slog.Logger) *Service {
	return &Service{
		cfg:            cfg,
		hostingChecker: hostingChecker,
		insights:       insightsProvider,
		logger:         logger,
	}
}

func (s *Service) Scan(ctx context.Context, rawURL string) (Report, error) {
	normalizedURL, hostname, err := urlutil.Normalize(rawURL)
	if err != nil {
		return Report{}, err
	}

	resources, perf, screenshot, warnings, err := s.runBrowserScan(ctx, normalizedURL)
	if err != nil {
		return Report{}, err
	}
	if warnings == nil {
		warnings = []string{}
	}

	hostingResult, hostingWarnings := s.resolveHosting(ctx, hostname)
	warnings = append(warnings, hostingWarnings...)

	totalBytes := int64(0)
	for _, resource := range resources {
		totalBytes += resource.Bytes
	}

	top, rankingWarnings := rankVampireResources(resources, totalBytes)
	warnings = append(warnings, rankingWarnings...)

	vampires := make([]ResourceSummary, 0, len(top))
	potentialSavings := int64(0)
	for _, resource := range resources {
		potentialSavings += estimateSavingsBytes(resource.Type, resource.Bytes)
	}
	visualMapped := 0
	for _, resource := range top {
		savings := estimateSavingsBytes(resource.Type, resource.Bytes)
		if resource.BoundingBox != nil {
			visualMapped++
		}
		vampires = append(vampires, ResourceSummary{
			ID:                    resource.ID,
			URL:                   resource.URL,
			Type:                  resource.Type,
			MIMEType:              resource.MIMEType,
			Hostname:              resource.Hostname,
			Party:                 resource.Party,
			StatusCode:            resource.StatusCode,
			Bytes:                 resource.Bytes,
			Failed:                resource.Failed,
			FailureReason:         resource.FailureReason,
			TransferShare:         shareOf(resource.Bytes, totalBytes),
			EstimatedSavingsBytes: savings,
			Recommendation: s.insights.SuggestResource(insights.ResourceContext{
				ID:                    resource.ID,
				URL:                   resource.URL,
				Type:                  resource.Type,
				MIMEType:              resource.MIMEType,
				Bytes:                 resource.Bytes,
				StatusCode:            resource.StatusCode,
				Failed:                resource.Failed,
				FailureReason:         resource.FailureReason,
				TransferShare:         shareOf(resource.Bytes, totalBytes),
				EstimatedSavingsBytes: savings,
			}),
			BoundingBox: resource.BoundingBox,
		})
	}

	grams := co2.FromBytes(totalBytes)
	breakdownByType, breakdownByParty := buildBreakdowns(resources, totalBytes)
	summary := buildSummary(resources, totalBytes, potentialSavings, visualMapped)
	finalScore := score.FromCO2(grams)

	report := Report{
		URL:                   normalizedURL,
		Score:                 finalScore,
		TotalBytesTransferred: totalBytes,
		CO2GramsPerVisit:      grams,
		HostingIsGreen:        hostingResult.IsGreen,
		HostingVerdict:        string(hostingResult.Verdict),
		HostedBy:              hostingResult.HostedBy,
		Summary:               summary,
		BreakdownByType:       breakdownByType,
		BreakdownByParty:      breakdownByParty,
		VampireElements:       vampires,
		Performance:           perf,
		Screenshot:            screenshot,
		Warnings:              warnings,
	}

	insightReport, err := s.insights.SummarizeReport(ctx, insights.ReportContext{
		URL:                   report.URL,
		Score:                 report.Score,
		TotalBytesTransferred: report.TotalBytesTransferred,
		CO2GramsPerVisit:      report.CO2GramsPerVisit,
		HostingIsGreen:        report.HostingIsGreen,
		HostingVerdict:        report.HostingVerdict,
		HostedBy:              report.HostedBy,
		Performance: insights.PerformanceContext{
			LoadMS:             report.Performance.LoadMS,
			DOMContentLoadedMS: report.Performance.DOMContentLoadedMS,
			ScriptDurationMS:   report.Performance.ScriptDurationMS,
			LCPMS:              report.Performance.LCPMS,
			FCPMS:              report.Performance.FCPMS,
		},
		Summary: insights.SummaryContext{
			TotalRequests:         report.Summary.TotalRequests,
			SuccessfulRequests:    report.Summary.SuccessfulRequests,
			FailedRequests:        report.Summary.FailedRequests,
			FirstPartyBytes:       report.Summary.FirstPartyBytes,
			ThirdPartyBytes:       report.Summary.ThirdPartyBytes,
			PotentialSavingsBytes: report.Summary.PotentialSavingsBytes,
			VisualMappedVampires:  report.Summary.VisualMappedVampires,
		},
		TopResources: makeInsightResources(vampires),
	})
	if err != nil {
		s.logger.Warn("report_insights_failed", "url", normalizedURL, "error", err)
		report.Warnings = append(report.Warnings, "La capa de IA no pudo enriquecer el informe; se usaron recomendaciones de respaldo.")
	} else {
		report.Insights = insightReport
	}

	return report, nil
}

func (s *Service) RefactorCode(ctx context.Context, request insights.RefactorRequest) (insights.RefactorResult, error) {
	request.Code = strings.TrimSpace(request.Code)
	request.Framework = strings.TrimSpace(request.Framework)
	request.Language = strings.TrimSpace(request.Language)
	if request.Code == "" {
		return insights.RefactorResult{}, fmt.Errorf("code is required")
	}
	if request.Framework == "" {
		request.Framework = "next"
	}
	if request.Language == "" {
		request.Language = "tsx"
	}
	return s.insights.RefactorCode(ctx, request)
}

type enrichedResource struct {
	ID            string
	URL           string
	Type          string
	MIMEType      string
	Hostname      string
	Party         string
	StatusCode    int
	Bytes         int64
	Failed        bool
	FailureReason string
	BoundingBox   *BoundingBox
}

func (s *Service) resolveHosting(ctx context.Context, hostname string) (hosting.Result, []string) {
	result, err := s.hostingChecker.Check(ctx, hostname)
	if err != nil {
		s.logger.Warn("greencheck_failed", "hostname", hostname, "error", err)
		return hosting.Result{Verdict: hosting.VerdictUnknown}, []string{"Green hosting check failed; returning unknown verdict."}
	}
	return result, nil
}

func (s *Service) runBrowserScan(ctx context.Context, targetURL string) ([]enrichedResource, PerformanceMetrics, Screenshot, []string, error) {
	browserURL, cleanup, err := s.launchBrowser()
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}
	defer cleanup()

	browser := rod.New().ControlURL(browserURL)
	if err := browser.Connect(); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}
	defer func() { _ = browser.Close() }()

	deadlineCtx, cancel := context.WithTimeout(ctx, s.cfg.RequestTimeout)
	defer cancel()

	browser = browser.Context(deadlineCtx)
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}
	defer func() { _ = page.Close() }()

	if err := (proto.EmulationSetDeviceMetricsOverride{
		Width:             s.cfg.ViewportWidth,
		Height:            s.cfg.ViewportHeight,
		DeviceScaleFactor: 1,
		Mobile:            false,
	}).Call(page); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	if err := (proto.NetworkEnable{}).Call(page); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	removePerfObserver, err := page.EvalOnNewDocument(performanceObserverScript())
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}
	defer func() { _ = removePerfObserver() }()

	resources := map[string]*rawResource{}
	mu := sync.Mutex{}
	pageHostname := resourceHostname(targetURL)

	go page.EachEvent(func(event *proto.NetworkResponseReceived) {
		mu.Lock()
		defer mu.Unlock()

		key := string(event.RequestID)
		entry, ok := resources[key]
		if !ok {
			entry = &rawResource{RequestID: key}
			resources[key] = entry
		}
		entry.URL = event.Response.URL
		entry.MIMEType = event.Response.MIMEType
		entry.StatusCode = event.Response.Status
		entry.Type = normalizeType(string(event.Type), event.Response.MIMEType, event.Response.URL)
	}, func(event *proto.NetworkRequestWillBeSent) {
		mu.Lock()
		defer mu.Unlock()

		key := string(event.RequestID)
		entry, ok := resources[key]
		if !ok {
			entry = &rawResource{RequestID: key}
			resources[key] = entry
		}
		if event.Request != nil {
			entry.URL = event.Request.URL
		}
		if entry.Type == "" {
			entry.Type = normalizeType(string(event.Type), entry.MIMEType, entry.URL)
		}
	}, func(event *proto.NetworkLoadingFinished) {
		mu.Lock()
		defer mu.Unlock()

		key := string(event.RequestID)
		entry, ok := resources[key]
		if !ok {
			entry = &rawResource{RequestID: key}
			resources[key] = entry
		}
		entry.Bytes = int64(event.EncodedDataLength)
	}, func(event *proto.NetworkLoadingFailed) {
		mu.Lock()
		defer mu.Unlock()

		key := string(event.RequestID)
		entry, ok := resources[key]
		if !ok {
			entry = &rawResource{RequestID: key}
			resources[key] = entry
		}
		entry.Failed = true
		entry.FailureReason = event.ErrorText
		if entry.Type == "" {
			entry.Type = normalizeType(string(event.Type), entry.MIMEType, entry.URL)
		}
	})()

	page = page.Timeout(s.cfg.NavigationTimeout)
	if err := page.Navigate(targetURL); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}
	if err := page.WaitLoad(); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}
	select {
	case <-deadlineCtx.Done():
		return nil, PerformanceMetrics{}, Screenshot{}, nil, deadlineCtx.Err()
	case <-time.After(s.cfg.NetworkIdleWait):
	}

	performanceMetrics, err := capturePerformance(page)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	elements, err := collectElementBoxes(page)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	screenshotBytes, err := page.Screenshot(false, &proto.PageCaptureScreenshot{
		Format:  proto.PageCaptureScreenshotFormatWebp,
		Quality: intPtr(85),
	})
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	screenshot := Screenshot{
		MimeType:   "image/webp",
		Width:      s.cfg.ViewportWidth,
		Height:     s.cfg.ViewportHeight,
		DataBase64: base64.StdEncoding.EncodeToString(screenshotBytes),
	}

	mu.Lock()
	defer mu.Unlock()

	enriched := make([]enrichedResource, 0, len(resources))
	for _, resource := range resources {
		if resource.URL == "" {
			continue
		}
		if resource.Bytes <= 0 && !resource.Failed && resource.StatusCode < 400 {
			continue
		}
		enriched = append(enriched, enrichedResource{
			ID:            resource.RequestID,
			URL:           resource.URL,
			Type:          resource.Type,
			MIMEType:      resource.MIMEType,
			Hostname:      resourceHostname(resource.URL),
			Party:         classifyParty(pageHostname, resourceHostname(resource.URL)),
			StatusCode:    resource.StatusCode,
			Bytes:         resource.Bytes,
			Failed:        resource.Failed,
			FailureReason: resource.FailureReason,
			BoundingBox:   matchBoundingBox(resource.URL, elements),
		})
	}

	return enriched, performanceMetrics, screenshot, []string{}, nil
}

func (s *Service) launchBrowser() (string, func(), error) {
	instance := launcher.New().
		Headless(true).
		Leakless(false).
		Set("disable-gpu").
		Set("no-sandbox").
		Set("disable-dev-shm-usage")

	if s.cfg.BrowserBin != "" {
		instance = instance.Bin(s.cfg.BrowserBin)
	}

	url, err := instance.Launch()
	if err != nil {
		return "", nil, fmt.Errorf("failed to launch Chromium: %w. Set BROWSER_BIN to a local Chromium/Chrome binary or run via docker compose", err)
	}

	return url, instance.Kill, nil
}

func capturePerformance(page *rod.Page) (PerformanceMetrics, error) {
	result, err := page.Evaluate(rod.Eval(`() => {
		const buffered = window.__wattlessMetrics || {};
		const [navigation] = performance.getEntriesByType("navigation");
		const paints = performance.getEntriesByType("paint");
		const fcp = paints.find((entry) => entry.name === "first-contentful-paint");
		const lcpEntries = performance.getEntriesByType("largest-contentful-paint");
		const lastLCP = lcpEntries.length > 0 ? lcpEntries[lcpEntries.length - 1] : null;
		return JSON.stringify({
			load_ms: navigation ? Math.round(navigation.loadEventEnd) : 0,
			dom_content_loaded_ms: navigation ? Math.round(navigation.domContentLoadedEventEnd) : 0,
			script_duration_ms: Math.round(
				performance
					.getEntriesByType("resource")
					.filter((entry) => entry.initiatorType === "script")
					.reduce((acc, entry) => acc + (entry.duration || 0), 0)
			),
			lcp_ms: Math.round(buffered.lcp_ms || (lastLCP ? lastLCP.startTime || 0 : 0)),
			fcp_ms: Math.round(buffered.fcp_ms || (fcp ? fcp.startTime || 0 : 0))
		});
	}`))
	if err != nil {
		return PerformanceMetrics{}, err
	}

	var perf PerformanceMetrics
	if err := json.Unmarshal([]byte(result.Value.Str()), &perf); err != nil {
		return PerformanceMetrics{}, err
	}
	return perf, nil
}

func performanceObserverScript() string {
	return `(() => {
		if (window.__wattlessMetrics) {
			return;
		}

		const metrics = {
			lcp_ms: 0,
			fcp_ms: 0
		};
		window.__wattlessMetrics = metrics;

		try {
			new PerformanceObserver((entryList) => {
				const entries = entryList.getEntries();
				const lastEntry = entries[entries.length - 1];
				if (lastEntry) {
					metrics.lcp_ms = Math.round(lastEntry.startTime || 0);
				}
			}).observe({ type: "largest-contentful-paint", buffered: true });
		} catch {}

		try {
			new PerformanceObserver((entryList) => {
				for (const entry of entryList.getEntries()) {
					if (entry.name === "first-contentful-paint" && !metrics.fcp_ms) {
						metrics.fcp_ms = Math.round(entry.startTime || 0);
					}
				}
			}).observe({ type: "paint", buffered: true });
		} catch {}
	})()`
}

func collectElementBoxes(page *rod.Page) ([]domElement, error) {
	result, err := page.Evaluate(rod.Eval(`() => {
		const seen = new Map();
		const selectors = ["img", "video", "iframe", "source"];
		for (const selector of selectors) {
			for (const node of document.querySelectorAll(selector)) {
				const rect = node.getBoundingClientRect();
				const url = node.currentSrc || node.src || node.getAttribute("src") || "";
				if (!url || rect.width <= 0 || rect.height <= 0) continue;
				if (!seen.has(url)) {
					seen.set(url, {
						url,
						x: Math.round(rect.x),
						y: Math.round(rect.y),
						width: Math.round(rect.width),
						height: Math.round(rect.height)
					});
				}
			}
		}
		return JSON.stringify(Array.from(seen.values()));
	}`))
	if err != nil {
		return nil, err
	}

	var boxes []domElement
	if err := json.Unmarshal([]byte(result.Value.Str()), &boxes); err != nil {
		return nil, err
	}
	return boxes, nil
}

func matchBoundingBox(resourceURL string, elements []domElement) *BoundingBox {
	for _, element := range elements {
		if sameAsset(resourceURL, element.URL) {
			return &BoundingBox{
				X:      element.X,
				Y:      element.Y,
				Width:  element.Width,
				Height: element.Height,
			}
		}
	}
	return nil
}

func sameAsset(left, right string) bool {
	return stripURLNoise(left) == stripURLNoise(right)
}

func stripURLNoise(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimSuffix(value, "/")
	if index := strings.Index(value, "?"); index >= 0 {
		value = value[:index]
	}
	if index := strings.Index(value, "#"); index >= 0 {
		value = value[:index]
	}
	return value
}

func intPtr(value int) *int {
	return &value
}

func makeInsightResources(resources []ResourceSummary) []insights.ResourceContext {
	output := make([]insights.ResourceContext, 0, len(resources))
	for _, resource := range resources {
		output = append(output, insights.ResourceContext{
			ID:                    resource.ID,
			URL:                   resource.URL,
			Type:                  resource.Type,
			MIMEType:              resource.MIMEType,
			Bytes:                 resource.Bytes,
			StatusCode:            resource.StatusCode,
			Failed:                resource.Failed,
			FailureReason:         resource.FailureReason,
			TransferShare:         resource.TransferShare,
			EstimatedSavingsBytes: resource.EstimatedSavingsBytes,
			Recommendation:        resource.Recommendation,
		})
	}
	return output
}
