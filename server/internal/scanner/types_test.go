package scanner

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReportJSONIncludesMetaAndMethodology(t *testing.T) {
	report := Report{
		URL: "https://example.com",
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
	if !strings.Contains(payload, `"scanner_version":"2026.03"`) {
		t.Fatal("expected scanner version in JSON payload")
	}
}
