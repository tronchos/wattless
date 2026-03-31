package scanner

import (
	"context"
	"log/slog"

	"github.com/tronchos/wattless/server/internal/config"
	"github.com/tronchos/wattless/server/internal/hosting"
	"github.com/tronchos/wattless/server/internal/insights"
	"github.com/tronchos/wattless/server/pkg/urlutil"
)

type HostingChecker interface {
	Check(context.Context, string) (hosting.Result, error)
}
type Service struct {
	cfg            config.Config
	hostingChecker HostingChecker
	insights       insights.Provider
	logger         *slog.Logger
}
type PreparedTarget struct {
	RawURL        string
	NormalizedURL string
	Hostname      string
	ResolvedIP    string
}

func NewService(cfg config.Config, hostingChecker HostingChecker, insightsProvider insights.Provider, logger *slog.Logger) *Service {
	return &Service{
		cfg:            cfg,
		hostingChecker: hostingChecker,
		insights:       insightsProvider,
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
