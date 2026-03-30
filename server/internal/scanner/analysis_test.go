package scanner

import (
	"strings"
	"testing"
)

func TestNormalizeTypePrefersURLHints(t *testing.T) {
	got := normalizeType("Other", "text/html", "https://example.com/favicon.ico")
	if got != "document" {
		t.Fatalf("expected html mime to win, got %s", got)
	}
}

func TestClassifyPartyUsesSiteRoot(t *testing.T) {
	if got := classifyParty("app.example.com", "cdn.example.com"); got != partyFirst {
		t.Fatalf("expected first party, got %s", got)
	}
	if got := classifyParty("example.com", "tracker.other.net"); got != partyThird {
		t.Fatalf("expected third party, got %s", got)
	}
	if got := classifyParty("docs.example.github.io", "cdn.other.github.io"); got != partyThird {
		t.Fatalf("expected github.io tenants to remain third party, got %s", got)
	}
}

func TestClassifyPartyTreatsBrandedCDNAsFirstParty(t *testing.T) {
	if got := classifyParty("www.marca.com", "objetos.estaticos-marca.com"); got != partyFirst {
		t.Fatalf("expected branded cdn to be first party, got %s", got)
	}
}

func TestClassifyThirdPartyKindRecognizesAdobeAndYTIMG(t *testing.T) {
	adobe := enrichedResource{
		URL:      "https://assets.adobedtm.com/launch-EN.js",
		Hostname: "assets.adobedtm.com",
		Party:    partyThird,
	}
	if got := classifyThirdPartyKind(adobe); got != thirdPartyAnalytics {
		t.Fatalf("expected adobe tag manager to be analytics, got %s", got)
	}

	video := enrichedResource{
		URL:      "https://i.ytimg.com/vi/example/maxresdefault.jpg",
		Hostname: "i.ytimg.com",
		Party:    partyThird,
	}
	if got := classifyThirdPartyKind(video); got != thirdPartyVideo {
		t.Fatalf("expected ytimg to be video, got %s", got)
	}
}

func TestClassifyThirdPartyKindDoesNotTreatEditorialAdSlugAsAds(t *testing.T) {
	resource := enrichedResource{
		URL:      "https://cdn.example.net/files/ad_208_webp/example.webp",
		Hostname: "cdn.example.net",
		Party:    partyThird,
	}
	if got := classifyThirdPartyKind(resource); got != thirdPartyUnknown {
		t.Fatalf("expected branded editorial path to avoid ads classification, got %s", got)
	}
}

func TestRankVampireResourcesKeepsFailedRequestsWhenTheyTransferBytes(t *testing.T) {
	resources := []enrichedResource{
		{ID: "req-1", URL: "https://example.com/favicon.ico", Bytes: 9000, StatusCode: 404, Failed: true, Party: partyFirst, Type: "image"},
		{ID: "req-2", URL: "https://example.com/app.js", Bytes: 5000, StatusCode: 200, Party: partyFirst, Type: "script"},
	}

	ranked, warnings := rankVampireResources(resources, nil, nil, 14_000)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked resources, got %d", len(ranked))
	}
	if ranked[0].URL != "https://example.com/favicon.ico" {
		t.Fatalf("unexpected ranked resource: %s", ranked[0].URL)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings")
	}
}

func TestBuildSummaryCountsNetworkFailuresWithoutStatusCodes(t *testing.T) {
	summary := buildSummary([]enrichedResource{
		{ID: "req-1", URL: "https://example.com/app.js", Bytes: 1200, Failed: true, FailureReason: "net::ERR_BLOCKED_BY_CLIENT", Party: partyThird},
		{ID: "req-2", URL: "https://example.com/style.css", Bytes: 800, StatusCode: 200, Party: partyFirst},
	}, 2_000, 300, 1)

	if summary.FailedRequests != 1 {
		t.Fatalf("expected 1 failed request, got %d", summary.FailedRequests)
	}
	if summary.SuccessfulRequests != 1 {
		t.Fatalf("expected 1 successful request, got %d", summary.SuccessfulRequests)
	}
}

func TestClassifyPositionBand(t *testing.T) {
	if got := classifyPositionBand(&BoundingBox{Y: 100, Height: 200}, 0.8, 900); got != positionAboveFold {
		t.Fatalf("expected above fold, got %s", got)
	}
	if got := classifyPositionBand(&BoundingBox{Y: 1200, Height: 200}, 0, 900); got != positionNearFold {
		t.Fatalf("expected near fold, got %s", got)
	}
	if got := classifyPositionBand(&BoundingBox{Y: 2400, Height: 200}, 0, 900); got != positionBelowFold {
		t.Fatalf("expected below fold, got %s", got)
	}
}

func TestClassifyPositionBandTreatsFoldEdgeAsNearFold(t *testing.T) {
	box := &BoundingBox{Y: 899, Height: 233}
	if got := classifyPositionBand(box, 1.0/233.0, 900); got != positionNearFold {
		t.Fatalf("expected near fold at viewport edge, got %s", got)
	}
}

func TestClassifyPositionBandDoesNotUpgradeDeepPageAssetFromVisibleRatio(t *testing.T) {
	box := &BoundingBox{Y: 3100, Height: 40}
	if got := classifyPositionBand(box, 1, 900); got != positionBelowFold {
		t.Fatalf("expected deep page asset to remain below fold, got %s", got)
	}
}

func TestSummarizePositionBandUsesDeterministicPriority(t *testing.T) {
	got := summarizePositionBand([]enrichedResource{
		{PositionBand: positionNearFold},
		{PositionBand: positionBelowFold},
	})

	if got != positionNearFold {
		t.Fatalf("expected near fold to win tie by priority, got %s", got)
	}
}

func TestClassifyThirdPartyKindDetectsAnalytics(t *testing.T) {
	resource := enrichedResource{
		URL:      "https://us.i.posthog.com/static/array.js",
		Hostname: "us.i.posthog.com",
		Party:    partyThird,
	}

	if got := classifyThirdPartyKind(resource); got != thirdPartyAnalytics {
		t.Fatalf("expected analytics, got %s", got)
	}
}

func TestEnrichResourcesForAnalysisAssignsSemanticRoles(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:          "hero",
			URL:         "https://example.com/hero.webp",
			Type:        "image",
			Hostname:    "example.com",
			Party:       partyFirst,
			Bytes:       340_000,
			BoundingBox: &BoundingBox{X: 0, Y: 0, Width: 960, Height: 420},
		},
		{
			ID:          "card-1",
			URL:         "https://example.com/courses/course-1.webp",
			Type:        "image",
			Hostname:    "example.com",
			Party:       partyFirst,
			Bytes:       180_000,
			BoundingBox: &BoundingBox{X: 0, Y: 1100, Width: 320, Height: 180},
		},
		{
			ID:          "card-2",
			URL:         "https://example.com/courses/course-2.webp",
			Type:        "image",
			Hostname:    "example.com",
			Party:       partyFirst,
			Bytes:       170_000,
			BoundingBox: &BoundingBox{X: 340, Y: 1120, Width: 320, Height: 180},
		},
		{
			ID:          "card-3",
			URL:         "https://example.com/courses/course-3.webp",
			Type:        "image",
			Hostname:    "example.com",
			Party:       partyFirst,
			Bytes:       165_000,
			BoundingBox: &BoundingBox{X: 680, Y: 1150, Width: 320, Height: 180},
		},
	}

	annotated, groups := enrichResourcesForAnalysis(resources, PerformanceMetrics{
		LCPResourceURL: "https://example.com/hero.webp",
	}, 1440, 900)

	if annotated[0].VisualRole != visualRoleLCPCandidate {
		t.Fatalf("expected lcp candidate, got %s", annotated[0].VisualRole)
	}
	if annotated[1].VisualRole != visualRoleRepeatedCard {
		t.Fatalf("expected repeated card media, got %s", annotated[1].VisualRole)
	}
	if len(groups) == 0 || groups[0].Kind != groupKindRepeatedGallery {
		t.Fatalf("expected repeated gallery group, got %#v", groups)
	}
}

func TestBuildAnalysisCreatesFindingsFromEvidence(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:           "hero",
			URL:          "https://example.com/hero.webp",
			Type:         "image",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        340_000,
			BoundingBox:  &BoundingBox{X: 0, Y: 0, Width: 960, Height: 420},
			PositionBand: positionAboveFold,
			VisualRole:   visualRoleLCPCandidate,
		},
		{
			ID:               "analytics",
			URL:              "https://us.i.posthog.com/static/array.js",
			Type:             "script",
			Hostname:         "us.i.posthog.com",
			Party:            partyThird,
			Bytes:            95_000,
			IsThirdPartyTool: true,
			ThirdPartyKind:   thirdPartyAnalytics,
			PositionBand:     positionUnknown,
		},
		{
			ID:           "font-1",
			URL:          "https://example.com/fonts/brand.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        150_000,
			PositionBand: positionUnknown,
		},
		{
			ID:           "font-2",
			URL:          "https://example.com/fonts/brand-bold.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        130_000,
			PositionBand: positionUnknown,
		},
	}

	analysis := buildAnalysis(resources, PerformanceMetrics{
		LCPMS:                    2400,
		LCPResourceURL:           "https://example.com/hero.webp",
		LCPResourceTag:           "img",
		ScriptResourceDurationMS: 420,
		LongTasksTotalMS:         410,
		LongTasksCount:           3,
	}, []ResourceGroup{
		{
			ID:                 "group-analytics",
			Kind:               groupKindThirdParty,
			Label:              "Cluster de analítica",
			TotalBytes:         95_000,
			ResourceCount:      1,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"analytics"},
		},
	})

	if len(analysis.Findings) == 0 {
		t.Fatal("expected findings")
	}
	if analysis.Summary.LCPResourceID != "hero" {
		t.Fatalf("expected lcp resource hero, got %s", analysis.Summary.LCPResourceID)
	}
	if !hasFinding(analysis.Findings, "render_lcp_candidate") {
		t.Fatal("expected lcp finding")
	}
	if !hasFinding(analysis.Findings, "third_party_analytics_overhead") {
		t.Fatal("expected analytics finding")
	}
	if !hasFinding(analysis.Findings, "font_stack_overweight") {
		t.Fatal("expected font finding")
	}
	if !hasFinding(analysis.Findings, "main_thread_cpu_pressure") {
		t.Fatal("expected main thread finding")
	}
}

func TestBuildAnalysisKeepsSmallRealLCPAsRenderCandidate(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:           "lcp-image",
			URL:          "https://example.com/hero.webp",
			Type:         "image",
			MIMEType:     "image/webp",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        16_817,
			BoundingBox:  &BoundingBox{X: 0, Y: 0, Width: 655, Height: 380},
			PositionBand: positionAboveFold,
			VisualRole:   visualRoleLCPCandidate,
		},
	}

	analysis := buildAnalysis(resources, PerformanceMetrics{
		LCPMS:                 1680,
		FCPMS:                 1400,
		RenderMetricsComplete: true,
		LCPResourceURL:        "https://example.com/hero.webp",
		LCPResourceTag:        "img",
	}, nil)

	finding := findFinding(analysis.Findings, "render_lcp_candidate")
	if finding == nil {
		t.Fatal("expected small real lcp resource to keep render_lcp_candidate")
	}
	if strings.Contains(strings.ToLower(finding.Summary), "sin asset de red") {
		t.Fatalf("expected real lcp resource summary, got %q", finding.Summary)
	}
	if hasFinding(analysis.Findings, "render_lcp_dom_node") {
		t.Fatal("expected dom-node fallback to stay absent when lcp resource exists")
	}
}

func TestBuildAnalysisSuppressesFastSmallRealLCPFinding(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:           "lcp-image",
			URL:          "https://example.com/hero.webp",
			Type:         "image",
			MIMEType:     "image/webp",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        16_817,
			BoundingBox:  &BoundingBox{X: 0, Y: 0, Width: 655, Height: 380},
			PositionBand: positionAboveFold,
			VisualRole:   visualRoleLCPCandidate,
		},
	}

	analysis := buildAnalysis(resources, PerformanceMetrics{
		LCPMS:                 1400,
		FCPMS:                 1100,
		RenderMetricsComplete: true,
		LCPResourceURL:        "https://example.com/hero.webp",
		LCPResourceTag:        "img",
	}, nil)

	if hasFinding(analysis.Findings, "render_lcp_candidate") {
		t.Fatalf("expected fast small real lcp resource to stay silent, got %#v", analysis.Findings)
	}
}

func TestBuildThirdPartyFindingsIncludesAdsFinding(t *testing.T) {
	resources := []enrichedResource{
		{ID: "ad-1", Type: "script", Party: partyThird, Bytes: 220_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyAds},
		{ID: "ad-2", Type: "script", Party: partyThird, Bytes: 180_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyAds},
		{ID: "ad-3", Type: "script", Party: partyThird, Bytes: 160_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyAds},
	}
	groups := []ResourceGroup{
		{
			ID:                 "group-third-party-ads",
			Kind:               groupKindThirdParty,
			Label:              "Cluster de anuncios",
			TotalBytes:         560_000,
			ResourceCount:      12,
			RelatedResourceIDs: []string{"ad-1", "ad-2", "ad-3"},
		},
	}

	findings := buildThirdPartyFindings(resources, groups)
	finding := findFinding(findings, "third_party_ads_overhead")
	if finding == nil {
		t.Fatal("expected ads finding")
	}
	if finding.EstimatedSavingsBytes != 420_000 {
		t.Fatalf("expected deferred ads savings factor, got %d", finding.EstimatedSavingsBytes)
	}
}

func TestBuildAnalysisAddsLegacyFormatFindings(t *testing.T) {
	resources := []enrichedResource{
		{ID: "img-1", Type: "image", MIMEType: "image/jpeg", Bytes: 400_000},
		{ID: "img-2", Type: "image", MIMEType: "image/jpeg", Bytes: 350_000},
		{ID: "img-3", Type: "image", MIMEType: "image/png", Bytes: 200_000},
		{ID: "img-4", Type: "image", MIMEType: "image/jpeg", Bytes: 150_000},
		{ID: "img-5", Type: "image", MIMEType: "image/jpeg", Bytes: 120_000},
		{ID: "font-1", Type: "font", MIMEType: "font/woff", URL: "https://example.com/fonts/brand.woff", Bytes: 60_000},
		{ID: "font-2", Type: "font", MIMEType: "font/woff", URL: "https://example.com/fonts/brand-bold.woff", Bytes: 55_000},
	}

	analysis := buildAnalysis(resources, PerformanceMetrics{}, nil)
	if !hasFinding(analysis.Findings, "legacy_image_format_overhead") {
		t.Fatal("expected legacy image format finding")
	}
	if !hasFinding(analysis.Findings, "legacy_font_format_overhead") {
		t.Fatal("expected legacy font format finding")
	}
}

func TestBuildAnalysisSkipsLegacyFontFindingWhenWOFFHasWOFF2Equivalent(t *testing.T) {
	resources := []enrichedResource{
		{ID: "font-modern", Type: "font", MIMEType: "font/woff2", URL: "https://example.com/fonts/brand.woff2", Bytes: 62_000},
		{ID: "font-fallback", Type: "font", MIMEType: "font/woff", URL: "https://example.com/fonts/brand.woff", Bytes: 61_000},
	}

	analysis := buildAnalysis(resources, PerformanceMetrics{}, nil)
	if hasFinding(analysis.Findings, "legacy_font_format_overhead") {
		t.Fatal("expected legacy font finding to stay silent when only fallback woff remains alongside woff2")
	}
}

func TestRankVampireResourcesSuppressesTinyImageNoise(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:          "tiny-thumb",
			URL:         "https://example.com/thumb.jpg",
			Type:        "image",
			MIMEType:    "image/jpeg",
			Bytes:       3_000,
			BoundingBox: &BoundingBox{X: 80, Y: 20, Width: 25, Height: 37},
			VisualRole:  visualRoleAboveFoldMedia,
		},
		{
			ID:           "real-font",
			URL:          "https://example.com/font.woff2",
			Type:         "font",
			MIMEType:     "font/woff2",
			Bytes:        40_000,
			PositionBand: positionUnknown,
		},
	}

	ranked, _ := rankVampireResources(resources, nil, nil, 43_000)
	for _, resource := range ranked {
		if resource.ID == "tiny-thumb" {
			t.Fatalf("expected noisy resources to be suppressed, got %#v", ranked)
		}
	}
}

func TestRankVampireResourcesKeepsHTMLInImgTargetsEligible(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:          "html-img",
			URL:         "https://example.com/subscription/",
			Type:        "document",
			MIMEType:    "text/html",
			Bytes:       75_000,
			DOMTag:      "img",
			BoundingBox: &BoundingBox{X: 0, Y: 800, Width: 320, Height: 180},
		},
		{
			ID:           "real-font",
			URL:          "https://example.com/font.woff2",
			Type:         "font",
			MIMEType:     "font/woff2",
			Bytes:        40_000,
			PositionBand: positionUnknown,
		},
	}

	ranked, _ := rankVampireResources(resources, nil, nil, 115_000)
	for _, resource := range ranked {
		if resource.ID == "html-img" {
			return
		}
	}
	t.Fatalf("expected html served into img target to remain eligible, got %#v", ranked)
}

func TestRankVampireResourcesKeepsVisibleHTMLWidgetsWhenTheyAreNotBrokenImgTargets(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:               "payment-widget",
			URL:              "https://buy.example.com/widget",
			Type:             "document",
			MIMEType:         "text/html",
			Bytes:            180_000,
			Party:            partyThird,
			IsThirdPartyTool: true,
			ThirdPartyKind:   thirdPartyPayment,
			BoundingBox:      &BoundingBox{X: 80, Y: 1200, Width: 960, Height: 640},
			VisualRole:       visualRoleBelowFoldMedia,
			PositionBand:     positionBelowFold,
			DOMTag:           "iframe",
		},
		{
			ID:           "fallback-font",
			URL:          "https://example.com/font.woff2",
			Type:         "font",
			MIMEType:     "font/woff2",
			Bytes:        20_000,
			PositionBand: positionUnknown,
		},
	}

	ranked, _ := rankVampireResources(resources, nil, nil, 200_000)
	for _, resource := range ranked {
		if resource.ID == "payment-widget" {
			return
		}
	}
	t.Fatalf("expected visible html widget to remain eligible as vampire anchor, got %#v", ranked)
}

func TestBuildMainThreadFindingUsesLowConfidenceNearThresholdBand(t *testing.T) {
	finding := buildMainThreadFinding([]enrichedResource{
		{
			ID:    "script-1",
			Type:  "script",
			Bytes: 80_000,
		},
	}, PerformanceMetrics{
		LongTasksTotalMS:         220,
		LongTasksCount:           3,
		ScriptResourceDurationMS: 1_800,
	})

	if finding == nil {
		t.Fatal("expected near-threshold cpu finding")
	}
	if finding.Severity != "low" {
		t.Fatalf("expected low severity near threshold, got %q", finding.Severity)
	}
	if finding.Confidence != "low" {
		t.Fatalf("expected low confidence near threshold, got %q", finding.Confidence)
	}
	if !strings.Contains(strings.ToLower(finding.Summary), "oscilar") {
		t.Fatalf("expected summary to explain scan variability, got %q", finding.Summary)
	}
}

func TestBuildMainThreadFindingStaysNilBelowNearThresholdBand(t *testing.T) {
	finding := buildMainThreadFinding([]enrichedResource{
		{
			ID:    "script-1",
			Type:  "script",
			Bytes: 80_000,
		},
	}, PerformanceMetrics{
		LongTasksTotalMS:         180,
		LongTasksCount:           2,
		ScriptResourceDurationMS: 1_100,
	})

	if finding != nil {
		t.Fatalf("expected cpu finding below near-threshold band, got %#v", finding)
	}
}

func TestBuildAnalysisCreatesTextLCPFindingWithoutAssetURL(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:           "font-1",
			URL:          "https://example.com/fonts/brand.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        170_000,
			PositionBand: positionUnknown,
		},
		{
			ID:           "style-1",
			URL:          "https://example.com/app.css",
			Type:         "stylesheet",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        60_000,
			PositionBand: positionUnknown,
		},
		{
			ID:           "script-1",
			URL:          "https://example.com/app.js",
			Type:         "script",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        55_000,
			PositionBand: positionUnknown,
		},
	}

	analysis := buildAnalysis(resources, PerformanceMetrics{
		LCPMS:           2200,
		LCPResourceTag:  "h1",
		LCPSelectorHint: "h1.hero-title",
		LCPSize:         3200,
	}, nil)

	finding := findFinding(analysis.Findings, "render_lcp_dom_node")
	if finding == nil {
		t.Fatal("expected text lcp finding")
	}
	if len(finding.RelatedResourceIDs) == 0 {
		t.Fatal("expected related resources for text lcp finding")
	}
	if !strings.Contains(strings.Join(finding.Evidence, " "), "h1.hero-title") {
		t.Fatalf("expected selector hint in evidence, got %#v", finding.Evidence)
	}
}

func TestBuildRepeatedGalleryFindingAvoidsBelowFoldClaimForMixedGroup(t *testing.T) {
	resources := []enrichedResource{
		{ID: "card-1", Type: "image", Bytes: 220_000, MIMEType: "image/webp", NaturalWidth: 1920, NaturalHeight: 1080, BoundingBox: &BoundingBox{Width: 414, Height: 233}},
		{ID: "card-2", Type: "image", Bytes: 210_000, MIMEType: "image/webp", NaturalWidth: 1920, NaturalHeight: 1080, BoundingBox: &BoundingBox{Width: 414, Height: 233}},
		{ID: "card-3", Type: "image", Bytes: 205_000, MIMEType: "image/webp", NaturalWidth: 1920, NaturalHeight: 1080, BoundingBox: &BoundingBox{Width: 414, Height: 233}},
	}

	finding := buildRepeatedGalleryFinding([]ResourceGroup{
		{
			ID:                 "group-cards",
			Kind:               groupKindRepeatedGallery,
			Label:              "Grid de tarjetas",
			TotalBytes:         635_000,
			ResourceCount:      3,
			PositionBand:       "mixed",
			RelatedResourceIDs: []string{"card-1", "card-2", "card-3"},
		},
	}, resources, nil)

	if finding == nil {
		t.Fatal("expected repeated gallery finding")
	}
	if finding.ID != "repeated_gallery_overdelivery" {
		t.Fatalf("expected renamed finding id, got %q", finding.ID)
	}
	title := strings.ToLower(finding.Title)
	summary := strings.ToLower(finding.Summary)
	if strings.Contains(title, "bajo el fold") {
		t.Fatalf("expected conservative title, got %q", finding.Title)
	}
	if strings.Contains(summary, "no frena el primer render") {
		t.Fatalf("expected conservative summary, got %q", finding.Summary)
	}
}

func TestRepeatedGalleryLabelUsesSemanticPathHintsForBlog(t *testing.T) {
	label := repeatedGalleryLabel([]enrichedResource{
		{
			URL:          "https://example.com/img/blog/post-1.jpg",
			PositionBand: positionBelowFold,
			BoundingBox:  &BoundingBox{Width: 288, Height: 114},
		},
		{
			URL:          "https://example.com/img/blog/post-2.avif",
			PositionBand: positionBelowFold,
			BoundingBox:  &BoundingBox{Width: 288, Height: 114},
		},
		{
			URL:          "https://example.com/img/blog/post-3.webp",
			PositionBand: positionBelowFold,
			BoundingBox:  &BoundingBox{Width: 288, Height: 114},
		},
	}, 900)

	if label != "Miniaturas del blog" {
		t.Fatalf("expected semantic blog label, got %q", label)
	}
}

func TestRepeatedGalleryLabelUsesSemanticPathHintsForSponsors(t *testing.T) {
	label := repeatedGalleryLabel([]enrichedResource{
		{
			URL:          "https://example.com/sponsors/acme.webp",
			PositionBand: positionBelowFold,
			BoundingBox:  &BoundingBox{Width: 260, Height: 48},
		},
		{
			URL:          "https://example.com/sponsors/contoso.webp",
			PositionBand: positionBelowFold,
			BoundingBox:  &BoundingBox{Width: 260, Height: 48},
		},
	}, 900)

	if label != "Logos de sponsors" {
		t.Fatalf("expected sponsor label, got %q", label)
	}
}

func TestRepeatedGalleryLabelFallsBackToSpeakerGalleryByStructure(t *testing.T) {
	resources := []enrichedResource{
		{URL: "https://example.com/img/ana-garcia.webp", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{URL: "https://example.com/img/luis-perez.webp", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{URL: "https://example.com/img/maria-lopez.webp", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{URL: "https://example.com/img/carla-ruiz.webp", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{URL: "https://example.com/img/julio-gomez.webp", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{URL: "https://example.com/img/sofia-vargas.webp", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{URL: "https://example.com/img/pablo-arias.webp", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{URL: "https://example.com/img/nora-fuentes.webp", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
	}

	if label := repeatedGalleryLabel(resources, 900); label != "Fotos de speakers" {
		t.Fatalf("expected structural speaker label, got %q", label)
	}
}

func TestBuildDominantImageFindingDetectsCatastrophicOutlier(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:            "blog-jpeg",
			URL:           "https://cdn.example.com/img/blog/post-1.jpg",
			Type:          "image",
			MIMEType:      "image/jpeg",
			Bytes:         3_967_443,
			NaturalWidth:  3024,
			NaturalHeight: 4032,
			BoundingBox:   &BoundingBox{X: 90, Y: 4303, Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "blog-avif",
			URL:           "https://cdn.example.com/img/blog/post-2.avif",
			Type:          "image",
			MIMEType:      "image/avif",
			Bytes:         532_665,
			NaturalWidth:  1964,
			NaturalHeight: 2455,
			BoundingBox:   &BoundingBox{X: 400, Y: 4303, Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "blog-webp",
			URL:           "https://cdn.example.com/img/blog/post-3.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Bytes:         179_274,
			NaturalWidth:  1964,
			NaturalHeight: 2455,
			BoundingBox:   &BoundingBox{X: 710, Y: 4303, Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
	}

	group := ResourceGroup{
		ID:                 "group-blog",
		Kind:               groupKindRepeatedGallery,
		Label:              "Miniaturas del blog",
		TotalBytes:         sumBytes(resources),
		ResourceCount:      len(resources),
		PositionBand:       positionBelowFold,
		RelatedResourceIDs: collectResourceIDs(resources),
	}

	finding := buildDominantImageFinding(resources, []ResourceGroup{group})
	if finding == nil {
		t.Fatal("expected dominant image finding")
	}
	if finding.ID != "dominant_image_overdelivery" {
		t.Fatalf("expected dominant image finding id, got %q", finding.ID)
	}
	if finding.Severity != "high" {
		t.Fatalf("expected catastrophic outlier to escalate to high severity, got %q", finding.Severity)
	}
	if finding.RelatedResourceIDs[0] != "blog-jpeg" {
		t.Fatalf("expected dominant jpeg to anchor the finding, got %#v", finding.RelatedResourceIDs)
	}
	evidence := strings.Join(finding.Evidence, " ")
	if !strings.Contains(strings.ToLower(evidence), "formatos legacy y modernos") {
		t.Fatalf("expected mixed-format evidence, got %#v", finding.Evidence)
	}
	if strings.Contains(strings.ToLower(evidence), "variantes modernas mucho más ligeras") {
		t.Fatalf("expected heterogeneous group to avoid same-variant claim, got %#v", finding.Evidence)
	}
	if !strings.Contains(strings.ToLower(finding.Summary), "miniaturas del blog") {
		t.Fatalf("expected summary to mention semantic group label, got %q", finding.Summary)
	}
}

func TestBuildDominantImageFindingRequiresMeasuredRenderedBox(t *testing.T) {
	finding := buildDominantImageFinding([]enrichedResource{
		{
			ID:            "unmapped-jpeg",
			URL:           "https://cdn.example.com/img/blog/post-1.jpg",
			Type:          "image",
			MIMEType:      "image/jpeg",
			Bytes:         1_800_000,
			NaturalWidth:  3024,
			NaturalHeight: 4032,
		},
	}, nil)

	if finding != nil {
		t.Fatalf("expected unmapped image to avoid dominant-image finding, got %#v", finding)
	}
}

func TestLegacyAssetOutweighsModernSiblingRequiresComparableVariant(t *testing.T) {
	candidate := enrichedResource{
		ID:            "blog-jpeg",
		URL:           "https://cdn.example.com/img/blog/post-1.jpg?w=800",
		Type:          "image",
		MIMEType:      "image/jpeg",
		Bytes:         3_967_443,
		NaturalWidth:  3024,
		NaturalHeight: 4032,
		BoundingBox:   &BoundingBox{Width: 288, Height: 114},
	}
	members := []enrichedResource{
		candidate,
		{
			ID:            "blog-avif",
			URL:           "https://cdn.example.com/img/blog/post-2.avif?w=800",
			Type:          "image",
			MIMEType:      "image/avif",
			Bytes:         532_665,
			NaturalWidth:  1964,
			NaturalHeight: 2455,
			BoundingBox:   &BoundingBox{Width: 288, Height: 114},
		},
	}

	if legacyAssetOutweighsModernSibling(candidate, members) {
		t.Fatal("expected different images in the same group to avoid same-variant format claim")
	}
}

func TestLegacyAssetOutweighsModernSiblingMatchesSameVariant(t *testing.T) {
	candidate := enrichedResource{
		ID:            "blog-jpeg",
		URL:           "https://cdn.example.com/img/blog/post-1.jpg?w=800",
		Type:          "image",
		MIMEType:      "image/jpeg",
		Bytes:         3_967_443,
		NaturalWidth:  3024,
		NaturalHeight: 4032,
		BoundingBox:   &BoundingBox{Width: 288, Height: 114},
	}
	members := []enrichedResource{
		candidate,
		{
			ID:            "blog-avif",
			URL:           "https://cdn.example.com/img/blog/post-1.avif?w=800",
			Type:          "image",
			MIMEType:      "image/avif",
			Bytes:         532_665,
			NaturalWidth:  3024,
			NaturalHeight: 4032,
			BoundingBox:   &BoundingBox{Width: 288, Height: 114},
		},
	}

	if !legacyAssetOutweighsModernSibling(candidate, members) {
		t.Fatal("expected same-image modern variant to be recognized")
	}
}

func TestBuildRepeatedGalleryFindingSuppressesDominatedGroupBelowThreshold(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:            "dominant",
			URL:           "https://cdn.example.com/img/blog/post-1.jpg",
			Type:          "image",
			MIMEType:      "image/jpeg",
			Bytes:         1_000_000,
			NaturalWidth:  3024,
			NaturalHeight: 4032,
			BoundingBox:   &BoundingBox{Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "sibling-1",
			URL:           "https://cdn.example.com/img/blog/post-2.avif",
			Type:          "image",
			MIMEType:      "image/avif",
			Bytes:         140_000,
			NaturalWidth:  1964,
			NaturalHeight: 2455,
			BoundingBox:   &BoundingBox{Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "sibling-2",
			URL:           "https://cdn.example.com/img/blog/post-3.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Bytes:         120_000,
			NaturalWidth:  1964,
			NaturalHeight: 2455,
			BoundingBox:   &BoundingBox{Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
	}
	group := ResourceGroup{
		ID:                 "group-blog",
		Kind:               groupKindRepeatedGallery,
		Label:              "Miniaturas del blog",
		TotalBytes:         sumBytes(resources),
		ResourceCount:      len(resources),
		PositionBand:       positionBelowFold,
		RelatedResourceIDs: collectResourceIDs(resources),
	}

	dominant := &AnalysisFinding{
		ID:                 "dominant_image_overdelivery",
		RelatedResourceIDs: []string{"dominant"},
	}
	if finding := buildRepeatedGalleryFinding([]ResourceGroup{group}, resources, dominant); finding != nil {
		t.Fatalf("expected dominated gallery below threshold to be suppressed, got %#v", finding)
	}
}

func TestBuildThirdPartyFindingsIncludesSocialCluster(t *testing.T) {
	resources := []enrichedResource{
		{ID: "social-1", Type: "script", Bytes: 120_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartySocial},
		{ID: "social-2", Type: "script", Bytes: 110_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartySocial},
		{ID: "social-3", Type: "script", Bytes: 90_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartySocial},
		{ID: "social-4", Type: "script", Bytes: 64_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartySocial},
		{ID: "analytics-1", Type: "script", Bytes: 180_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyAnalytics},
		{ID: "analytics-2", Type: "script", Bytes: 90_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyAnalytics},
	}
	groups := []ResourceGroup{
		{
			ID:                 "group-social",
			Kind:               groupKindThirdParty,
			Label:              "Cluster social",
			TotalBytes:         384_000,
			ResourceCount:      11,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"social-1", "social-2", "social-3", "social-4"},
		},
		{
			ID:                 "group-analytics",
			Kind:               groupKindThirdParty,
			Label:              "Cluster de analítica",
			TotalBytes:         270_000,
			ResourceCount:      2,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"analytics-1", "analytics-2"},
		},
	}

	findings := buildThirdPartyFindings(resources, groups)
	if !hasFinding(findings, "third_party_analytics_overhead") {
		t.Fatalf("expected analytics finding, got %#v", findings)
	}
	if !hasFinding(findings, "third_party_social_overhead") {
		t.Fatalf("expected social finding, got %#v", findings)
	}
}

func TestBuildThirdPartyFindingsIncludePaymentAndVideoClusters(t *testing.T) {
	resources := []enrichedResource{
		{ID: "payment-1", Type: "script", Bytes: 120_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyPayment},
		{ID: "payment-2", Type: "script", Bytes: 100_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyPayment},
		{ID: "video-1", Type: "script", Bytes: 170_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyVideo},
		{ID: "video-2", Type: "image", Bytes: 120_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyVideo},
	}
	groups := []ResourceGroup{
		{
			ID:                 "group-payment",
			Kind:               groupKindThirdParty,
			Label:              "Cluster de pagos",
			TotalBytes:         220_000,
			ResourceCount:      7,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"payment-1", "payment-2"},
		},
		{
			ID:                 "group-video",
			Kind:               groupKindThirdParty,
			Label:              "Embeds de video",
			TotalBytes:         290_000,
			ResourceCount:      4,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"video-1", "video-2"},
		},
	}

	findings := buildThirdPartyFindings(resources, groups)
	if !hasFinding(findings, "third_party_payment_overhead") {
		t.Fatalf("expected payment finding, got %#v", findings)
	}
	if !hasFinding(findings, "third_party_video_overhead") {
		t.Fatalf("expected video finding, got %#v", findings)
	}
}

func TestBuildRepeatedGalleryFindingNotesLazyMajority(t *testing.T) {
	resources := []enrichedResource{
		{ID: "speaker-1", Type: "image", Bytes: 120_000, MIMEType: "image/webp", NaturalWidth: 800, NaturalHeight: 800, LoadingAttr: "lazy", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{ID: "speaker-2", Type: "image", Bytes: 118_000, MIMEType: "image/webp", NaturalWidth: 800, NaturalHeight: 800, LoadingAttr: "lazy", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{ID: "speaker-3", Type: "image", Bytes: 116_000, MIMEType: "image/webp", NaturalWidth: 800, NaturalHeight: 800, LoadingAttr: "lazy", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
		{ID: "speaker-4", Type: "image", Bytes: 114_000, MIMEType: "image/webp", NaturalWidth: 800, NaturalHeight: 800, LoadingAttr: "lazy", BoundingBox: &BoundingBox{Width: 284, Height: 300}},
	}

	finding := buildRepeatedGalleryFinding([]ResourceGroup{
		{
			ID:                 "group-speakers",
			Kind:               groupKindRepeatedGallery,
			Label:              "Fotos de speakers",
			TotalBytes:         468_000,
			ResourceCount:      4,
			PositionBand:       positionBelowFold,
			RelatedResourceIDs: []string{"speaker-1", "speaker-2", "speaker-3", "speaker-4"},
		},
	}, resources, nil)

	if finding == nil {
		t.Fatal("expected repeated gallery finding")
	}
	if !strings.Contains(strings.ToLower(finding.Summary), "ya usan lazy loading") {
		t.Fatalf("expected lazy-majority summary, got %q", finding.Summary)
	}
	if !strings.Contains(strings.Join(finding.Evidence, " "), `Lazy loading ya presente en la mayoría`) {
		t.Fatalf("expected lazy-majority evidence, got %#v", finding.Evidence)
	}
}

func TestBuildAnalysisPrioritizesDominantImageOutlier(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:            "blog-jpeg",
			URL:           "https://cdn.example.com/img/blog/post-1.jpg",
			Type:          "image",
			MIMEType:      "image/jpeg",
			Hostname:      "cdn.example.com",
			Party:         partyFirst,
			Bytes:         3_967_443,
			NaturalWidth:  3024,
			NaturalHeight: 4032,
			BoundingBox:   &BoundingBox{X: 90, Y: 4303, Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "blog-avif",
			URL:           "https://cdn.example.com/img/blog/post-2.avif",
			Type:          "image",
			MIMEType:      "image/avif",
			Hostname:      "cdn.example.com",
			Party:         partyFirst,
			Bytes:         532_665,
			NaturalWidth:  1964,
			NaturalHeight: 2455,
			BoundingBox:   &BoundingBox{X: 400, Y: 4303, Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "blog-webp",
			URL:           "https://cdn.example.com/img/blog/post-3.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "cdn.example.com",
			Party:         partyFirst,
			Bytes:         179_274,
			NaturalWidth:  1964,
			NaturalHeight: 2455,
			BoundingBox:   &BoundingBox{X: 710, Y: 4303, Width: 288, Height: 114},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:               "analytics",
			URL:              "https://www.googletagmanager.com/gtag/js?id=G-123",
			Type:             "script",
			MIMEType:         "application/javascript",
			Hostname:         "www.googletagmanager.com",
			Party:            partyThird,
			Bytes:            174_419,
			IsThirdPartyTool: true,
			ThirdPartyKind:   thirdPartyAnalytics,
		},
		{
			ID:               "social",
			URL:              "https://platform.linkedin.com/in.js",
			Type:             "script",
			MIMEType:         "application/javascript",
			Hostname:         "platform.linkedin.com",
			Party:            partyThird,
			Bytes:            384_000,
			IsThirdPartyTool: true,
			ThirdPartyKind:   thirdPartySocial,
		},
	}
	groups := []ResourceGroup{
		{
			ID:                 "group-blog",
			Kind:               groupKindRepeatedGallery,
			Label:              "Miniaturas del blog",
			TotalBytes:         4_679_382,
			ResourceCount:      3,
			PositionBand:       positionBelowFold,
			RelatedResourceIDs: []string{"blog-jpeg", "blog-avif", "blog-webp"},
		},
		{
			ID:                 "group-analytics",
			Kind:               groupKindThirdParty,
			Label:              "Cluster de analítica",
			TotalBytes:         174_419,
			ResourceCount:      5,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"analytics"},
		},
		{
			ID:                 "group-social",
			Kind:               groupKindThirdParty,
			Label:              "Cluster social",
			TotalBytes:         384_000,
			ResourceCount:      11,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"social"},
		},
	}

	analysis := buildAnalysis(resources, PerformanceMetrics{
		LCPMS:           348,
		LCPResourceTag:  "p",
		LCPSelectorHint: "p.hero-copy",
	}, groups)
	if len(analysis.Findings) == 0 || analysis.Findings[0].ID != "dominant_image_overdelivery" {
		t.Fatalf("expected dominant image finding to lead, got %#v", analysis.Findings)
	}
	if !hasFinding(analysis.Findings, "third_party_social_overhead") {
		t.Fatalf("expected social cluster finding, got %#v", analysis.Findings)
	}
}

func TestEstimateResourceSavingsUsesImageFormatAndOverdelivery(t *testing.T) {
	noOverdelivery := enrichedResource{
		Type:          "image",
		MIMEType:      "image/webp",
		Bytes:         100_000,
		NaturalWidth:  800,
		NaturalHeight: 450,
		BoundingBox:   &BoundingBox{Width: 400, Height: 225},
	}
	if got := estimateResourceSavings(noOverdelivery); got != 20_000 {
		t.Fatalf("expected base webp savings, got %d", got)
	}

	strongOverdelivery := enrichedResource{
		Type:          "image",
		MIMEType:      "image/webp",
		Bytes:         100_000,
		NaturalWidth:  1920,
		NaturalHeight: 1080,
		BoundingBox:   &BoundingBox{Width: 640, Height: 360},
	}
	if got := estimateResourceSavings(strongOverdelivery); got != 50_000 {
		t.Fatalf("expected responsive overdelivery savings, got %d", got)
	}

	cappedOverdelivery := enrichedResource{
		Type:          "image",
		MIMEType:      "image/webp",
		Bytes:         100_000,
		NaturalWidth:  1920,
		NaturalHeight: 1080,
		BoundingBox:   &BoundingBox{Width: 160, Height: 90},
	}
	if got := estimateResourceSavings(cappedOverdelivery); got != 50_000 {
		t.Fatalf("expected capped image savings, got %d", got)
	}

	legacyPNG := enrichedResource{
		Type:          "image",
		MIMEType:      "image/png",
		Bytes:         100_000,
		NaturalWidth:  1920,
		NaturalHeight: 1080,
		BoundingBox:   &BoundingBox{Width: 160, Height: 90},
	}
	if got := estimateResourceSavings(legacyPNG); got != 60_000 {
		t.Fatalf("expected legacy image cap, got %d", got)
	}
}

func TestBuildAnalysisDoesNotDuplicateHeavyMediaForMixedRepeatedGallery(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:            "card-1",
			URL:           "https://example.com/courses/course-1.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         220_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 0, Y: 700, Width: 414, Height: 233},
			VisibleRatio:  0.86,
		},
		{
			ID:            "card-2",
			URL:           "https://example.com/courses/course-2.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         210_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 420, Y: 950, Width: 414, Height: 233},
		},
		{
			ID:            "card-3",
			URL:           "https://example.com/courses/course-3.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         205_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 840, Y: 2000, Width: 414, Height: 233},
		},
	}

	annotated, groups := enrichResourcesForAnalysis(resources, PerformanceMetrics{}, 1440, 900)
	for index, resource := range annotated {
		if resource.VisualRole != visualRoleRepeatedCard {
			t.Fatalf("expected repeated card visual role for resource %d, got %s", index, resource.VisualRole)
		}
	}

	analysis := buildAnalysis(annotated, PerformanceMetrics{}, groups)
	if !hasFinding(analysis.Findings, "repeated_gallery_overdelivery") {
		t.Fatal("expected repeated gallery finding")
	}
	if hasFinding(analysis.Findings, "heavy_above_fold_media") {
		t.Fatal("did not expect duplicate heavy above fold finding for repeated gallery members")
	}
}

func TestRankVampireResourcesPrefersRepeatedGalleryCardOverDeepAvatar(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:            "card-1",
			URL:           "https://example.com/courses/course-1.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         220_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 0, Y: 950, Width: 414, Height: 233},
		},
		{
			ID:            "card-2",
			URL:           "https://example.com/courses/course-2.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         210_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 420, Y: 970, Width: 414, Height: 233},
		},
		{
			ID:            "card-3",
			URL:           "https://example.com/courses/course-3.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         205_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 840, Y: 990, Width: 414, Height: 233},
		},
		{
			ID:            "avatar",
			URL:           "https://example.com/teachers/avatar.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         18_000,
			NaturalWidth:  500,
			NaturalHeight: 500,
			VisibleRatio:  1,
			BoundingBox:   &BoundingBox{X: 0, Y: 3100, Width: 40, Height: 40},
		},
	}

	annotated, groups := enrichResourcesForAnalysis(resources, PerformanceMetrics{}, 1440, 900)
	ranked, _ := rankVampireResources(annotated, groups, nil, sumBytes(annotated))
	if len(ranked) < 2 {
		t.Fatalf("expected at least 2 ranked resources, got %d", len(ranked))
	}
	topLimit := minInt(len(ranked), 3)
	foundCourseCard := false
	for _, resource := range ranked[:topLimit] {
		if resource.ID == "card-1" || resource.ID == "card-2" || resource.ID == "card-3" {
			foundCourseCard = true
			break
		}
	}
	if !foundCourseCard {
		t.Fatalf("expected a course card near the top of the ranking, got %#v", ranked)
	}
	for index, resource := range ranked {
		if resource.ID == "avatar" && index == 0 {
			t.Fatalf("expected deep avatar to lose against repeated gallery cards, got %#v", ranked)
		}
	}
}

func TestBuildAnalysisExcludesDeepAssetsFromAboveFoldVisualBytesEvenIfVisibleRatioIsSet(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:            "avatar",
			URL:           "https://example.com/teachers/avatar.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         18_000,
			NaturalWidth:  500,
			NaturalHeight: 500,
			VisibleRatio:  1,
			BoundingBox:   &BoundingBox{X: 0, Y: 3100, Width: 40, Height: 40},
		},
	}

	annotated, groups := enrichResourcesForAnalysis(resources, PerformanceMetrics{}, 1440, 900)
	if annotated[0].PositionBand != positionBelowFold {
		t.Fatalf("expected deep avatar to be below fold, got %s", annotated[0].PositionBand)
	}
	analysis := buildAnalysis(annotated, PerformanceMetrics{}, groups)
	if analysis.Summary.AboveFoldVisualBytes != 0 {
		t.Fatalf("expected deep asset to stay out of above fold visual bytes, got %d", analysis.Summary.AboveFoldVisualBytes)
	}
}

func TestRankVampireResourcesPromotesVisualDiversityAndCapsClusters(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:           "font-semibold",
			URL:          "https://example.com/fonts/inter-semibold.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        119_000,
			PositionBand: positionUnknown,
		},
		{
			ID:           "font-bold",
			URL:          "https://example.com/fonts/inter-bold.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        118_000,
			PositionBand: positionUnknown,
		},
		{
			ID:           "font-medium",
			URL:          "https://example.com/fonts/inter-medium.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        117_000,
			PositionBand: positionUnknown,
		},
		{
			ID:           "font-regular",
			URL:          "https://example.com/fonts/inter-regular.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        116_000,
			PositionBand: positionUnknown,
		},
		{
			ID:               "analytics",
			URL:              "https://us-assets.i.posthog.com/static/posthog-recorder.js",
			Type:             "script",
			Hostname:         "us-assets.i.posthog.com",
			Party:            partyThird,
			Bytes:            90_000,
			IsThirdPartyTool: true,
			ThirdPartyKind:   thirdPartyAnalytics,
			PositionBand:     positionUnknown,
		},
		{
			ID:            "card-1",
			URL:           "https://example.com/courses/course-1.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         320_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 0, Y: 700, Width: 414, Height: 233},
			PositionBand:  positionAboveFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "card-2",
			URL:           "https://example.com/courses/course-2.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         300_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 420, Y: 950, Width: 414, Height: 233},
			PositionBand:  positionNearFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "card-3",
			URL:           "https://example.com/courses/course-3.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         290_000,
			NaturalWidth:  1920,
			NaturalHeight: 1080,
			BoundingBox:   &BoundingBox{X: 840, Y: 1200, Width: 414, Height: 233},
			PositionBand:  positionNearFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "hero",
			URL:           "https://example.com/hero.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         180_000,
			NaturalWidth:  1280,
			NaturalHeight: 720,
			BoundingBox:   &BoundingBox{X: 0, Y: 0, Width: 960, Height: 420},
			PositionBand:  positionAboveFold,
			VisualRole:    visualRoleHeroMedia,
		},
	}

	groups := []ResourceGroup{
		{
			ID:                 "group-cards",
			Kind:               groupKindRepeatedGallery,
			Label:              "Grid de tarjetas",
			TotalBytes:         910_000,
			ResourceCount:      3,
			PositionBand:       "mixed",
			RelatedResourceIDs: []string{"card-1", "card-2", "card-3"},
		},
		{
			ID:                 "group-fonts",
			Kind:               groupKindFontCluster,
			Label:              "Stack tipográfico",
			TotalBytes:         470_000,
			ResourceCount:      4,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"font-semibold", "font-bold", "font-medium", "font-regular"},
		},
		{
			ID:                 "group-analytics",
			Kind:               groupKindThirdParty,
			Label:              "Cluster de analítica",
			TotalBytes:         90_000,
			ResourceCount:      1,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"analytics"},
		},
	}

	ranked, warnings := rankVampireResources(resources, groups, nil, sumBytes(resources))
	if len(ranked) != 5 {
		t.Fatalf("expected 5 ranked resources, got %d", len(ranked))
	}

	visualCount := countVisualResources(ranked)
	if visualCount < 2 {
		t.Fatalf("expected at least 2 visually mapped vampires, got %d (%#v)", visualCount, ranked)
	}

	fontCount := 0
	analyticsCount := 0
	repeatedCards := 0
	for _, resource := range ranked {
		if resource.Type == "font" {
			fontCount++
		}
		if resource.IsThirdPartyTool && resource.ThirdPartyKind == thirdPartyAnalytics {
			analyticsCount++
		}
		if resource.VisualRole == visualRoleRepeatedCard {
			repeatedCards++
		}
	}
	if fontCount > 1 {
		t.Fatalf("expected font cluster to be capped at 1 representative, got %d (%#v)", fontCount, ranked)
	}
	if analyticsCount > 1 {
		t.Fatalf("expected analytics cluster to be capped at 1 representative, got %d (%#v)", analyticsCount, ranked)
	}
	if repeatedCards < 1 {
		t.Fatalf("expected at least one repeated card vampire, got %#v", ranked)
	}
	if containsWarning(warnings, "The heaviest resources could not be mapped to visible DOM boxes.") {
		t.Fatalf("did not expect unmapped warning when visual candidates exist, got %#v", warnings)
	}
}

func TestRankVampireResourcesSkipsLowImpactVisualFiller(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:           "font-semibold",
			URL:          "https://example.com/fonts/inter-semibold.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        119_000,
			PositionBand: positionUnknown,
		},
		{
			ID:           "font-bold",
			URL:          "https://example.com/fonts/inter-bold.woff2",
			Type:         "font",
			Hostname:     "example.com",
			Party:        partyFirst,
			Bytes:        118_000,
			PositionBand: positionUnknown,
		},
		{
			ID:               "analytics",
			URL:              "https://us-assets.i.posthog.com/static/posthog-recorder.js",
			Type:             "script",
			Hostname:         "us-assets.i.posthog.com",
			Party:            partyThird,
			Bytes:            90_000,
			IsThirdPartyTool: true,
			ThirdPartyKind:   thirdPartyAnalytics,
			PositionBand:     positionUnknown,
		},
		{
			ID:            "card-1",
			URL:           "https://example.com/courses/course-1.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         320_000,
			NaturalWidth:  3840,
			NaturalHeight: 2160,
			BoundingBox:   &BoundingBox{X: 0, Y: 648, Width: 414, Height: 233},
			PositionBand:  positionAboveFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "card-2",
			URL:           "https://example.com/courses/course-2.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         290_000,
			NaturalWidth:  3840,
			NaturalHeight: 2160,
			BoundingBox:   &BoundingBox{X: 420, Y: 648, Width: 414, Height: 233},
			PositionBand:  positionAboveFold,
			VisualRole:    visualRoleRepeatedCard,
		},
		{
			ID:            "avatar",
			URL:           "https://example.com/teachers/avatar.webp",
			Type:          "image",
			MIMEType:      "image/webp",
			Hostname:      "example.com",
			Party:         partyFirst,
			Bytes:         18_000,
			NaturalWidth:  500,
			NaturalHeight: 500,
			BoundingBox:   &BoundingBox{X: 600, Y: 3100, Width: 40, Height: 40},
			PositionBand:  positionBelowFold,
			VisualRole:    visualRoleBelowFoldMedia,
		},
	}

	groups := []ResourceGroup{
		{
			ID:                 "group-cards",
			Kind:               groupKindRepeatedGallery,
			Label:              "Grid de tarjetas",
			TotalBytes:         610_000,
			ResourceCount:      2,
			PositionBand:       "mixed",
			RelatedResourceIDs: []string{"card-1", "card-2"},
		},
		{
			ID:                 "group-fonts",
			Kind:               groupKindFontCluster,
			Label:              "Stack tipográfico",
			TotalBytes:         237_000,
			ResourceCount:      2,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"font-semibold", "font-bold"},
		},
		{
			ID:                 "group-analytics",
			Kind:               groupKindThirdParty,
			Label:              "Cluster de analítica",
			TotalBytes:         90_000,
			ResourceCount:      1,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"analytics"},
		},
	}

	ranked, _ := rankVampireResources(resources, groups, nil, sumBytes(resources))
	if len(ranked) != 4 {
		t.Fatalf("expected low-impact filler to be skipped, got %d vampires (%#v)", len(ranked), ranked)
	}
	for _, resource := range ranked {
		if resource.ID == "avatar" {
			t.Fatalf("expected low-impact avatar to stay out of vampires, got %#v", ranked)
		}
	}
}

func TestRankVampireResourcesPromotesVisibleResponsiveAnchor(t *testing.T) {
	resources := []enrichedResource{
		{ID: "font", URL: "https://example.com/fonts/inter.woff2", Type: "font", Party: partyFirst, Bytes: 150_000, PositionBand: positionUnknown},
		{ID: "analytics", URL: "https://www.googletagmanager.com/gtag/js?id=G-123", Type: "script", Party: partyThird, Bytes: 95_000, IsThirdPartyTool: true, ThirdPartyKind: thirdPartyAnalytics, PositionBand: positionUnknown},
		{ID: "hero", URL: "https://example.com/hero.webp", Type: "image", MIMEType: "image/webp", Party: partyFirst, Bytes: 220_000, NaturalWidth: 1920, NaturalHeight: 1080, BoundingBox: &BoundingBox{X: 0, Y: 0, Width: 960, Height: 420}, PositionBand: positionAboveFold, VisualRole: visualRoleHeroMedia},
		{ID: "card-1", URL: "https://example.com/speakers/ana.webp", Type: "image", MIMEType: "image/webp", Party: partyFirst, Bytes: 130_000, NaturalWidth: 768, NaturalHeight: 1024, BoundingBox: &BoundingBox{X: 0, Y: 2200, Width: 284, Height: 300}, PositionBand: positionBelowFold, VisualRole: visualRoleRepeatedCard},
		{ID: "card-2", URL: "https://example.com/speakers/luis.webp", Type: "image", MIMEType: "image/webp", Party: partyFirst, Bytes: 125_000, NaturalWidth: 768, NaturalHeight: 1024, BoundingBox: &BoundingBox{X: 320, Y: 2200, Width: 284, Height: 300}, PositionBand: positionBelowFold, VisualRole: visualRoleRepeatedCard},
		{ID: "venue", URL: "https://example.com/img/venue.webp", Type: "image", MIMEType: "image/webp", Party: partyFirst, Bytes: 282_000, NaturalWidth: 2348, NaturalHeight: 1203, BoundingBox: &BoundingBox{X: 90, Y: 1200, Width: 596, Height: 396}, PositionBand: positionNearFold, VisualRole: visualRoleBelowFoldMedia},
	}
	groups := []ResourceGroup{
		{
			ID:                 "group-speakers",
			Kind:               groupKindRepeatedGallery,
			Label:              "Fotos de speakers",
			TotalBytes:         255_000,
			ResourceCount:      2,
			PositionBand:       positionBelowFold,
			RelatedResourceIDs: []string{"card-1", "card-2"},
		},
		{
			ID:                 "group-fonts",
			Kind:               groupKindFontCluster,
			Label:              "Stack tipográfico",
			TotalBytes:         150_000,
			ResourceCount:      1,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"font"},
		},
		{
			ID:                 "group-analytics",
			Kind:               groupKindThirdParty,
			Label:              "Cluster de analítica",
			TotalBytes:         95_000,
			ResourceCount:      1,
			PositionBand:       positionUnknown,
			RelatedResourceIDs: []string{"analytics"},
		},
	}
	findings := []AnalysisFinding{
		{
			ID:                 "responsive_image_overdelivery",
			Category:           "media",
			Severity:           "medium",
			Confidence:         "high",
			RelatedResourceIDs: []string{"venue"},
		},
	}

	ranked, _ := rankVampireResources(resources, groups, findings, sumBytes(resources))
	if len(ranked) == 0 {
		t.Fatal("expected ranked vampires")
	}
	foundVenue := false
	for _, resource := range ranked {
		if resource.ID == "venue" {
			foundVenue = true
			break
		}
	}
	if !foundVenue {
		t.Fatalf("expected visible responsive-image anchor to be promoted, got %#v", ranked)
	}
}

func TestBuildFontFindingDetectsIconFontDominance(t *testing.T) {
	finding := buildFontFinding([]enrichedResource{
		{
			ID:    "fa-solid",
			URL:   "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/webfonts/fa-solid-900.woff2",
			Type:  "font",
			Bytes: 150_000,
		},
		{
			ID:    "brand",
			URL:   "https://example.com/fonts/brand.woff2",
			Type:  "font",
			Bytes: 120_000,
		},
	}, AnalysisSummary{
		FontBytes:    270_000,
		FontRequests: 2,
	})

	if finding == nil {
		t.Fatal("expected font finding")
	}
	if finding.Title != "Recorta el coste de la fuente de iconos" {
		t.Fatalf("expected icon-font title, got %q", finding.Title)
	}
	if !strings.Contains(strings.ToLower(finding.Summary), "svgs individuales") {
		t.Fatalf("expected icon-font summary, got %q", finding.Summary)
	}
}

func TestRankVampireResourcesWarnsWhenNoVisualCandidatesExist(t *testing.T) {
	resources := []enrichedResource{
		{ID: "font-1", URL: "https://example.com/font-1.woff2", Type: "font", Party: partyFirst, Bytes: 100_000},
		{ID: "font-2", URL: "https://example.com/font-2.woff2", Type: "font", Party: partyFirst, Bytes: 90_000},
		{
			ID:               "analytics",
			URL:              "https://us-assets.i.posthog.com/static/posthog.js",
			Type:             "script",
			Party:            partyThird,
			Bytes:            80_000,
			IsThirdPartyTool: true,
			ThirdPartyKind:   thirdPartyAnalytics,
		},
	}

	ranked, warnings := rankVampireResources(resources, nil, nil, sumBytes(resources))
	if len(ranked) == 0 {
		t.Fatal("expected ranked resources")
	}
	if !containsWarning(warnings, "The heaviest resources could not be mapped to visible DOM boxes.") {
		t.Fatalf("expected unmapped warning, got %#v", warnings)
	}
}

func TestBuildResponsiveImageFindingAvoidsCallingResponsiveMarkupNonResponsive(t *testing.T) {
	finding := buildResponsiveImageFinding([]enrichedResource{
		{
			ID:              "hero-card",
			Type:            "image",
			MIMEType:        "image/webp",
			Bytes:           180_000,
			NaturalWidth:    2400,
			NaturalHeight:   1350,
			ResponsiveImage: true,
			BoundingBox:     &BoundingBox{Width: 480, Height: 270},
		},
	}, nil, "")

	if finding == nil {
		t.Fatal("expected responsive image finding")
	}
	if strings.Contains(strings.ToLower(finding.Title), "no responsive") {
		t.Fatalf("expected neutral title for responsive markup, got %q", finding.Title)
	}
	if strings.Contains(strings.ToLower(finding.Summary), "no se detectan variantes responsive") {
		t.Fatalf("expected summary to acknowledge responsive markup, got %q", finding.Summary)
	}
}

func hasFinding(findings []AnalysisFinding, id string) bool {
	for _, finding := range findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}

func findFinding(findings []AnalysisFinding, id string) *AnalysisFinding {
	for index := range findings {
		if findings[index].ID == id {
			return &findings[index]
		}
	}
	return nil
}

func containsWarning(warnings []string, target string) bool {
	for _, warning := range warnings {
		if warning == target {
			return true
		}
	}
	return false
}
