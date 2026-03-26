package insights

import (
	"context"
	"log/slog"
)

type CompositeProvider struct {
	primary  Provider
	fallback Provider
}

func NewCompositeProvider(primary, fallback Provider) CompositeProvider {
	return CompositeProvider{
		primary:  primary,
		fallback: fallback,
	}
}

func (provider CompositeProvider) Name() string {
	if provider.primary != nil {
		return provider.primary.Name()
	}
	if provider.fallback != nil {
		return provider.fallback.Name()
	}
	return "unknown"
}

func (provider CompositeProvider) SuggestResource(resource ResourceContext) string {
	if provider.primary != nil {
		return provider.primary.SuggestResource(resource)
	}
	if provider.fallback != nil {
		return provider.fallback.SuggestResource(resource)
	}
	return ""
}

func (provider CompositeProvider) SummarizeReport(ctx context.Context, report ReportContext) (ScanInsights, error) {
	if provider.primary != nil {
		result, err := provider.primary.SummarizeReport(ctx, report)
		if err == nil && result.ExecutiveSummary != "" {
			if result.Provider == "" {
				result.Provider = provider.primary.Name()
			}
			return result, nil
		}
		if err != nil {
			slog.Warn("insights_primary_failed", "provider", provider.primary.Name(), "error", err)
		} else {
			slog.Warn("insights_primary_empty", "provider", provider.primary.Name())
		}
	}

	if provider.fallback != nil {
		result, err := provider.fallback.SummarizeReport(ctx, report)
		if result.Provider == "" {
			result.Provider = provider.fallback.Name()
		}
		return result, err
	}

	return ScanInsights{}, nil
}
