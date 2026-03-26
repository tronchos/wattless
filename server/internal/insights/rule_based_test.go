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
				ID:                    "below_fold_gallery_waste",
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
