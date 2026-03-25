package scanner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
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

const scannerVersion = "2026.03"

func NewService(cfg config.Config, hostingChecker HostingChecker, insightsProvider insights.Provider, logger *slog.Logger) *Service {
	return &Service{
		cfg:            cfg,
		hostingChecker: hostingChecker,
		insights:       insightsProvider,
		logger:         logger,
	}
}

func (s *Service) Scan(ctx context.Context, rawURL string) (Report, error) {
	startedAt := time.Now()

	normalizedURL, hostname, err := urlutil.Normalize(rawURL)
	if err != nil {
		return Report{}, err
	}
	if err := urlutil.ValidatePublicTarget(ctx, hostname); err != nil {
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
		Methodology:           defaultMethodology(),
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

	report.Meta = buildMeta(startedAt, time.Now())

	return report, nil
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

type documentMetrics struct {
	ViewportWidth  int
	ViewportHeight int
	DocumentWidth  int
	DocumentHeight int
}

type screenshotTilePlan struct {
	ID     string
	Y      int
	Width  int
	Height int
}

type screenshotPlan struct {
	Strategy       string
	ViewportWidth  int
	ViewportHeight int
	DocumentWidth  int
	DocumentHeight int
	CapturedHeight int
	Tiles          []screenshotTilePlan
	Truncated      bool
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

	mu.Lock()
	baselineResources := snapshotRawResources(resources)
	mu.Unlock()

	metrics, err := measureDocument(page, s.cfg)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	warnings := make([]string, 0, 3)
	primingWarnings, err := primeScrollableContent(deadlineCtx, page, metrics, s.cfg)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}
	warnings = append(warnings, primingWarnings...)

	metrics, err = measureDocument(page, s.cfg)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	if err := scrollToTop(page); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	elements, err := collectElementBoxes(page)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}

	plan := buildScreenshotPlan(metrics, s.cfg)
	screenshot, err := captureDocumentScreenshot(page, plan, s.cfg.FullPageCaptureQuality)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, nil, err
	}
	if plan.Truncated {
		warnings = append(warnings, fmt.Sprintf("Visual inspector capture truncated at %dpx for efficiency.", plan.CapturedHeight))
	}

	enriched := make([]enrichedResource, 0, len(baselineResources))
	anchorsOutsideCapturedRange := false
	for _, resource := range baselineResources {
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
		box := enriched[len(enriched)-1].BoundingBox
		if box != nil && box.Y >= float64(plan.CapturedHeight) {
			anchorsOutsideCapturedRange = true
		}
	}

	if anchorsOutsideCapturedRange {
		warnings = append(warnings, "Some visual anchors are below the captured range.")
	}

	return enriched, performanceMetrics, screenshot, warnings, nil
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

func buildMeta(startedAt, finishedAt time.Time) Meta {
	return Meta{
		GeneratedAt:    finishedAt.UTC().Format(time.RFC3339),
		ScanDurationMS: finishedAt.Sub(startedAt).Milliseconds(),
		ScannerVersion: scannerVersion,
	}
}

func defaultMethodology() Methodology {
	return Methodology{
		Model:   "sustainable-web-design-mvp",
		Formula: "(bytes / 1_000_000_000) * 0.8 * 0.75 * 442",
		Source:  "Sustainable Web Design",
		Assumptions: []string{
			"0.75 return-visit factor",
			"442 gCO2e/kWh global average",
		},
	}
}

func measureDocument(page *rod.Page, cfg config.Config) (documentMetrics, error) {
	metrics, err := (proto.PageGetLayoutMetrics{}).Call(page)
	if err != nil {
		return documentMetrics{}, err
	}
	if metrics.CSSContentSize == nil || metrics.CSSLayoutViewport == nil {
		return documentMetrics{}, fmt.Errorf("failed to measure document layout")
	}

	contentWidth := int(math.Ceil(metrics.CSSContentSize.Width))
	contentHeight := int(math.Ceil(metrics.CSSContentSize.Height))
	layoutWidth := metrics.CSSLayoutViewport.ClientWidth
	layoutHeight := metrics.CSSLayoutViewport.ClientHeight

	documentWidth := maxInt(layoutWidth, minInt(contentWidth, cfg.ViewportWidth))
	documentHeight := maxInt(contentHeight, cfg.ViewportHeight)

	return documentMetrics{
		ViewportWidth:  maxInt(cfg.ViewportWidth, layoutWidth),
		ViewportHeight: maxInt(cfg.ViewportHeight, layoutHeight),
		DocumentWidth:  maxInt(documentWidth, 1),
		DocumentHeight: maxInt(documentHeight, 1),
	}, nil
}

func primeScrollableContent(ctx context.Context, page *rod.Page, metrics documentMetrics, cfg config.Config) ([]string, error) {
	if metrics.DocumentHeight <= metrics.ViewportHeight {
		return nil, nil
	}

	step := maxInt(int(math.Round(float64(metrics.ViewportHeight)*0.75)), 1)
	maxHeight := minInt(metrics.DocumentHeight, cfg.FullPageMaxHeight)
	deadline := time.Now().Add(cfg.FullPagePrimeMaxDuration)
	partial := false

	for y := step; y < maxHeight; y += step {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if time.Now().After(deadline) {
			partial = true
			break
		}

		if err := scrollTo(page, y); err != nil {
			return nil, err
		}
		if err := sleepWithContext(ctx, 150*time.Millisecond); err != nil {
			return nil, err
		}

		refreshed, err := measureDocument(page, cfg)
		if err == nil {
			metrics = refreshed
			maxHeight = minInt(metrics.DocumentHeight, cfg.FullPageMaxHeight)
		}
	}

	if err := scrollToTop(page); err != nil {
		return nil, err
	}
	if err := sleepWithContext(ctx, 300*time.Millisecond); err != nil {
		return nil, err
	}

	if partial {
		return []string{"Lazy content was partially hydrated before capture."}, nil
	}
	return nil, nil
}

func buildScreenshotPlan(metrics documentMetrics, cfg config.Config) screenshotPlan {
	capturedHeight := minInt(metrics.DocumentHeight, cfg.FullPageMaxHeight)
	if capturedHeight <= 0 {
		capturedHeight = metrics.ViewportHeight
	}

	strategy := "single"
	tileHeight := capturedHeight
	if capturedHeight > cfg.FullPageSingleShotThreshold {
		strategy = "tiled"
		tileHeight = maxInt(cfg.FullPageTileHeight, 1)
	}

	tiles := make([]screenshotTilePlan, 0, maxInt(1, int(math.Ceil(float64(capturedHeight)/float64(maxInt(tileHeight, 1))))))
	for y := 0; y < capturedHeight; y += tileHeight {
		height := minInt(tileHeight, capturedHeight-y)
		tiles = append(tiles, screenshotTilePlan{
			ID:     fmt.Sprintf("tile-%d", len(tiles)),
			Y:      y,
			Width:  metrics.DocumentWidth,
			Height: height,
		})
	}

	return screenshotPlan{
		Strategy:       strategy,
		ViewportWidth:  metrics.ViewportWidth,
		ViewportHeight: metrics.ViewportHeight,
		DocumentWidth:  metrics.DocumentWidth,
		DocumentHeight: metrics.DocumentHeight,
		CapturedHeight: capturedHeight,
		Tiles:          tiles,
		Truncated:      metrics.DocumentHeight > capturedHeight,
	}
}

func captureDocumentScreenshot(page *rod.Page, plan screenshotPlan, quality int) (Screenshot, error) {
	tiles := make([]ScreenshotTile, 0, len(plan.Tiles))
	for _, tilePlan := range plan.Tiles {
		tile, err := captureScreenshotTile(page, tilePlan, quality)
		if err != nil {
			return Screenshot{}, err
		}
		tiles = append(tiles, tile)
	}

	return Screenshot{
		MimeType:       "image/webp",
		Strategy:       plan.Strategy,
		ViewportWidth:  plan.ViewportWidth,
		ViewportHeight: plan.ViewportHeight,
		DocumentWidth:  plan.DocumentWidth,
		DocumentHeight: plan.DocumentHeight,
		CapturedHeight: plan.CapturedHeight,
		Tiles:          tiles,
	}, nil
}

func captureScreenshotTile(page *rod.Page, tile screenshotTilePlan, quality int) (ScreenshotTile, error) {
	req := proto.PageCaptureScreenshot{
		Format:                proto.PageCaptureScreenshotFormatWebp,
		Quality:               intPtr(quality),
		Clip:                  &proto.PageViewport{X: 0, Y: float64(tile.Y), Width: float64(tile.Width), Height: float64(tile.Height), Scale: 1},
		CaptureBeyondViewport: true,
	}

	result, err := req.Call(page)
	if err != nil {
		return ScreenshotTile{}, err
	}

	return ScreenshotTile{
		ID:         tile.ID,
		Y:          tile.Y,
		Width:      tile.Width,
		Height:     tile.Height,
		DataBase64: encodeScreenshotBytes(result.Data),
	}, nil
}

func scrollToTop(page *rod.Page) error {
	return scrollTo(page, 0)
}

func scrollTo(page *rod.Page, y int) error {
	_, err := page.Evaluate(rod.Eval(`targetY => {
		window.scrollTo(0, targetY);
		return window.scrollY || document.documentElement.scrollTop || 0;
	}`, y))
	return err
}

func sleepWithContext(ctx context.Context, wait time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(wait):
		return nil
	}
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
						x: Math.round(rect.left + window.scrollX),
						y: Math.round(rect.top + window.scrollY),
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

func encodeScreenshotBytes(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func snapshotRawResources(resources map[string]*rawResource) []rawResource {
	snapshot := make([]rawResource, 0, len(resources))
	for _, resource := range resources {
		if resource == nil {
			continue
		}
		snapshot = append(snapshot, *resource)
	}
	return snapshot
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
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
