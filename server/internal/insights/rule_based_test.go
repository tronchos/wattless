package insights

import (
	"strings"
	"testing"
)

func TestBuildExecutiveSummaryAvoidsBelowFoldClaimForMixedGallery(t *testing.T) {
	report := ReportContext{
		Performance: PerformanceContext{
			LCPMS: 580,
		},
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				RepeatedGalleryBytes: 900_000,
			},
			ResourceGroups: []ResourceGroupContext{
				{
					ID:           "group-cards",
					Kind:         "repeated_gallery",
					TotalBytes:   900_000,
					PositionBand: "mixed",
				},
			},
		},
	}

	summary := buildExecutiveSummary(report)
	if strings.Contains(strings.ToLower(summary), "por debajo del fold") {
		t.Fatalf("expected conservative summary, got %q", summary)
	}
}

func TestBuildPitchLineAvoidsBelowFoldClaimForMixedGallery(t *testing.T) {
	report := ReportContext{
		Performance: PerformanceContext{
			LCPMS: 580,
		},
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				RepeatedGalleryBytes: 900_000,
			},
			ResourceGroups: []ResourceGroupContext{
				{
					ID:           "group-cards",
					Kind:         "repeated_gallery",
					TotalBytes:   900_000,
					PositionBand: "mixed",
				},
			},
		},
	}

	pitch := buildPitchLine(report)
	if strings.Contains(strings.ToLower(pitch), "bajo el fold") {
		t.Fatalf("expected conservative pitch, got %q", pitch)
	}
}

func TestBuildRuleBasedAssetInsightAvoidsBelowFoldClaimForMixedGalleryAsset(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:           "card-1",
			Type:         "image",
			Bytes:        220_000,
			PositionBand: "mixed",
			VisualRole:   "repeated_card_media",
		},
		[]AnalysisFindingContext{
			{
				ID:                    "repeated_gallery_overdelivery",
				Category:              "media",
				Severity:              "medium",
				Confidence:            "high",
				Title:                 "Galería repetida sobredimensionada",
				Summary:               "La galería repetida suma demasiado peso para el valor que aporta.",
				EstimatedSavingsBytes: 180_000,
				RelatedResourceIDs:    []string{"card-1", "card-2", "card-3"},
			},
		},
		nil,
	)

	if strings.Contains(strings.ToLower(draft.ShortProblem), "below the fold") {
		t.Fatalf("expected conservative asset copy, got %q", draft.ShortProblem)
	}
	if draft.Scope != "group" {
		t.Fatalf("expected group scope, got %q", draft.Scope)
	}
}

func TestBuildRuleBasedAssetInsightDefaultsToAssetScopeWithoutFinding(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:           "lonely-script",
			Type:         "script",
			Bytes:        45_000,
			PositionBand: "unknown",
			VisualRole:   "unknown",
		},
		nil,
		nil,
	)

	if draft.Scope != "asset" {
		t.Fatalf("expected asset scope, got %q", draft.Scope)
	}
}

func TestRecommendedFixForFindingUsesAstroSnippetForGallery(t *testing.T) {
	fix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{
			FrameworkHint: "astro",
			Evidence:      []string{"Se detectaron nodos astro-island."},
		},
	}, AnalysisFindingContext{
		ID: "repeated_gallery_overdelivery",
	})

	if fix == nil {
		t.Fatal("expected fix suggestion")
	}
	if strings.Contains(fix.OptimizedCode, `next/image`) {
		t.Fatalf("expected astro-specific code, got %q", fix.OptimizedCode)
	}
	if !strings.Contains(fix.OptimizedCode, `astro:assets`) {
		t.Fatalf("expected astro snippet, got %q", fix.OptimizedCode)
	}
	if !strings.Contains(fix.OptimizedCode, `"eager"`) || !strings.Contains(fix.OptimizedCode, `"lazy"`) {
		t.Fatalf("expected mixed eager/lazy guidance, got %q", fix.OptimizedCode)
	}
	if strings.Contains(fix.OptimizedCode, `index < 3`) {
		t.Fatalf("expected snippet to avoid hardcoded first row size, got %q", fix.OptimizedCode)
	}
	if !strings.Contains(fix.OptimizedCode, `firstRowCount`) {
		t.Fatalf("expected snippet to rely on firstRowCount, got %q", fix.OptimizedCode)
	}
}
