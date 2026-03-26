package insights

import (
	"context"
	"errors"
	"testing"
)

type failingProvider struct{}

func (failingProvider) Name() string { return "failing" }

func (failingProvider) SuggestResource(resource ResourceContext) string { return "" }

func (failingProvider) SummarizeReport(ctx context.Context, report ReportContext) (ProviderResult, error) {
	return ProviderResult{}, errors.New("boom")
}

func TestCompositeProviderFallsBackForSummary(t *testing.T) {
	provider := NewCompositeProvider(failingProvider{}, NewRuleBasedProvider())

	result, err := provider.SummarizeReport(context.Background(), ReportContext{
		URL:                   "https://example.com",
		Score:                 "B",
		TotalBytesTransferred: 900_000,
		CO2GramsPerVisit:      0.24,
		Performance: PerformanceContext{
			LCPMS: 2800,
		},
		TopResources: []ResourceContext{
			{ID: "req-1", Type: "image", Bytes: 500_000, EstimatedSavingsBytes: 200_000, TransferShare: 55},
		},
	})
	if err != nil {
		t.Fatalf("expected fallback summary, got error: %v", err)
	}
	if result.Insights.Provider != "rule_based" {
		t.Fatalf("expected fallback provider, got %s", result.Insights.Provider)
	}
	if result.Insights.ExecutiveSummary == "" {
		t.Fatal("expected executive summary")
	}
}
