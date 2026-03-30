package scanner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"net"
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

type PreparedTarget struct {
	RawURL        string
	NormalizedURL string
	Hostname      string
	ResolvedIP    string
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
	target, err := s.PrepareTarget(ctx, rawURL)
	if err != nil {
		return Report{}, err
	}

	return s.ScanPrepared(ctx, target)
}

func (s *Service) PrepareTarget(ctx context.Context, rawURL string) (PreparedTarget, error) {
	normalizedURL, hostname, err := urlutil.Normalize(rawURL)
	if err != nil {
		return PreparedTarget{}, err
	}

	resolvedIPs, err := urlutil.ValidatePublicTarget(ctx, hostname)
	if err != nil {
		return PreparedTarget{}, err
	}

	resolvedIP, err := preferredResolvedIP(resolvedIPs)
	if err != nil {
		return PreparedTarget{}, err
	}

	return PreparedTarget{
		RawURL:        rawURL,
		NormalizedURL: normalizedURL,
		Hostname:      hostname,
		ResolvedIP:    resolvedIP,
	}, nil
}

func (s *Service) ScanPrepared(ctx context.Context, target PreparedTarget) (Report, error) {
	startedAt := time.Now()
	resources, perf, screenshot, siteProfile, warnings, err := s.runBrowserScan(ctx, target)
	if err != nil {
		return Report{}, err
	}
	if warnings == nil {
		warnings = []string{}
	}
	resources, resourceGroups := enrichResourcesForAnalysis(resources, perf, screenshot.ViewportWidth, screenshot.ViewportHeight)

	hostingResult, hostingWarnings := s.resolveHosting(ctx, target.Hostname)
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
		potentialSavings += estimateResourceSavings(resource)
	}
	visualMapped := 0
	for _, resource := range top {
		savings := estimateResourceSavings(resource)
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
			PositionBand:          resource.PositionBand,
			VisualRole:            resource.VisualRole,
			DOMTag:                resource.DOMTag,
			LoadingAttr:           resource.LoadingAttr,
			FetchPriority:         resource.FetchPriority,
			ResponsiveImage:       resource.ResponsiveImage,
			NaturalWidth:          resource.NaturalWidth,
			NaturalHeight:         resource.NaturalHeight,
			VisibleRatio:          resource.VisibleRatio,
			IsThirdPartyTool:      resource.IsThirdPartyTool,
			ThirdPartyKind:        resource.ThirdPartyKind,
			BoundingBox:           resource.BoundingBox,
		})
	}

	grams := co2.FromBytes(totalBytes)
	breakdownByType, breakdownByParty := buildBreakdowns(resources, totalBytes)
	summary := buildSummary(resources, totalBytes, potentialSavings, visualMapped)
	analysis := buildAnalysis(resources, perf, resourceGroups)
	finalScore := score.FromCO2(grams)

	report := Report{
		URL:                   target.NormalizedURL,
		Score:                 finalScore,
		TotalBytesTransferred: totalBytes,
		CO2GramsPerVisit:      grams,
		HostingIsGreen:        hostingResult.IsGreen,
		HostingVerdict:        string(hostingResult.Verdict),
		HostedBy:              hostingResult.HostedBy,
		SiteProfile:           siteProfile,
		Summary:               summary,
		BreakdownByType:       breakdownByType,
		BreakdownByParty:      breakdownByParty,
		VampireElements:       vampires,
		Performance:           perf,
		Analysis:              analysis,
		Screenshot:            screenshot,
		Methodology:           defaultMethodology(),
		Warnings:              warnings,
	}

	reportContext := insights.ReportContext{
		URL:                   report.URL,
		Score:                 report.Score,
		TotalBytesTransferred: report.TotalBytesTransferred,
		CO2GramsPerVisit:      report.CO2GramsPerVisit,
		HostingIsGreen:        report.HostingIsGreen,
		HostingVerdict:        report.HostingVerdict,
		HostedBy:              report.HostedBy,
		SiteProfile: insights.SiteProfileContext{
			FrameworkHint: report.SiteProfile.FrameworkHint,
			Evidence:      append([]string(nil), report.SiteProfile.Evidence...),
		},
		Performance: insights.PerformanceContext{
			LoadMS:                   report.Performance.LoadMS,
			DOMContentLoadedMS:       report.Performance.DOMContentLoadedMS,
			ScriptResourceDurationMS: report.Performance.ScriptResourceDurationMS,
			LCPMS:                    report.Performance.LCPMS,
			FCPMS:                    report.Performance.FCPMS,
			LongTasksTotalMS:         report.Performance.LongTasksTotalMS,
			LongTasksCount:           report.Performance.LongTasksCount,
			LCPResourceURL:           report.Performance.LCPResourceURL,
			LCPResourceTag:           report.Performance.LCPResourceTag,
			LCPSelectorHint:          report.Performance.LCPSelectorHint,
			LCPSize:                  report.Performance.LCPSize,
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
		Analysis:     makeInsightAnalysis(report.Analysis),
		TopResources: makeInsightResources(vampires),
	}

	providerResult, err := s.insights.SummarizeReport(ctx, reportContext)
	if err != nil {
		s.logger.Warn("report_insights_failed", "url", target.NormalizedURL, "error", err)
		report.Warnings = append(report.Warnings, "La capa de IA no pudo enriquecer el informe; se usaron recomendaciones de respaldo.")
		fallbackResult, fallbackErr := insights.NewRuleBasedProvider().SummarizeReport(ctx, reportContext)
		if fallbackErr != nil {
			s.logger.Warn("report_insights_fallback_failed", "url", target.NormalizedURL, "error", fallbackErr)
		} else {
			providerResult = fallbackResult
		}
	}
	providerResult.Insights = sanitizeInsightReport(providerResult.Insights, report.Analysis.Findings, vampires)
	report.Insights = providerResult.Insights
	report.VampireElements = attachAssetInsights(
		vampires,
		report.Analysis,
		report.Insights.TopActions,
		providerResult.AssetInsights,
	)

	report.Meta = buildMeta(startedAt, time.Now())

	return report, nil
}

type enrichedResource struct {
	ID               string
	URL              string
	Type             string
	MIMEType         string
	Hostname         string
	Party            string
	StatusCode       int
	Bytes            int64
	Failed           bool
	FailureReason    string
	BoundingBox      *BoundingBox
	DOMTag           string
	LoadingAttr      string
	FetchPriority    string
	ResponsiveImage  bool
	NaturalWidth     int
	NaturalHeight    int
	VisibleRatio     float64
	SelectorHint     string
	PositionBand     string
	VisualRole       string
	IsThirdPartyTool bool
	ThirdPartyKind   string
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

func (s *Service) runBrowserScan(ctx context.Context, target PreparedTarget) ([]enrichedResource, PerformanceMetrics, Screenshot, SiteProfile, []string, error) {
	browserURL, cleanup, err := s.launchBrowser(target.Hostname, target.ResolvedIP)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	defer cleanup()

	browser := rod.New().ControlURL(browserURL)
	if err := browser.Connect(); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	defer func() { _ = browser.Close() }()

	browser = browser.Context(ctx)
	page, err := browser.Page(proto.TargetCreateTarget{})
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	defer func() { _ = page.Close() }()

	if err := (proto.EmulationSetDeviceMetricsOverride{
		Width:             s.cfg.ViewportWidth,
		Height:            s.cfg.ViewportHeight,
		DeviceScaleFactor: 1,
		Mobile:            false,
	}).Call(page); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}

	if err := (proto.NetworkEnable{}).Call(page); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}

	removePerfObserver, err := page.EvalOnNewDocument(performanceObserverScript())
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	defer func() { _ = removePerfObserver() }()

	resources := map[string]*rawResource{}
	mu := sync.Mutex{}
	pageHostname := resourceHostname(target.NormalizedURL)

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
	if err := page.Navigate(target.NormalizedURL); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	if err := page.WaitLoad(); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	select {
	case <-ctx.Done():
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, ctx.Err()
	case <-time.After(s.cfg.NetworkIdleWait):
	}

	performanceMetrics, err := capturePerformance(page)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}

	mu.Lock()
	baselineResources := snapshotRawResources(resources)
	mu.Unlock()

	metrics, err := measureDocument(page, s.cfg)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}

	warnings := make([]string, 0, 3)
	primingWarnings, err := primeScrollableContent(ctx, page, metrics, s.cfg)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	warnings = append(warnings, primingWarnings...)

	metrics, err = measureDocument(page, s.cfg)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}

	if err := scrollToTop(page); err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}

	snapshot, err := collectDOMSnapshot(page)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}

	plan := buildScreenshotPlan(metrics, s.cfg)
	screenshot, err := captureDocumentScreenshot(page, plan, s.cfg.FullPageCaptureQuality)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
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
		matchedElement := matchDOMElement(resource.URL, snapshot.Elements)
		var boundingBox *BoundingBox
		if matchedElement != nil {
			boundingBox = &BoundingBox{
				X:      matchedElement.X,
				Y:      matchedElement.Y,
				Width:  matchedElement.Width,
				Height: matchedElement.Height,
			}
		}
		enriched = append(enriched, enrichedResource{
			ID:              resource.RequestID,
			URL:             resource.URL,
			Type:            resource.Type,
			MIMEType:        resource.MIMEType,
			Hostname:        resourceHostname(resource.URL),
			Party:           classifyParty(pageHostname, resourceHostname(resource.URL)),
			StatusCode:      resource.StatusCode,
			Bytes:           resource.Bytes,
			Failed:          resource.Failed,
			FailureReason:   resource.FailureReason,
			BoundingBox:     boundingBox,
			DOMTag:          stringValue(matchedElement, func(element *domElement) string { return element.Tag }),
			LoadingAttr:     stringValue(matchedElement, func(element *domElement) string { return element.LoadingAttr }),
			FetchPriority:   stringValue(matchedElement, func(element *domElement) string { return element.FetchPriority }),
			ResponsiveImage: boolValue(matchedElement, func(element *domElement) bool { return element.ResponsiveImage }),
			NaturalWidth:    intValue(matchedElement, func(element *domElement) int { return element.NaturalWidth }),
			NaturalHeight:   intValue(matchedElement, func(element *domElement) int { return element.NaturalHeight }),
			VisibleRatio:    floatValue(matchedElement, func(element *domElement) float64 { return element.VisibleRatio }),
			SelectorHint:    stringValue(matchedElement, func(element *domElement) string { return element.SelectorHint }),
		})
		box := enriched[len(enriched)-1].BoundingBox
		if box != nil && box.Y >= float64(plan.CapturedHeight) {
			anchorsOutsideCapturedRange = true
		}
	}

	if anchorsOutsideCapturedRange {
		warnings = append(warnings, "Some visual anchors are below the captured range.")
	}

	return enriched, performanceMetrics, screenshot, normalizeSiteProfile(snapshot.SiteProfile), warnings, nil
}

func (s *Service) launchBrowser(hostname, resolvedIP string) (string, func(), error) {
	instance := launcher.New().
		Headless(true).
		Leakless(true).
		Set("disable-gpu").
		Set("no-sandbox").
		Set("disable-dev-shm-usage")

	if hostname != "" && resolvedIP != "" {
		instance = instance.Set("host-resolver-rules", fmt.Sprintf("MAP %s %s", hostname, chromiumResolverAddress(resolvedIP)))
	}

	if s.cfg.BrowserBin != "" {
		instance = instance.Bin(s.cfg.BrowserBin)
	}

	url, err := instance.Launch()
	if err != nil {
		return "", nil, fmt.Errorf("failed to launch Chromium: %w. Set BROWSER_BIN to a local Chromium/Chrome binary or run via docker compose", err)
	}

	return url, instance.Kill, nil
}

func preferredResolvedIP(addresses []net.IP) (string, error) {
	for _, address := range addresses {
		if ipv4 := address.To4(); ipv4 != nil {
			return ipv4.String(), nil
		}
	}

	for _, address := range addresses {
		if address == nil {
			continue
		}

		value := address.String()
		if value != "" {
			return value, nil
		}
	}

	return "", fmt.Errorf("%w: no public IP available", urlutil.ErrInvalidURL)
}

func chromiumResolverAddress(address string) string {
	if strings.Contains(address, ":") {
		return "[" + address + "]"
	}
	return address
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
			script_resource_duration_ms: Math.round(
				performance
					.getEntriesByType("resource")
					.filter((entry) => entry.initiatorType === "script")
					.reduce((acc, entry) => acc + (entry.duration || 0), 0)
			),
			lcp_ms: Math.round(buffered.lcp_ms || (lastLCP ? lastLCP.startTime || 0 : 0)),
			fcp_ms: Math.round(buffered.fcp_ms || (fcp ? fcp.startTime || 0 : 0)),
			long_tasks_total_ms: Math.round(buffered.long_tasks_total_ms || 0),
			long_tasks_count: Math.round(buffered.long_tasks_count || 0),
			lcp_resource_url: buffered.lcp_resource_url || (lastLCP && lastLCP.url ? lastLCP.url : ""),
			lcp_resource_tag: buffered.lcp_resource_tag || "",
			lcp_selector_hint: buffered.lcp_selector_hint || "",
			lcp_size: Math.round(buffered.lcp_size || (lastLCP && lastLCP.size ? lastLCP.size : 0))
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

		const selectorHint = (element) => {
			if (!element || !element.tagName) {
				return "";
			}
			if (element.id) {
				return "#" + element.id;
			}
			const className = typeof element.className === "string" ? element.className.trim().split(/\s+/).slice(0, 2).join(".") : "";
			if (className) {
				return element.tagName.toLowerCase() + "." + className;
			}
			return element.tagName.toLowerCase();
		};

		const metrics = {
			lcp_ms: 0,
			fcp_ms: 0,
			lcp_resource_url: "",
			lcp_resource_tag: "",
			lcp_selector_hint: "",
			lcp_size: 0,
			long_tasks_total_ms: 0,
			long_tasks_count: 0
		};
		window.__wattlessMetrics = metrics;

		try {
			new PerformanceObserver((entryList) => {
				const entries = entryList.getEntries();
				const lastEntry = entries[entries.length - 1];
				if (lastEntry) {
					metrics.lcp_ms = Math.round(lastEntry.startTime || 0);
					metrics.lcp_size = Math.round(lastEntry.size || 0);
					if (lastEntry.url) {
						metrics.lcp_resource_url = lastEntry.url;
					}
					const element = lastEntry.element;
					if (element) {
						metrics.lcp_resource_url = element.currentSrc || element.src || metrics.lcp_resource_url || "";
						metrics.lcp_resource_tag = (element.tagName || "").toLowerCase();
						metrics.lcp_selector_hint = selectorHint(element);
					}
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

		try {
			new PerformanceObserver((entryList) => {
				for (const entry of entryList.getEntries()) {
					metrics.long_tasks_total_ms += Math.round(entry.duration || 0);
					metrics.long_tasks_count += 1;
				}
			}).observe({ type: "longtask", buffered: true });
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

func collectDOMSnapshot(page *rod.Page) (domSnapshot, error) {
	result, err := page.Evaluate(rod.Eval(`() => {
		const clamp = (value, min, max) => Math.min(max, Math.max(min, value));
		const seen = new Map();
		const selectors = ["img", "video", "iframe", "source"];
		const viewportHeight = window.innerHeight || document.documentElement.clientHeight || 0;
		const selectorHint = (node) => {
			if (!node || !node.tagName) return "";
			if (node.id) return "#" + node.id;
			const className = typeof node.className === "string" ? node.className.trim().split(/\s+/).slice(0, 2).join(".") : "";
			if (className) return node.tagName.toLowerCase() + "." + className;
			return node.tagName.toLowerCase();
		};
		const hasResponsiveMarkup = (node, tagName) => {
			if (!node) return false;
			if (node.getAttribute("srcset") || node.getAttribute("sizes")) return true;
			if (tagName === "img" && typeof node.closest === "function") {
				const picture = node.closest("picture");
				if (picture && picture.querySelector('source[srcset], source[sizes]')) {
					return true;
				}
			}
			return false;
		};
		const siteProfile = {
			framework_hint: "generic",
			evidence: []
		};
		const pushEvidence = (value) => {
			if (!value || siteProfile.evidence.includes(value) || siteProfile.evidence.length >= 4) return;
			siteProfile.evidence.push(value);
		};
		if (document.querySelector("astro-island")) {
			siteProfile.framework_hint = "astro";
			pushEvidence("Se detectaron nodos astro-island.");
		}
		if (document.querySelector('script[src*="/_astro/"],link[href*="/_astro/"]')) {
			siteProfile.framework_hint = "astro";
			pushEvidence("Se detectaron assets servidos desde /_astro/.");
		}
		if (siteProfile.framework_hint === "generic" && document.getElementById("__NEXT_DATA__")) {
			siteProfile.framework_hint = "nextjs";
			pushEvidence("Se detectó __NEXT_DATA__.");
		}
		if (siteProfile.framework_hint === "generic" && document.querySelector('script[src*="/_next/"],link[href*="/_next/"]')) {
			siteProfile.framework_hint = "nextjs";
			pushEvidence("Se detectaron assets servidos desde /_next/.");
		}
		if (siteProfile.evidence.length === 0) {
			if (document.body) {
				pushEvidence("No se detectaron marcadores claros de framework; se usa perfil genérico.");
			} else {
				siteProfile.framework_hint = "unknown";
				pushEvidence("No se pudo inspeccionar el DOM renderizado.");
			}
		}
		for (const selector of selectors) {
			for (const node of document.querySelectorAll(selector)) {
				const rect = node.getBoundingClientRect();
				const url = node.currentSrc || node.src || node.getAttribute("src") || "";
				if (!url || rect.width <= 0 || rect.height <= 0) continue;
				let naturalWidth = 0;
				let naturalHeight = 0;
				const tagName = (node.tagName || "").toLowerCase();
				if (tagName === "img") {
					naturalWidth = node.naturalWidth || 0;
					naturalHeight = node.naturalHeight || 0;
				} else if (tagName === "video") {
					naturalWidth = node.videoWidth || 0;
					naturalHeight = node.videoHeight || 0;
				}
				const visibleHeight = clamp(Math.min(rect.bottom, viewportHeight) - Math.max(rect.top, 0), 0, rect.height);
				const visibleRatio = rect.height > 0 ? Math.round((visibleHeight / rect.height) * 1000) / 1000 : 0;
				if (!seen.has(url)) {
					seen.set(url, {
						url,
						tag: tagName,
						loading: node.getAttribute("loading") || "",
						fetch_priority: node.getAttribute("fetchpriority") || "",
						responsive_image: hasResponsiveMarkup(node, tagName),
						natural_width: naturalWidth,
						natural_height: naturalHeight,
						visible_ratio: visibleRatio,
						selector_hint: selectorHint(node),
						x: Math.round(rect.left + window.scrollX),
						y: Math.round(rect.top + window.scrollY),
						width: Math.round(rect.width),
						height: Math.round(rect.height)
					});
				}
			}
		}
		return JSON.stringify({
			elements: Array.from(seen.values()),
			site_profile: siteProfile
		});
	}`))
	if err != nil {
		return domSnapshot{}, err
	}

	var snapshot domSnapshot
	if err := json.Unmarshal([]byte(result.Value.Str()), &snapshot); err != nil {
		return domSnapshot{}, err
	}
	return snapshot, nil
}

func matchDOMElement(resourceURL string, elements []domElement) *domElement {
	for index := range elements {
		if sameAsset(resourceURL, elements[index].URL) {
			return &elements[index]
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

func intValue[T any](value *T, getter func(*T) int) int {
	if value == nil {
		return 0
	}
	return getter(value)
}

func floatValue[T any](value *T, getter func(*T) float64) float64 {
	if value == nil {
		return 0
	}
	return getter(value)
}

func normalizeSiteProfile(profile SiteProfile) SiteProfile {
	switch profile.FrameworkHint {
	case "astro", "nextjs", "generic", "unknown":
	default:
		profile.FrameworkHint = "generic"
	}
	if len(profile.Evidence) == 0 {
		profile.Evidence = []string{"No se detectaron marcadores claros de framework; se usa perfil genérico."}
	}
	return profile
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
			PositionBand:          resource.PositionBand,
			VisualRole:            resource.VisualRole,
			DOMTag:                resource.DOMTag,
			LoadingAttr:           resource.LoadingAttr,
			FetchPriority:         resource.FetchPriority,
			ResponsiveImage:       resource.ResponsiveImage,
			NaturalWidth:          resource.NaturalWidth,
			NaturalHeight:         resource.NaturalHeight,
			VisibleRatio:          resource.VisibleRatio,
			IsThirdPartyTool:      resource.IsThirdPartyTool,
			ThirdPartyKind:        resource.ThirdPartyKind,
		})
	}
	return output
}

func makeInsightAnalysis(analysis Analysis) insights.AnalysisContext {
	findings := make([]insights.AnalysisFindingContext, 0, len(analysis.Findings))
	for _, finding := range analysis.Findings {
		findings = append(findings, insights.AnalysisFindingContext{
			ID:                    finding.ID,
			Category:              finding.Category,
			Severity:              finding.Severity,
			Confidence:            finding.Confidence,
			Title:                 finding.Title,
			Summary:               finding.Summary,
			Evidence:              append([]string(nil), finding.Evidence...),
			EstimatedSavingsBytes: finding.EstimatedSavingsBytes,
			RelatedResourceIDs:    append([]string(nil), finding.RelatedResourceIDs...),
		})
	}

	groups := make([]insights.ResourceGroupContext, 0, len(analysis.ResourceGroups))
	for _, group := range analysis.ResourceGroups {
		groups = append(groups, insights.ResourceGroupContext{
			ID:                 group.ID,
			Kind:               group.Kind,
			Label:              group.Label,
			TotalBytes:         group.TotalBytes,
			ResourceCount:      group.ResourceCount,
			PositionBand:       group.PositionBand,
			RelatedResourceIDs: append([]string(nil), group.RelatedResourceIDs...),
		})
	}

	return insights.AnalysisContext{
		Summary: insights.AnalysisSummaryContext{
			AboveFoldBytes:       analysis.Summary.AboveFoldBytes,
			BelowFoldBytes:       analysis.Summary.BelowFoldBytes,
			LCPResourceID:        analysis.Summary.LCPResourceID,
			LCPResourceURL:       analysis.Summary.LCPResourceURL,
			LCPResourceBytes:     analysis.Summary.LCPResourceBytes,
			AnalyticsBytes:       analysis.Summary.AnalyticsBytes,
			AnalyticsRequests:    analysis.Summary.AnalyticsRequests,
			FontBytes:            analysis.Summary.FontBytes,
			FontRequests:         analysis.Summary.FontRequests,
			RepeatedGalleryBytes: analysis.Summary.RepeatedGalleryBytes,
			RepeatedGalleryCount: analysis.Summary.RepeatedGalleryCount,
			RenderCriticalBytes:  analysis.Summary.RenderCriticalBytes,
		},
		Findings:       findings,
		ResourceGroups: groups,
	}
}

func sanitizeInsightReport(result insights.ScanInsights, findings []AnalysisFinding, vampires []ResourceSummary) insights.ScanInsights {
	result.TopActions = sanitizeTopActions(result.TopActions, findings, vampires)
	return result
}

func sanitizeTopActions(actions []insights.TopAction, findings []AnalysisFinding, vampires []ResourceSummary) []insights.TopAction {
	if len(actions) == 0 {
		return actions
	}

	visibleIDs := make(map[string]struct{}, len(vampires))
	for _, vampire := range vampires {
		visibleIDs[vampire.ID] = struct{}{}
	}

	findingsByID := make(map[string]AnalysisFinding, len(findings))
	for _, finding := range findings {
		findingsByID[finding.ID] = finding
	}

	output := append([]insights.TopAction(nil), actions...)
	for index := range output {
		related := filterVisibleActionResourceIDs(output[index].RelatedResourceIDs, visibleIDs)
		if len(related) == 0 {
			if finding, ok := findingsByID[output[index].RelatedFindingID]; ok {
				related = filterVisibleActionResourceIDs(finding.RelatedResourceIDs, visibleIDs)
			}
		}
		if len(related) == 0 {
			if shouldFallbackTopActionResourceID(output[index].RelatedFindingID) {
				if fallback := fallbackTopActionResourceID(output[index].RelatedFindingID, vampires); fallback != "" {
					related = []string{fallback}
				}
			}
		}
		output[index].RelatedResourceIDs = related
	}

	return output
}

func filterVisibleActionResourceIDs(ids []string, visibleIDs map[string]struct{}) []string {
	filtered := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := visibleIDs[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		filtered = append(filtered, id)
	}
	return filtered
}

func shouldFallbackTopActionResourceID(findingID string) bool {
	switch findingID {
	case "responsive_image_overdelivery":
		return false
	default:
		return true
	}
}

func fallbackTopActionResourceID(findingID string, vampires []ResourceSummary) string {
	for _, vampire := range vampires {
		switch findingID {
		case "render_lcp_candidate":
			if vampire.VisualRole == visualRoleLCPCandidate || vampire.VisualRole == visualRoleHeroMedia || vampire.VisualRole == visualRoleAboveFoldMedia {
				return vampire.ID
			}
		case "render_lcp_dom_node":
			if vampire.Type == "font" || vampire.Type == "stylesheet" || vampire.Type == "script" {
				return vampire.ID
			}
			if vampire.VisualRole == visualRoleLCPCandidate || vampire.VisualRole == visualRoleHeroMedia || vampire.VisualRole == visualRoleAboveFoldMedia {
				return vampire.ID
			}
		case "repeated_gallery_overdelivery":
			if vampire.VisualRole == visualRoleRepeatedCard || vampire.VisualRole == visualRoleBelowFoldMedia || vampire.Type == "image" {
				return vampire.ID
			}
		case "third_party_analytics_overhead":
			if vampire.IsThirdPartyTool && vampire.ThirdPartyKind == thirdPartyAnalytics {
				return vampire.ID
			}
		case "font_stack_overweight":
			if vampire.Type == "font" {
				return vampire.ID
			}
		case "main_thread_cpu_pressure":
			if vampire.Type == "script" {
				return vampire.ID
			}
		case "heavy_above_fold_media":
			if vampire.VisualRole == visualRoleHeroMedia || vampire.VisualRole == visualRoleAboveFoldMedia {
				return vampire.ID
			}
		}
	}

	if len(vampires) == 0 {
		return ""
	}
	return vampires[0].ID
}

func stringValue[T any](value *T, project func(*T) string) string {
	if value == nil {
		return ""
	}
	return project(value)
}

func boolValue[T any](value *T, project func(*T) bool) bool {
	if value == nil {
		return false
	}
	return project(value)
}
