package scanner

import (
	"context"
	"errors"
	"testing"

	"github.com/tronchos/wattless/server/internal/insights"
)

type stubInsightsProvider struct {
	name   string
	result insights.ProviderResult
	err    error
}

func (s stubInsightsProvider) Name() string { return s.name }

func (s stubInsightsProvider) SuggestResource(resource insights.ResourceContext) string { return "" }

func (s stubInsightsProvider) SummarizeReport(ctx context.Context, report insights.ReportContext) (insights.ProviderResult, error) {
	return s.result, s.err
}

func TestSanitizeTopActionsPreservesFactualIDsAndAddsVisibleSubset(t *testing.T) {
	actions := []insights.TopAction{
		{
			ID:                 "act-1",
			RelatedFindingID:   "repeated_gallery_overdelivery",
			RelatedResourceIDs: []string{"missing-1", "visible-card"},
		},
	}
	findings := []AnalysisFinding{
		{
			ID:                 "repeated_gallery_overdelivery",
			RelatedResourceIDs: []string{"missing-1", "visible-card"},
		},
	}
	vampires := []ResourceSummary{
		{
			ID:         "visible-card",
			Type:       "image",
			VisualRole: visualRoleRepeatedCard,
		},
		{
			ID:               "visible-analytics",
			Type:             "script",
			IsThirdPartyTool: true,
			ThirdPartyKind:   thirdPartyAnalytics,
		},
	}

	sanitized := sanitizeTopActions(actions, findings, vampires)
	if len(sanitized) != 1 {
		t.Fatalf("expected 1 action, got %d", len(sanitized))
	}
	if len(sanitized[0].RelatedResourceIDs) != 2 {
		t.Fatalf("expected factual ids to be preserved, got %#v", sanitized[0].RelatedResourceIDs)
	}
	if len(sanitized[0].VisibleRelatedResourceIDs) != 1 {
		t.Fatalf("expected 1 visible related resource, got %#v", sanitized[0].VisibleRelatedResourceIDs)
	}
	if sanitized[0].VisibleRelatedResourceIDs[0] != "visible-card" {
		t.Fatalf("expected visible repeated card match, got %#v", sanitized[0].VisibleRelatedResourceIDs)
	}
}

func TestMakeInsightAnalysisKeepsGroupKindWireStrings(t *testing.T) {
	analysis := Analysis{
		ResourceGroups: []ResourceGroup{
			{
				ID:                 "group-1",
				Kind:               GroupKindRepeatedGallery,
				Label:              "Grid de tarjetas",
				PositionBand:       PositionBandMixed,
				RelatedResourceIDs: []string{"img-1"},
			},
		},
	}

	result := makeInsightAnalysis(analysis)
	if len(result.ResourceGroups) != 1 {
		t.Fatalf("expected 1 resource group, got %d", len(result.ResourceGroups))
	}
	if result.ResourceGroups[0].Kind != "repeated_gallery" {
		t.Fatalf("expected string kind in insights bridge, got %q", result.ResourceGroups[0].Kind)
	}
	if result.ResourceGroups[0].PositionBand != "mixed" {
		t.Fatalf("expected string position band in insights bridge, got %q", result.ResourceGroups[0].PositionBand)
	}
}

func TestMakeInsightResourceKeepsVisualRoleWireStrings(t *testing.T) {
	resource := ResourceSummary{
		ID:             "hero",
		PositionBand:   PositionBandAboveFold,
		VisualRole:     VisualRoleHeroMedia,
		ThirdPartyKind: ThirdPartyKindAnalytics,
	}

	result := makeInsightResource(resource)
	if result.PositionBand != "above_fold" {
		t.Fatalf("expected string position band in resource bridge, got %q", result.PositionBand)
	}
	if result.VisualRole != "hero_media" {
		t.Fatalf("expected string visual role in resource bridge, got %q", result.VisualRole)
	}
	if result.ThirdPartyKind != "analytics" {
		t.Fatalf("expected string third-party kind in resource bridge, got %q", result.ThirdPartyKind)
	}
}

func TestSanitizeTopActionsLeavesVisibleSubsetEmptyWithoutMatch(t *testing.T) {
	actions := []insights.TopAction{
		{
			ID:                 "act-1",
			RelatedFindingID:   "repeated_gallery_overdelivery",
			RelatedResourceIDs: []string{"missing-image"},
		},
	}
	findings := []AnalysisFinding{
		{
			ID:                 "repeated_gallery_overdelivery",
			RelatedResourceIDs: []string{"missing-image"},
		},
	}
	vampires := []ResourceSummary{
		{
			ID:   "visible-avatar",
			Type: "image",
		},
	}

	sanitized := sanitizeTopActions(actions, findings, vampires)
	if len(sanitized) != 1 {
		t.Fatalf("expected 1 action, got %d", len(sanitized))
	}
	if len(sanitized[0].RelatedResourceIDs) != 1 {
		t.Fatalf("expected factual gallery ids to remain, got %#v", sanitized[0].RelatedResourceIDs)
	}
	if len(sanitized[0].VisibleRelatedResourceIDs) != 0 {
		t.Fatalf("expected visible subset to stay empty without exact match, got %#v", sanitized[0].VisibleRelatedResourceIDs)
	}
}

func TestSanitizeTopActionsKeepsResponsiveImageFactualIDsWithoutVisibleMatch(t *testing.T) {
	actions := []insights.TopAction{
		{
			ID:                 "act-1",
			RelatedFindingID:   "responsive_image_overdelivery",
			RelatedResourceIDs: []string{"missing-image"},
		},
	}
	findings := []AnalysisFinding{
		{
			ID:                 "responsive_image_overdelivery",
			RelatedResourceIDs: []string{"missing-image"},
		},
	}
	vampires := []ResourceSummary{
		{
			ID:            "visible-image",
			Type:          "image",
			NaturalWidth:  1920,
			NaturalHeight: 1080,
		},
	}

	sanitized := sanitizeTopActions(actions, findings, vampires)
	if len(sanitized) != 1 {
		t.Fatalf("expected 1 action, got %d", len(sanitized))
	}
	if len(sanitized[0].RelatedResourceIDs) != 1 {
		t.Fatalf("expected responsive finding ids to remain factual, got %#v", sanitized[0].RelatedResourceIDs)
	}
	if len(sanitized[0].VisibleRelatedResourceIDs) != 0 {
		t.Fatalf("expected visible subset to stay empty without exact visible match, got %#v", sanitized[0].VisibleRelatedResourceIDs)
	}
}

func TestSanitizeTopActionsFallsBackToFindingIDsWhenActionOmitsThem(t *testing.T) {
	actions := []insights.TopAction{
		{
			ID:               "act-1",
			RelatedFindingID: "third_party_analytics_overhead",
		},
	}
	findings := []AnalysisFinding{
		{
			ID:                 "third_party_analytics_overhead",
			RelatedResourceIDs: []string{"analytics-script"},
		},
	}
	vampires := []ResourceSummary{
		{
			ID:   "visible-card",
			Type: "image",
		},
	}

	sanitized := sanitizeTopActions(actions, findings, vampires)
	if len(sanitized) != 1 {
		t.Fatalf("expected 1 action, got %d", len(sanitized))
	}
	if len(sanitized[0].RelatedResourceIDs) != 1 || sanitized[0].RelatedResourceIDs[0] != "analytics-script" {
		t.Fatalf("expected finding ids to backfill the action, got %#v", sanitized[0].RelatedResourceIDs)
	}
	if len(sanitized[0].VisibleRelatedResourceIDs) != 0 {
		t.Fatalf("expected analytics action to stay visually unbound without exact match, got %#v", sanitized[0].VisibleRelatedResourceIDs)
	}
}

func TestAttachAssetInsightsIgnoresInvalidDraftsAndFallsBackPerAsset(t *testing.T) {
	vampires := []ResourceSummary{
		{
			ID:                    "visible-card",
			URL:                   "https://example.com/courses/course-1.webp",
			Type:                  "image",
			Bytes:                 220_000,
			EstimatedSavingsBytes: 110_000,
			PositionBand:          "mixed",
			VisualRole:            visualRoleRepeatedCard,
		},
		{
			ID:                    "visible-analytics",
			URL:                   "https://us.i.posthog.com/static/array.js",
			Type:                  "script",
			Bytes:                 95_000,
			EstimatedSavingsBytes: 40_000,
			PositionBand:          "unknown",
			IsThirdPartyTool:      true,
			ThirdPartyKind:        thirdPartyAnalytics,
		},
	}
	analysis := Analysis{
		Findings: []AnalysisFinding{
			{
				ID:                    "repeated_gallery_overdelivery",
				Category:              "media",
				Severity:              "medium",
				Confidence:            "high",
				Title:                 "Galería repetida sobredimensionada",
				Summary:               "La galería repetida suma demasiado peso para el valor que aporta.",
				Evidence:              []string{"El grupo suma 600 KB."},
				EstimatedSavingsBytes: 180_000,
				RelatedResourceIDs:    []string{"visible-card"},
			},
		},
	}
	actions := []insights.TopAction{
		{
			ID:                        "act-1",
			RelatedFindingID:          "repeated_gallery_overdelivery",
			RelatedResourceIDs:        []string{"visible-card"},
			VisibleRelatedResourceIDs: []string{"visible-card"},
			Reason:                    "Optimiza el grid repetido con miniaturas más pequeñas.",
			Confidence:                "high",
			LikelyLCPImpact:           "low",
			RecommendedFix: &insights.RecommendedFix{
				Summary:       "Fix de catálogo repetido.",
				OptimizedCode: "<Image />",
			},
		},
	}
	drafts := []insights.AssetInsightDraft{
		{
			ResourceID:        "missing-id",
			Title:             "No debería sobrevivir",
			ShortProblem:      "draft inválido",
			WhyItMatters:      "draft inválido",
			RecommendedAction: "draft inválido",
			Confidence:        "high",
			LikelyLCPImpact:   "low",
		},
		{
			ResourceID:        "visible-analytics",
			Title:             "Analítica con ruido evitable",
			ShortProblem:      "Este tercero añade bytes antes de aportar valor visible.",
			WhyItMatters:      "Suma variabilidad al arranque.",
			RecommendedAction: "Retrásala hasta interacción.",
			Confidence:        "high",
			LikelyLCPImpact:   "low",
			Evidence:          []string{"Carga 95 KB de analítica."},
			Scope:             "asset",
		},
	}

	enriched := attachAssetInsights(vampires, analysis, actions, drafts)
	if enriched[0].AssetInsight.Source != "rule_based" {
		t.Fatalf("expected fallback rule_based source, got %q", enriched[0].AssetInsight.Source)
	}
	if enriched[0].AssetInsight.RelatedActionID != "act-1" {
		t.Fatalf("expected related action id, got %q", enriched[0].AssetInsight.RelatedActionID)
	}
	if enriched[0].AssetInsight.RecommendedFix == nil {
		t.Fatal("expected fallback fix to be preserved")
	}
	if enriched[1].AssetInsight.Source != "gemini" {
		t.Fatalf("expected gemini source for valid provider draft, got %q", enriched[1].AssetInsight.Source)
	}
	if enriched[1].AssetInsight.Title != "Analítica con ruido evitable" {
		t.Fatalf("expected gemini title, got %q", enriched[1].AssetInsight.Title)
	}
}

func TestAttachAssetInsightsDoesNotLetUnrelatedAvatarInheritGalleryAction(t *testing.T) {
	vampires := []ResourceSummary{
		{
			ID:                    "avatar",
			URL:                   "https://example.com/teachers/avatar.webp",
			Type:                  "image",
			Bytes:                 18_000,
			EstimatedSavingsBytes: 12_000,
			PositionBand:          "below_fold",
			VisualRole:            visualRoleBelowFoldMedia,
			NaturalWidth:          500,
			NaturalHeight:         500,
		},
	}
	analysis := Analysis{
		Findings: []AnalysisFinding{
			{
				ID:                    "repeated_gallery_overdelivery",
				Category:              "media",
				Severity:              "medium",
				Confidence:            "high",
				Title:                 "Galería repetida sobredimensionada",
				Summary:               "La galería repetida suma demasiado peso para el valor que aporta.",
				Evidence:              []string{"El grupo suma 1.8 MB."},
				EstimatedSavingsBytes: 900_000,
				RelatedResourceIDs:    []string{"card-1", "card-2"},
			},
		},
	}
	actions := []insights.TopAction{
		{
			ID:                 "act-1",
			RelatedFindingID:   "repeated_gallery_overdelivery",
			RelatedResourceIDs: nil,
			Reason:             "Optimiza el grid repetido con miniaturas más pequeñas.",
			Confidence:         "high",
			LikelyLCPImpact:    "low",
			RecommendedFix: &insights.RecommendedFix{
				Summary:       "Fix de catálogo repetido.",
				OptimizedCode: "<Image />",
			},
		},
	}

	enriched := attachAssetInsights(vampires, analysis, actions, nil)
	if enriched[0].AssetInsight.RelatedActionID != "" {
		t.Fatalf("expected unrelated avatar to stay detached from gallery action, got %q", enriched[0].AssetInsight.RelatedActionID)
	}
	if enriched[0].AssetInsight.RecommendedFix != nil {
		t.Fatal("expected unrelated avatar to avoid inheriting gallery fix")
	}
}

func TestBuildReportContextUsesCurrentReportShape(t *testing.T) {
	service := &Service{}
	report := Report{
		URL:                   "https://example.com",
		Score:                 "B",
		TotalBytesTransferred: 321_000,
		CO2GramsPerVisit:      0.32,
		HostingIsGreen:        true,
		HostingVerdict:        "green",
		HostedBy:              "Green Host",
		SiteProfile: SiteProfile{
			FrameworkHint: "astro",
			Evidence:      []string{"astro marker"},
		},
		Summary: Summary{
			TotalRequests:         12,
			SuccessfulRequests:    12,
			FailedRequests:        0,
			FirstPartyBytes:       200_000,
			ThirdPartyBytes:       121_000,
			PotentialSavingsBytes: 80_000,
			VisualMappedVampires:  1,
		},
		Performance: PerformanceMetrics{
			LoadMS:                   1200,
			DOMContentLoadedMS:       600,
			ScriptResourceDurationMS: 200,
			LCPMS:                    1400,
			FCPMS:                    700,
			RenderMetricsComplete:    true,
			LongTasksTotalMS:         80,
			LongTasksCount:           2,
			LCPResourceURL:           "https://example.com/hero.webp",
			LCPResourceTag:           "img",
			LCPSelectorHint:          ".hero img",
			LCPSize:                  180_000,
		},
		Analysis: Analysis{
			Summary: AnalysisSummary{
				AboveFoldVisualBytes: 180_000,
				BelowFoldBytes:       90_000,
				RenderCriticalBytes:  190_000,
			},
			Findings: []AnalysisFinding{
				{
					ID:                 "render_lcp_candidate",
					Category:           "render",
					Severity:           "high",
					Confidence:         "high",
					Title:              "Hero pesada",
					Summary:            "La hero empuja el LCP.",
					RelatedResourceIDs: []string{"hero"},
				},
			},
		},
		VampireElements: []ResourceSummary{
			{
				ID:                    "hero",
				URL:                   "https://example.com/hero.webp",
				Type:                  "image",
				Bytes:                 180_000,
				EstimatedSavingsBytes: 70_000,
				PositionBand:          PositionBandAboveFold,
				VisualRole:            VisualRoleHeroMedia,
				ThirdPartyKind:        ThirdPartyKindUnknown,
			},
		},
	}

	result := service.BuildReportContext(report)
	if result.SiteProfile.FrameworkHint != "astro" {
		t.Fatalf("expected framework hint, got %#v", result.SiteProfile)
	}
	if len(result.TopResources) != 1 || result.TopResources[0].ID != "hero" {
		t.Fatalf("expected vampire elements to bridge into top resources, got %#v", result.TopResources)
	}
	if len(result.Analysis.Findings) != 1 || result.Analysis.Findings[0].ID != "render_lcp_candidate" {
		t.Fatalf("expected analysis findings in context, got %#v", result.Analysis.Findings)
	}
}

func TestGenerateInsightsUsesAIProviderOnly(t *testing.T) {
	service := &Service{
		ai: stubInsightsProvider{
			name: "gemini",
			result: insights.ProviderResult{
				Insights: insights.ScanInsights{
					Provider:         "gemini",
					ExecutiveSummary: "Resumen Gemini",
				},
			},
		},
	}

	result, err := service.GenerateInsights(context.Background(), Report{
		URL: "https://example.com",
	})
	if err != nil {
		t.Fatalf("expected ai insights, got error: %v", err)
	}
	if result.Insights.Provider != "gemini" {
		t.Fatalf("expected gemini provider, got %#v", result.Insights)
	}
}

func TestGenerateInsightsRejectsEmptyPayload(t *testing.T) {
	service := &Service{
		ai: stubInsightsProvider{
			name:   "gemini",
			result: insights.ProviderResult{},
		},
	}

	_, err := service.GenerateInsights(context.Background(), Report{
		URL: "https://example.com",
	})
	if !errors.Is(err, errEmptyAIInsights) {
		t.Fatalf("expected empty insights error, got %v", err)
	}
}

func TestApplyInsightsOverlaysOnlyInsightsFields(t *testing.T) {
	service := &Service{}
	report := Report{
		URL: "https://example.com",
		Insights: insights.ScanInsights{
			Provider:         "rule_based",
			ExecutiveSummary: "Resumen base",
			TopActions: []insights.TopAction{
				{
					ID:               "act-1",
					RelatedFindingID: "third_party_analytics_overhead",
				},
			},
		},
		VampireElements: []ResourceSummary{
			{
				ID:                    "analytics",
				URL:                   "https://us.i.posthog.com/static/array.js",
				Type:                  "script",
				Bytes:                 95_000,
				EstimatedSavingsBytes: 40_000,
				IsThirdPartyTool:      true,
				ThirdPartyKind:        thirdPartyAnalytics,
			},
		},
		Analysis: Analysis{
			Findings: []AnalysisFinding{
				{
					ID:                    "third_party_analytics_overhead",
					Category:              "third_party",
					Severity:              "medium",
					Confidence:            "high",
					Title:                 "Analítica temprana",
					Summary:               "Compite con el arranque.",
					EstimatedSavingsBytes: 40_000,
					RelatedResourceIDs:    []string{"analytics"},
				},
			},
		},
		Meta: Meta{
			ScannerVersion: "2026.03",
		},
	}

	service.ApplyInsights(&report, insights.ProviderResult{
		Insights: insights.ScanInsights{
			Provider:         "gemini",
			ExecutiveSummary: "Resumen Gemini",
			TopActions: []insights.TopAction{
				{
					ID:               "act-1",
					RelatedFindingID: "third_party_analytics_overhead",
					RelatedResourceIDs: []string{
						"analytics",
					},
				},
			},
		},
		AssetInsights: []insights.AssetInsightDraft{
			{
				ResourceID:        "analytics",
				Title:             "Analítica con ruido evitable",
				ShortProblem:      "Carga bytes antes de aportar valor.",
				WhyItMatters:      "Empuja la red inicial.",
				RecommendedAction: "Retrásala hasta interacción.",
				Confidence:        "high",
				LikelyLCPImpact:   "low",
				Scope:             "asset",
				Source:            "gemini",
			},
		},
	})

	if report.Insights.Provider != "gemini" {
		t.Fatalf("expected gemini summary after apply, got %#v", report.Insights)
	}
	if report.Meta.ScannerVersion != "2026.03" {
		t.Fatalf("expected metadata to remain untouched, got %#v", report.Meta)
	}
	if report.VampireElements[0].AssetInsight.Source != "hybrid" {
		t.Fatalf("expected asset insights to be enriched, got %#v", report.VampireElements[0].AssetInsight)
	}
}
