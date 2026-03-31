package scanner

import (
	"context"
	"errors"
	"log/slog"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/hosting"
	"github.com/tronchos/wattless/server/internal/insights"
	"github.com/tronchos/wattless/server/pkg/urlutil"
)

var errEmptyAIInsights = errors.New("ai provider returned no usable insights")

type HostingChecker interface {
	Check(context.Context, string) (hosting.Result, error)
}
type Service struct {
	cfg            config.Config
	hostingChecker HostingChecker
	ruleBased      insights.Provider
	ai             insights.Provider
	logger         *slog.Logger
}
type PreparedTarget struct {
	RawURL        string
	NormalizedURL string
	Hostname      string
	ResolvedIP    string
}

func NewService(
	cfg config.Config,
	hostingChecker HostingChecker,
	ruleBasedProvider insights.Provider,
	aiProvider insights.Provider,
	logger *slog.Logger,
) *Service {
	if ruleBasedProvider == nil {
		ruleBasedProvider = insights.NewRuleBasedProvider()
	}
	return &Service{
		cfg:            cfg,
		hostingChecker: hostingChecker,
		ruleBased:      ruleBasedProvider,
		ai:             aiProvider,
		logger:         logger,
	}
}
func (s *Service) Scan(ctx context.Context, rawURL string) (Report, error) {
	target, err := s.PrepareTarget(ctx, rawURL)
	if err != nil {
		return Report{}, err
	}

	return s.ScanPrepared(ctx, target)
}
func (s *Service) PrepareTarget(ctx context.Context, rawURL string) (PreparedTarget, error) {
	normalizedURL, hostname, err := urlutil.Normalize(rawURL)
	if err != nil {
		return PreparedTarget{}, err
	}

	resolvedIPs, err := urlutil.ValidatePublicTarget(ctx, hostname)
	if err != nil {
		return PreparedTarget{}, err
	}

	resolvedIP, err := preferredResolvedIP(resolvedIPs)
	if err != nil {
		return PreparedTarget{}, err
	}

	return PreparedTarget{
		RawURL:        rawURL,
		NormalizedURL: normalizedURL,
		Hostname:      hostname,
		ResolvedIP:    resolvedIP,
	}, nil
}

func (s *Service) HasAIProvider() bool {
	return s.ai != nil
}

func (s *Service) GenerateInsights(ctx context.Context, report Report) (insights.ProviderResult, error) {
	if s.ai == nil {
		return insights.ProviderResult{}, nil
	}

	result, err := s.ai.SummarizeReport(ctx, s.BuildReportContext(report))
	if err != nil {
		return insights.ProviderResult{}, err
	}
	if !hasMaterialInsights(result) {
		return insights.ProviderResult{}, errEmptyAIInsights
	}

	return result, nil
}

func (s *Service) ApplyInsights(report *Report, result insights.ProviderResult) {
	if report == nil {
		return
	}

	result.Insights = sanitizeInsightReport(result.Insights, report.Analysis.Findings, report.VampireElements)
	report.Insights = result.Insights
	report.VampireElements = attachAssetInsights(
		report.VampireElements,
		report.Analysis,
		report.Insights.TopActions,
		result.AssetInsights,
	)
}

func hasMaterialInsights(result insights.ProviderResult) bool {
	return result.Insights.ExecutiveSummary != "" ||
		result.Insights.PitchLine != "" ||
		len(result.Insights.TopActions) > 0 ||
		len(result.AssetInsights) > 0
}
