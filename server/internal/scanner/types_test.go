package scanner

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportJSONIncludesMetaAndMethodology(t *testing.T) {
	report := Report{
		URL: "https://example.com",
		Analysis: Analysis{
			Summary: AnalysisSummary{
				AboveFoldVisualBytes: 1234,
			},
			Findings: []AnalysisFinding{
				{ID: "render_lcp_candidate", Title: "Ataca el LCP"},
			},
		},
		Meta: Meta{
			GeneratedAt:    "2026-03-25T22:10:00Z",
			ScanDurationMS: 1842,
			ScannerVersion: "2026.03",
		},
		Methodology: Methodology{
			Model:   "sustainable-web-design-mvp",
			Formula: "(bytes / 1_000_000_000) * 0.8 * 0.75 * 442",
			Source:  "Sustainable Web Design",
			Assumptions: []string{
				"0.75 return-visit factor",
				"442 gCO2e/kWh global average",
			},
		},
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal report: %v", err)
	}

	payload := string(raw)
	if !strings.Contains(payload, `"meta"`) {
		t.Fatal("expected meta in JSON payload")
	}
	if !strings.Contains(payload, `"methodology"`) {
		t.Fatal("expected methodology in JSON payload")
	}
	if !strings.Contains(payload, `"analysis"`) {
		t.Fatal("expected analysis in JSON payload")
	}
	if !strings.Contains(payload, `"scanner_version":"2026.03"`) {
		t.Fatal("expected scanner version in JSON payload")
	}
}

func TestResourceSummaryPartyMarshalsAsString(t *testing.T) {
	summary := ResourceSummary{
		ID:             "hero",
		Party:          PartyFirst,
		PositionBand:   PositionBandAboveFold,
		VisualRole:     VisualRoleHeroMedia,
		ThirdPartyKind: ThirdPartyKindUnknown,
	}

	raw, err := json.Marshal(summary)
	if err != nil {
		t.Fatalf("marshal resource summary: %v", err)
	}

	if !strings.Contains(string(raw), `"party":"first_party"`) {
		t.Fatalf("expected party to keep string wire contract, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"position_band":"above_fold"`) {
		t.Fatalf("expected position band to keep string wire contract, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"visual_role":"hero_media"`) {
		t.Fatalf("expected visual role to keep string wire contract, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"third_party_kind":"unknown"`) {
		t.Fatalf("expected third-party kind to keep string wire contract, got %s", string(raw))
	}
}

func TestResourceGroupKindMarshalsAsString(t *testing.T) {
	group := ResourceGroup{
		ID:           "gallery",
		Kind:         GroupKindRepeatedGallery,
		PositionBand: PositionBandMixed,
	}

	raw, err := json.Marshal(group)
	if err != nil {
		t.Fatalf("marshal resource group: %v", err)
	}

	if !strings.Contains(string(raw), `"kind":"repeated_gallery"`) {
		t.Fatalf("expected kind to keep string wire contract, got %s", string(raw))
	}
	if !strings.Contains(string(raw), `"position_band":"mixed"`) {
		t.Fatalf("expected group position band to keep string wire contract, got %s", string(raw))
	}
}

func TestTypedEnumsUnmarshalFromWireStrings(t *testing.T) {
	var summary ResourceSummary
	if err := json.Unmarshal([]byte(`{
		"id":"hero",
		"party":"first_party",
		"position_band":"above_fold",
		"visual_role":"hero_media",
		"third_party_kind":"unknown"
	}`), &summary); err != nil {
		t.Fatalf("unmarshal resource summary: %v", err)
	}

	if summary.Party != PartyFirst {
		t.Fatalf("expected typed party after unmarshal, got %#v", summary.Party)
	}
	if summary.PositionBand != PositionBandAboveFold {
		t.Fatalf("expected typed position band after unmarshal, got %#v", summary.PositionBand)
	}
	if summary.VisualRole != VisualRoleHeroMedia {
		t.Fatalf("expected typed visual role after unmarshal, got %#v", summary.VisualRole)
	}
	if summary.ThirdPartyKind != ThirdPartyKindUnknown {
		t.Fatalf("expected typed third-party kind after unmarshal, got %#v", summary.ThirdPartyKind)
	}

	var group ResourceGroup
	if err := json.Unmarshal([]byte(`{
		"id":"gallery",
		"kind":"repeated_gallery",
		"position_band":"mixed"
	}`), &group); err != nil {
		t.Fatalf("unmarshal resource group: %v", err)
	}

	if group.Kind != GroupKindRepeatedGallery {
		t.Fatalf("expected typed group kind after unmarshal, got %#v", group.Kind)
	}
	if group.PositionBand != PositionBandMixed {
		t.Fatalf("expected typed group position band after unmarshal, got %#v", group.PositionBand)
	}
}
