package scanner

import (
	"testing"

	"github.com/tronchos/wattless/server/internal/insights"
)

func TestSanitizeTopActionsKeepsExactVisibleMatchesFromFinding(t *testing.T) {
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
	if len(sanitized[0].RelatedResourceIDs) != 1 {
		t.Fatalf("expected 1 related resource, got %#v", sanitized[0].RelatedResourceIDs)
	}
	if sanitized[0].RelatedResourceIDs[0] != "visible-card" {
		t.Fatalf("expected visible repeated card match, got %#v", sanitized[0].RelatedResourceIDs)
	}
}

func TestSanitizeTopActionsLeavesRepeatedGalleryActionUnboundWithoutVisibleMatch(t *testing.T) {
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
	if len(sanitized[0].RelatedResourceIDs) != 0 {
		t.Fatalf("expected gallery finding to stay unbound without exact visible match, got %#v", sanitized[0].RelatedResourceIDs)
	}
}

func TestSanitizeTopActionsKeepsResponsiveImageActionUnboundWithoutVisibleMatch(t *testing.T) {
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
	if len(sanitized[0].RelatedResourceIDs) != 0 {
		t.Fatalf("expected responsive finding to stay unbound without exact visible match, got %#v", sanitized[0].RelatedResourceIDs)
	}
}

func TestSanitizeTopActionsLeavesAnalyticsActionUnboundWithoutVisibleMatch(t *testing.T) {
	actions := []insights.TopAction{
		{
			ID:                 "act-1",
			RelatedFindingID:   "third_party_analytics_overhead",
			RelatedResourceIDs: []string{"analytics-script"},
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
	if len(sanitized[0].RelatedResourceIDs) != 0 {
		t.Fatalf("expected analytics action to stay unbound without exact visible match, got %#v", sanitized[0].RelatedResourceIDs)
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
			ID:                 "act-1",
			RelatedFindingID:   "repeated_gallery_overdelivery",
			RelatedResourceIDs: []string{"visible-card"},
			Reason:             "Optimiza el grid repetido con miniaturas más pequeñas.",
			Confidence:         "high",
			LikelyLCPImpact:    "low",
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
