package scanner

import (
	"strings"
	"testing"
)

func TestNormalizeTypePrefersURLHints(t *testing.T) {
	got := normalizeType("Other", "text/html", "https://example.com/favicon.ico")
	if got != "image" {
		t.Fatalf("expected image, got %s", got)
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

func TestRankVampireResourcesKeepsFailedRequestsWhenTheyTransferBytes(t *testing.T) {
	resources := []enrichedResource{
		{ID: "req-1", URL: "https://example.com/favicon.ico", Bytes: 9000, StatusCode: 404, Failed: true, Party: partyFirst, Type: "image"},
		{ID: "req-2", URL: "https://example.com/app.js", Bytes: 5000, StatusCode: 200, Party: partyFirst, Type: "script"},
	}

	ranked, warnings := rankVampireResources(resources, 14_000)
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
	if got := classifyPositionBand(&BoundingBox{Y: 100}, 900); got != positionAboveFold {
		t.Fatalf("expected above fold, got %s", got)
	}
	if got := classifyPositionBand(&BoundingBox{Y: 1200}, 900); got != positionNearFold {
		t.Fatalf("expected near fold, got %s", got)
	}
	if got := classifyPositionBand(&BoundingBox{Y: 2400}, 900); got != positionBelowFold {
		t.Fatalf("expected below fold, got %s", got)
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
	if !hasFinding(analysis.Findings, "main_thread_pressure") {
		t.Fatal("expected main thread finding")
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
		{ID: "card-1", Type: "image", Bytes: 220_000},
		{ID: "card-2", Type: "image", Bytes: 210_000},
		{ID: "card-3", Type: "image", Bytes: 205_000},
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
	}, resources)

	if finding == nil {
		t.Fatal("expected repeated gallery finding")
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
