package scanner

import (
	"testing"

	"github.com/tronchos/wattless/server/internal/insights"
)

func TestSanitizeTopActionsFallsBackToVisibleVampire(t *testing.T) {
	actions := []insights.TopAction{
		{
			ID:                 "act-1",
			RelatedFindingID:   "below_fold_gallery_waste",
			RelatedResourceIDs: []string{"missing-1", "missing-2"},
		},
	}
	findings := []AnalysisFinding{
		{
			ID:                 "below_fold_gallery_waste",
			RelatedResourceIDs: []string{"missing-1", "missing-2"},
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
		t.Fatalf("expected visible repeated card fallback, got %#v", sanitized[0].RelatedResourceIDs)
	}
}
