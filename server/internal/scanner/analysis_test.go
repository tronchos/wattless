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
	}, resources)

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
			URL:           "https://example.com/teachers/midu.webp",
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

	annotated, _ := enrichResourcesForAnalysis(resources, PerformanceMetrics{}, 1440, 900)
	ranked, _ := rankVampireResources(annotated, sumBytes(annotated))
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

func TestBuildAnalysisExcludesDeepAssetsFromAboveFoldBytesEvenIfVisibleRatioIsSet(t *testing.T) {
	resources := []enrichedResource{
		{
			ID:            "avatar",
			URL:           "https://example.com/teachers/midu.webp",
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
	if analysis.Summary.AboveFoldBytes != 0 {
		t.Fatalf("expected deep asset to stay out of above fold bytes, got %d", analysis.Summary.AboveFoldBytes)
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
