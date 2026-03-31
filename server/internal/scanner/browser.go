package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/pkg/urlutil"
)

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
