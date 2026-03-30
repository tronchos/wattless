package scanner

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
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
	if !perf.RenderMetricsComplete {
		warnings = append(warnings, "Render metrics could not be captured completely; LCP/FCP-based editorial framing may be limited for this scan.")
	}

	totalBytes := int64(0)
	for _, resource := range resources {
		totalBytes += resource.Bytes
	}

	potentialSavings := int64(0)
	for _, resource := range resources {
		potentialSavings += estimateResourceSavings(resource)
	}

	analysis := buildAnalysis(resources, perf, resourceGroups)
	top, rankingWarnings := rankVampireResources(resources, resourceGroups, analysis.Findings, totalBytes)
	warnings = append(warnings, rankingWarnings...)

	vampires := make([]ResourceSummary, 0, len(top))
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
			RenderMetricsComplete:    report.Performance.RenderMetricsComplete,
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

type layoutDocumentMetrics struct {
	ContentWidth  int
	ContentHeight int
	LayoutWidth   int
	LayoutHeight  int
}

type domDocumentMetrics struct {
	DocumentWidth  int `json:"document_width"`
	DocumentHeight int `json:"document_height"`
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

type capturedScreenshotTile struct {
	ID       string
	Y        int
	Width    int
	Height   int
	MimeType string
	Data     []byte
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
	plan := buildScreenshotPlan(metrics, s.cfg)
	captureSnapshotAfterScreenshot := plan.Strategy == "tiled"

	if !captureSnapshotAfterScreenshot {
		primingWarnings, err := primeScrollableContent(ctx, page, metrics, s.cfg)
		if err != nil {
			return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
		}
		warnings = append(warnings, primingWarnings...)

		metrics, err = measureDocument(page, s.cfg)
		if err != nil {
			return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
		}
		plan = buildScreenshotPlan(metrics, s.cfg)
	}

	scrollWarning, err := scrollToTopAndWait(ctx, page)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	if scrollWarning != "" {
		warnings = append(warnings, scrollWarning)
	}
	metrics, err = measureDocument(page, s.cfg)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	plan = buildScreenshotPlan(metrics, s.cfg)

	screenshot, screenshotWarnings, err := captureDocumentScreenshot(ctx, page, plan, s.cfg.FullPageCaptureQuality, s.cfg)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	warnings = append(warnings, screenshotWarnings...)
	if plan.Truncated {
		warnings = append(warnings, fmt.Sprintf("Visual inspector capture truncated at %dpx for efficiency.", plan.CapturedHeight))
	}

	scrollWarning, err = scrollToTopAndWait(ctx, page)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
	}
	if scrollWarning != "" {
		warnings = append(warnings, scrollWarning)
	}

	snapshot, err := collectDOMSnapshot(page)
	if err != nil {
		return nil, PerformanceMetrics{}, Screenshot{}, SiteProfile{}, nil, err
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
	perf.RenderMetricsComplete = perf.LCPMS > 0 && perf.FCPMS > 0
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

	layout := layoutDocumentMetrics{
		ContentWidth:  int(math.Ceil(metrics.CSSContentSize.Width)),
		ContentHeight: int(math.Ceil(metrics.CSSContentSize.Height)),
		LayoutWidth:   metrics.CSSLayoutViewport.ClientWidth,
		LayoutHeight:  metrics.CSSLayoutViewport.ClientHeight,
	}

	domMetrics, domErr := readDOMDocumentMetrics(page)
	if domErr != nil {
		domMetrics = domDocumentMetrics{}
	}

	return composeDocumentMetrics(layout, domMetrics, cfg), nil
}

func readDOMDocumentMetrics(page *rod.Page) (domDocumentMetrics, error) {
	result, err := page.Evaluate(rod.Eval(`() => {
		const doc = document.documentElement;
		const body = document.body;
		const widths = [
			doc ? doc.scrollWidth : 0,
			doc ? doc.offsetWidth : 0,
			doc ? doc.clientWidth : 0,
			body ? body.scrollWidth : 0,
			body ? body.offsetWidth : 0,
			body ? body.clientWidth : 0,
			window.innerWidth || 0
		];
		const heights = [
			doc ? doc.scrollHeight : 0,
			doc ? doc.offsetHeight : 0,
			doc ? doc.clientHeight : 0,
			body ? body.scrollHeight : 0,
			body ? body.offsetHeight : 0,
			body ? body.clientHeight : 0,
			window.innerHeight || 0
		];
		return JSON.stringify({
			document_width: Math.max(...widths),
			document_height: Math.max(...heights)
		});
	}`))
	if err != nil {
		return domDocumentMetrics{}, err
	}

	var metrics domDocumentMetrics
	if err := json.Unmarshal([]byte(result.Value.Str()), &metrics); err != nil {
		return domDocumentMetrics{}, err
	}
	return metrics, nil
}

func composeDocumentMetrics(layout layoutDocumentMetrics, dom domDocumentMetrics, cfg config.Config) documentMetrics {
	contentWidth := maxInt(layout.ContentWidth, dom.DocumentWidth)
	contentHeight := maxInt(layout.ContentHeight, dom.DocumentHeight)
	layoutWidth := maxInt(layout.LayoutWidth, 0)
	layoutHeight := maxInt(layout.LayoutHeight, 0)

	documentWidth := maxInt(layoutWidth, minInt(contentWidth, cfg.ViewportWidth))
	documentHeight := maxInt(contentHeight, cfg.ViewportHeight)

	return documentMetrics{
		ViewportWidth:  maxInt(cfg.ViewportWidth, layoutWidth),
		ViewportHeight: maxInt(cfg.ViewportHeight, layoutHeight),
		DocumentWidth:  maxInt(documentWidth, 1),
		DocumentHeight: maxInt(documentHeight, 1),
	}
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

func captureDocumentScreenshot(ctx context.Context, page *rod.Page, plan screenshotPlan, quality int, cfg config.Config) (Screenshot, []string, error) {
	if plan.Strategy != "tiled" {
		tiles := make([]ScreenshotTile, 0, len(plan.Tiles))
		for _, tilePlan := range plan.Tiles {
			tile, err := captureScreenshotTile(page, tilePlan, quality)
			if err != nil {
				return Screenshot{}, nil, err
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
		}, nil, nil
	}

	return captureComposedDocumentScreenshot(ctx, page, plan, cfg)
}

func captureComposedDocumentScreenshot(ctx context.Context, page *rod.Page, plan screenshotPlan, cfg config.Config) (Screenshot, []string, error) {
	var warnings []string

	neutralized, neutralizeRaw, err := neutralizeScrollHijack(page)
	slog.Info("scroll_hijack_neutralization", "neutralized", neutralized, "error", err, "raw", neutralizeRaw)
	if err != nil {
		warnings = append(warnings, "Scroll hijack neutralization failed; capture may be incomplete.")
	} else if neutralized {
		defer func() { _ = restoreScrollHijack(page) }()

		refreshed, err := measureDocument(page, cfg)
		if err == nil {
			plan = buildScreenshotPlan(refreshed, cfg)
			slog.Info("scroll_hijack_remeasured", "document_height", plan.DocumentHeight, "captured_height", plan.CapturedHeight, "strategy", plan.Strategy)
		}

		if err := scrollToTop(page); err != nil {
			warnings = append(warnings, "Could not reset scroll after neutralization.")
		}

		screenshot, expandErr := captureWithScrollPrime(ctx, page, plan, cfg)
		slog.Info("scroll_prime_capture", "success", expandErr == nil, "error", expandErr, "tiles", len(screenshot.Tiles))
		if expandErr == nil {
			return screenshot, warnings, nil
		}
		warnings = append(warnings, fmt.Sprintf("Scroll prime capture failed: %v; falling back to tiled capture.", expandErr))
	}

	segments := buildInternalCaptureSegments(plan, cfg)
	rawTiles := make([]capturedScreenshotTile, 0, len(segments))
	for _, segment := range segments {
		if err := advanceViewportToYFast(ctx, page, segment.Y); err != nil {
			return Screenshot{}, nil, err
		}

		tile, err := captureViewportScreenshotTile(page, segment)
		if err != nil {
			return Screenshot{}, nil, err
		}
		rawTiles = append(rawTiles, tile)
	}

	_ = scrollToTop(page)

	screenshot, composeWarnings, err := finalizeScrollableScreenshot(plan, rawTiles)
	warnings = append(warnings, composeWarnings...)
	return screenshot, warnings, err
}

func captureWithScrollPrime(ctx context.Context, page *rod.Page, plan screenshotPlan, cfg config.Config) (Screenshot, error) {
	_, _ = page.Evaluate(rod.Eval(`() => {
		window.__wattlessOriginalRAF = window.requestAnimationFrame;
		window.requestAnimationFrame = () => 0;
	}`))
	defer func() {
		_, _ = page.Evaluate(rod.Eval(`() => {
			if (window.__wattlessOriginalRAF) {
				window.requestAnimationFrame = window.__wattlessOriginalRAF;
				delete window.__wattlessOriginalRAF;
			}
		}`))
	}()

	quality := minInt(cfg.FullPageCaptureQuality, 55)
	segments := buildInternalCaptureSegments(plan, cfg)
	tiles := make([]ScreenshotTile, 0, len(segments))

	for i, seg := range segments {
		if ctx.Err() != nil {
			return Screenshot{}, ctx.Err()
		}

		_, _ = page.Evaluate(rod.Eval(`y => {
			const root = document.scrollingElement || document.documentElement;
			root.scrollTop = y;
			if (document.body) document.body.scrollTop = y;
			window.scrollTo(0, y);
		}`, seg.Y))

		if err := sleepWithContext(ctx, 50*time.Millisecond); err != nil {
			return Screenshot{}, err
		}

		req := proto.PageCaptureScreenshot{
			Format:  proto.PageCaptureScreenshotFormatWebp,
			Quality: intPtr(quality),
			Clip: &proto.PageViewport{
				X:      0,
				Y:      float64(seg.Y),
				Width:  float64(seg.Width),
				Height: float64(seg.Height),
				Scale:  1,
			},
			CaptureBeyondViewport: true,
		}
		result, err := req.Call(page)
		if err != nil {
			return Screenshot{}, fmt.Errorf("tile %d (y=%d): %w", i, seg.Y, err)
		}

		tiles = append(tiles, ScreenshotTile{
			ID:         seg.ID,
			Y:          seg.Y,
			Width:      seg.Width,
			Height:     seg.Height,
			DataBase64: encodeScreenshotBytes(result.Data),
		})
	}

	_, _ = page.Evaluate(rod.Eval(`() => {
		const root = document.scrollingElement || document.documentElement;
		root.scrollTop = 0;
		window.scrollTo(0, 0);
	}`))

	return Screenshot{
		MimeType:       "image/webp",
		Strategy:       "tiled",
		ViewportWidth:  plan.ViewportWidth,
		ViewportHeight: plan.ViewportHeight,
		DocumentWidth:  plan.DocumentWidth,
		DocumentHeight: plan.DocumentHeight,
		CapturedHeight: plan.CapturedHeight,
		Tiles:          tiles,
	}, nil
}

func buildInternalCaptureSegments(plan screenshotPlan, cfg config.Config) []screenshotTilePlan {
	segmentHeight := maxInt(cfg.FullPageTileHeight, 1)
	if segmentHeight > maxInt(plan.ViewportHeight, 1) {
		segmentHeight = maxInt(plan.ViewportHeight, 1)
	}
	segments := make([]screenshotTilePlan, 0, maxInt(1, int(math.Ceil(float64(plan.CapturedHeight)/float64(segmentHeight)))))
	for y := 0; y < plan.CapturedHeight; y += segmentHeight {
		height := minInt(segmentHeight, plan.CapturedHeight-y)
		segments = append(segments, screenshotTilePlan{
			ID:     fmt.Sprintf("segment-%d", len(segments)),
			Y:      y,
			Width:  plan.DocumentWidth,
			Height: height,
		})
	}
	return segments
}

func neutralizeScrollHijack(page *rod.Page) (bool, string, error) {
	result, err := page.Evaluate(rod.Eval(`() => {
		const html = document.documentElement;
		const body = document.body;
		if (!html || !body) {
			return JSON.stringify({ neutralized: false, reason: "no_dom" });
		}

		const cs = (el) => window.getComputedStyle(el);
		const htmlStyle = cs(html);
		const bodyStyle = cs(body);

		const bodyHasDeepContent = body.scrollHeight > body.clientHeight * 1.5;
		const htmlLocked = htmlStyle.overflow.includes("hidden") || htmlStyle.overflowY === "hidden";
		const bodyLocked = bodyStyle.overflow.includes("hidden") || bodyStyle.overflowY === "hidden";

		const diag = {
			bodyScrollHeight: body.scrollHeight,
			bodyClientHeight: body.clientHeight,
			htmlOverflow: htmlStyle.overflow,
			htmlOverflowY: htmlStyle.overflowY,
			bodyOverflow: bodyStyle.overflow,
			bodyOverflowY: bodyStyle.overflowY,
			bodyHasDeepContent,
			htmlLocked,
			bodyLocked,
		};

		if (!bodyHasDeepContent || (!htmlLocked && !bodyLocked)) {
			return JSON.stringify({ neutralized: false, reason: "no_hijack", ...diag });
		}

		window.__wattlessOriginalStyles = {
			htmlOverflow: html.style.overflow,
			htmlOverflowY: html.style.overflowY,
			htmlHeight: html.style.height,
			htmlMaxHeight: html.style.maxHeight,
			htmlPosition: html.style.position,
			bodyOverflow: body.style.overflow,
			bodyOverflowY: body.style.overflowY,
			bodyHeight: body.style.height,
			bodyMaxHeight: body.style.maxHeight,
			bodyPosition: body.style.position,
		};

		html.style.overflow = "visible";
		html.style.overflowY = "visible";
		html.style.height = "auto";
		html.style.maxHeight = "none";
		html.style.position = "static";
		body.style.overflow = "visible";
		body.style.overflowY = "visible";
		body.style.height = "auto";
		body.style.maxHeight = "none";
		body.style.position = "static";

		const wrapper = body.querySelector(
			"[data-lenis-content], [data-scroll-container], [data-scroll-section], .smooth-wrapper, .locomotive-scroll"
		);
		if (wrapper) {
			wrapper.style.transform = "none";
		}

		body.scrollTop = 0;
		html.scrollTop = 0;
		window.scrollTo(0, 0);

		delete window.__wattlessScrollTarget;

		return JSON.stringify({
			neutralized: true,
			bodyScrollHeight: body.scrollHeight,
			htmlScrollHeight: html.scrollHeight,
		});
	}`))
	if err != nil {
		return false, "", err
	}

	raw := result.Value.Str()
	var info struct {
		Neutralized bool `json:"neutralized"`
	}
	if err := json.Unmarshal([]byte(raw), &info); err != nil {
		return false, raw, err
	}
	return info.Neutralized, raw, nil
}

func restoreScrollHijack(page *rod.Page) error {
	_, err := page.Evaluate(rod.Eval(`() => {
		const orig = window.__wattlessOriginalStyles;
		if (!orig) return;
		const html = document.documentElement;
		const body = document.body;
		html.style.overflow = orig.htmlOverflow;
		html.style.overflowY = orig.htmlOverflowY;
		html.style.height = orig.htmlHeight;
		html.style.maxHeight = orig.htmlMaxHeight;
		html.style.position = orig.htmlPosition;
		body.style.overflow = orig.bodyOverflow;
		body.style.overflowY = orig.bodyOverflowY;
		body.style.height = orig.bodyHeight;
		body.style.maxHeight = orig.bodyMaxHeight;
		body.style.position = orig.bodyPosition;
		delete window.__wattlessOriginalStyles;
		delete window.__wattlessScrollTarget;
	}`))
	return err
}

func advanceViewportToY(ctx context.Context, page *rod.Page, targetY int) error {
	if err := scrollTo(page, targetY); err != nil {
		return err
	}
	if err := waitForScrollPosition(ctx, page, targetY, 1200*time.Millisecond); err != nil {
		return err
	}

	return waitForViewportSettle(ctx, page)
}

func advanceViewportToYFast(ctx context.Context, page *rod.Page, targetY int) error {
	if err := scrollTo(page, targetY); err != nil {
		return err
	}
	if err := sleepWithContext(ctx, 80*time.Millisecond); err != nil {
		return err
	}
	return waitForNextPaint(ctx, page)
}

func waitForViewportSettle(ctx context.Context, page *rod.Page) error {
	if err := sleepWithContext(ctx, 60*time.Millisecond); err != nil {
		return err
	}
	if err := page.WaitDOMStable(100*time.Millisecond, 0); err != nil {
		return err
	}
	if err := waitForNextPaint(ctx, page); err != nil {
		return err
	}
	if err := waitForVisibleImages(ctx, page, 150*time.Millisecond); err != nil {
		return err
	}
	return waitForNextPaint(ctx, page)
}

func waitForScrollPosition(ctx context.Context, page *rod.Page, targetY int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		scrollY, err := readScrollY(page)
		if err != nil {
			return err
		}
		if withinScrollTolerance(scrollY, targetY) {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("viewport did not reach target scroll position %d (current %d)", targetY, scrollY)
		}
		if err := sleepWithContext(ctx, 50*time.Millisecond); err != nil {
			return err
		}
	}
}

func withinScrollTolerance(currentY, targetY int) bool {
	return absInt(currentY-targetY) <= 2
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func waitForVisibleImages(ctx context.Context, page *rod.Page, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		_, err := page.Evaluate(rod.Eval(`timeoutMs => new Promise((resolve) => {
			const images = Array.from(document.images || []).filter((img) => {
				const rect = img.getBoundingClientRect();
				return rect.bottom > 0 && rect.top < window.innerHeight && rect.right > 0 && rect.left < window.innerWidth;
			});
			if (images.length === 0) {
				resolve(true);
				return;
			}

			let settled = false;
			const finish = () => {
				if (settled) return;
				settled = true;
				resolve(true);
			};

			const timer = setTimeout(finish, timeoutMs);
			const waiters = images.map((img) => {
				if (img.complete) {
					return Promise.resolve();
				}
				if (typeof img.decode === "function") {
					return img.decode().catch(() => undefined);
				}
				return new Promise((imageResolve) => {
					img.addEventListener("load", imageResolve, { once: true });
					img.addEventListener("error", imageResolve, { once: true });
				});
			});

			Promise.allSettled(waiters).finally(() => {
				clearTimeout(timer);
				finish();
			});
		})`, int(timeout.Milliseconds())))
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func captureViewportScreenshotTile(page *rod.Page, tile screenshotTilePlan) (capturedScreenshotTile, error) {
	req := scrolledTileCaptureRequest(tile)

	result, err := req.Call(page)
	if err != nil {
		return capturedScreenshotTile{}, err
	}

	return capturedScreenshotTile{
		ID:       tile.ID,
		Y:        tile.Y,
		Width:    tile.Width,
		Height:   tile.Height,
		MimeType: "image/png",
		Data:     result.Data,
	}, nil
}

func finalizeScrollableScreenshot(plan screenshotPlan, rawTiles []capturedScreenshotTile) (Screenshot, []string, error) {
	composed, err := composeCapturedTilesPNG(plan.DocumentWidth, plan.CapturedHeight, rawTiles)
	if err == nil {
		return Screenshot{
			MimeType:       "image/png",
			Strategy:       "single",
			ViewportWidth:  plan.ViewportWidth,
			ViewportHeight: plan.ViewportHeight,
			DocumentWidth:  plan.DocumentWidth,
			DocumentHeight: plan.DocumentHeight,
			CapturedHeight: plan.CapturedHeight,
			Tiles: []ScreenshotTile{
				{
					ID:         "tile-0",
					Y:          0,
					Width:      plan.DocumentWidth,
					Height:     plan.CapturedHeight,
					DataBase64: encodeScreenshotBytes(composed),
				},
			},
		}, nil, nil
	}

	tiles := make([]ScreenshotTile, 0, len(rawTiles))
	for _, tile := range rawTiles {
		tiles = append(tiles, ScreenshotTile{
			ID:         tile.ID,
			Y:          tile.Y,
			Width:      tile.Width,
			Height:     tile.Height,
			DataBase64: encodeScreenshotBytes(tile.Data),
		})
	}

	return Screenshot{
		MimeType:       "image/png",
		Strategy:       "tiled",
		ViewportWidth:  plan.ViewportWidth,
		ViewportHeight: plan.ViewportHeight,
		DocumentWidth:  plan.DocumentWidth,
		DocumentHeight: plan.DocumentHeight,
		CapturedHeight: plan.CapturedHeight,
		Tiles:          tiles,
	}, []string{"Screenshot composition failed; returning raw capture tiles."}, nil
}

func composeCapturedTilesPNG(documentWidth, capturedHeight int, tiles []capturedScreenshotTile) ([]byte, error) {
	if documentWidth <= 0 || capturedHeight <= 0 {
		return nil, fmt.Errorf("invalid screenshot composition size %dx%d", documentWidth, capturedHeight)
	}
	if len(tiles) == 0 {
		return nil, fmt.Errorf("no screenshot tiles to compose")
	}

	canvas := image.NewNRGBA(image.Rect(0, 0, documentWidth, capturedHeight))
	for _, tile := range tiles {
		img, _, err := image.Decode(bytes.NewReader(tile.Data))
		if err != nil {
			return nil, fmt.Errorf("decode tile %s: %w", tile.ID, err)
		}
		bounds := img.Bounds()
		drawWidth := minInt(bounds.Dx(), documentWidth)
		drawHeight := minInt(bounds.Dy(), tile.Height)
		dstRect := image.Rect(0, tile.Y, drawWidth, minInt(tile.Y+drawHeight, capturedHeight))
		draw.Draw(canvas, dstRect, img, bounds.Min, draw.Src)
	}

	var buffer bytes.Buffer
	if err := png.Encode(&buffer, canvas); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func captureScreenshotTile(page *rod.Page, tile screenshotTilePlan, quality int) (ScreenshotTile, error) {
	req := proto.PageCaptureScreenshot{
		Format:                proto.PageCaptureScreenshotFormatWebp,
		Quality:               intPtr(quality),
		Clip:                  documentScreenshotClipForTile(tile),
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

func documentScreenshotClipForTile(tile screenshotTilePlan) *proto.PageViewport {
	return &proto.PageViewport{
		X:      0,
		Y:      float64(tile.Y),
		Width:  float64(tile.Width),
		Height: float64(tile.Height),
		Scale:  1,
	}
}

func scrolledTileCaptureRequest(tile screenshotTilePlan) proto.PageCaptureScreenshot {
	return proto.PageCaptureScreenshot{
		Format:                proto.PageCaptureScreenshotFormatPng,
		Clip:                  viewportScreenshotClipForTile(tile),
		FromSurface:           false,
		CaptureBeyondViewport: false,
	}
}

func viewportScreenshotClipForTile(tile screenshotTilePlan) *proto.PageViewport {
	return &proto.PageViewport{
		X:      0,
		Y:      0,
		Width:  float64(tile.Width),
		Height: float64(tile.Height),
		Scale:  1,
	}
}

func scrollToTop(page *rod.Page) error {
	return scrollTo(page, 0)
}

func scrollToTopAndWait(ctx context.Context, page *rod.Page) (string, error) {
	if err := scrollToTop(page); err != nil {
		return "", err
	}

	deadline := time.Now().Add(1200 * time.Millisecond)
	for {
		scrollY, err := readScrollY(page)
		if err != nil {
			return "", err
		}
		if scrollY == 0 {
			if err := waitForNextPaint(ctx, page); err != nil {
				return "", err
			}
			settledScrollY, err := readScrollY(page)
			if err != nil {
				return "", err
			}
			if settledScrollY == 0 {
				return "", nil
			}
		}

		if time.Now().After(deadline) {
			return "Visual inspector snapshot could not fully return to the first viewport; fold and visibility hints may be less precise.", nil
		}
		if err := sleepWithContext(ctx, 50*time.Millisecond); err != nil {
			return "", err
		}
		if err := scrollToTop(page); err != nil {
			return "", err
		}
	}
}

func scrollTo(page *rod.Page, y int) error {
	_, err := page.Evaluate(rod.Eval(`targetY => {
		const clamp = (value, min, max) => Math.min(max, Math.max(min, value));
		const getCachedScroller = () => {
			const cached = window.__wattlessScrollTarget;
			if (!cached || !(cached instanceof HTMLElement) || !cached.isConnected) {
				return null;
			}
			const range = Math.max((cached.scrollHeight || 0) - (cached.clientHeight || 0), 0);
			return range > 0 ? cached : null;
		};
		const pickScroller = () => {
			const cached = getCachedScroller();
			if (cached) return cached;
			const root = document.scrollingElement || document.documentElement || document.body;
			let best = root;
			let bestRange = root ? Math.max((root.scrollHeight || 0) - (root.clientHeight || 0), 0) : 0;
			for (const el of document.querySelectorAll("*")) {
				if (!(el instanceof HTMLElement)) continue;
				const style = window.getComputedStyle(el);
				const overflowY = style.overflowY || "";
				if (!/(auto|scroll|overlay)/.test(overflowY)) continue;
				const range = Math.max((el.scrollHeight || 0) - (el.clientHeight || 0), 0);
				if (range <= 0) continue;
				if (range > bestRange + 32) {
					best = el;
					bestRange = range;
				}
			}
			window.__wattlessScrollTarget = best || root || null;
			return window.__wattlessScrollTarget;
		};

		const scroller = pickScroller();
		if (!scroller) {
			window.scrollTo(0, targetY);
			return Math.round(window.scrollY || document.documentElement.scrollTop || 0);
		}

		const maxY = Math.max((scroller.scrollHeight || 0) - (scroller.clientHeight || 0), 0);
		const nextY = clamp(targetY, 0, maxY);
		if (typeof scroller.scrollTo === "function") {
			scroller.scrollTo(0, nextY);
		} else {
			scroller.scrollTop = nextY;
		}
		return Math.round(scroller.scrollTop || window.scrollY || document.documentElement.scrollTop || 0);
	}`, y))
	return err
}

func readScrollY(page *rod.Page) (int, error) {
	result, err := page.Evaluate(rod.Eval(`() => {
		const cached = window.__wattlessScrollTarget;
		const best = cached && cached instanceof HTMLElement && cached.isConnected ? cached : (document.scrollingElement || document.documentElement || document.body);
		return Math.round((best && best.scrollTop) || window.scrollY || document.documentElement.scrollTop || 0);
	}`))
	if err != nil {
		return 0, err
	}
	return result.Value.Int(), nil
}

func waitForNextPaint(ctx context.Context, page *rod.Page) error {
	done := make(chan error, 1)
	go func() {
		_, err := page.Evaluate(rod.Eval(`() => new Promise((resolve) => {
			requestAnimationFrame(() => {
				requestAnimationFrame(() => resolve(true));
			});
		})`))
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
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
			AboveFoldVisualBytes: analysis.Summary.AboveFoldVisualBytes,
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
		related := dedupeActionResourceIDs(output[index].RelatedResourceIDs)
		if len(related) == 0 {
			if finding, ok := findingsByID[output[index].RelatedFindingID]; ok {
				related = dedupeActionResourceIDs(finding.RelatedResourceIDs)
			}
		}
		output[index].RelatedResourceIDs = related
		output[index].VisibleRelatedResourceIDs = filterVisibleActionResourceIDs(related, visibleIDs)
	}

	return output
}

func dedupeActionResourceIDs(ids []string) []string {
	filtered := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
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
