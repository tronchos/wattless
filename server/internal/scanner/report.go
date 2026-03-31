package scanner

import (
	"context"
	"strings"
	"time"

	"github.com/tronchos/wattless/server/internal/hosting"
	"github.com/tronchos/wattless/server/internal/insights"
	"github.com/tronchos/wattless/server/pkg/co2"
	"github.com/tronchos/wattless/server/pkg/score"
)

const scannerVersion = "2026.03"

func (s *Service) ScanPrepared(ctx context.Context, target PreparedTarget) (Report, error) {
	startedAt := time.Now()
	resources, perf, screenshot, siteProfile, warnings, err := s.runBrowserScan(ctx, target)
	if err != nil {
		return Report{}, err
	}
	if warnings == nil {
		warnings = []string{}
	}
	resources, resourceGroups := enrichResourcesForAnalysis(resources, perf, screenshot.ViewportWidth, screenshot.ViewportHeight)

	hostingResult, hostingWarnings := s.resolveHosting(ctx, target.Hostname)
	warnings = append(warnings, hostingWarnings...)
	if !perf.RenderMetricsComplete {
		warnings = append(warnings, "Render metrics could not be captured completely; LCP/FCP-based editorial framing may be limited for this scan.")
	}

	totalBytes := int64(0)
	for _, resource := range resources {
		totalBytes += resource.Bytes
	}

	potentialSavings := int64(0)
	for _, resource := range resources {
		potentialSavings += estimateResourceSavings(resource)
	}

	analysis := buildAnalysis(resources, perf, resourceGroups)
	top, rankingWarnings := rankVampireResources(resources, resourceGroups, analysis.Findings, totalBytes)
	warnings = append(warnings, rankingWarnings...)

	vampires := make([]ResourceSummary, 0, len(top))
	visualMapped := 0
	for _, resource := range top {
		savings := estimateResourceSavings(resource)
		if resource.BoundingBox != nil {
			visualMapped++
		}
		vampires = append(vampires, ResourceSummary{
			ID:                    resource.ID,
			URL:                   resource.URL,
			Type:                  resource.Type,
			MIMEType:              resource.MIMEType,
			Hostname:              resource.Hostname,
			Party:                 resource.Party,
			StatusCode:            resource.StatusCode,
			Bytes:                 resource.Bytes,
			Failed:                resource.Failed,
			FailureReason:         resource.FailureReason,
			TransferShare:         shareOf(resource.Bytes, totalBytes),
			EstimatedSavingsBytes: savings,
			PositionBand:          resource.PositionBand,
			VisualRole:            resource.VisualRole,
			DOMTag:                resource.DOMTag,
			LoadingAttr:           resource.LoadingAttr,
			FetchPriority:         resource.FetchPriority,
			ResponsiveImage:       resource.ResponsiveImage,
			NaturalWidth:          resource.NaturalWidth,
			NaturalHeight:         resource.NaturalHeight,
			VisibleRatio:          resource.VisibleRatio,
			IsThirdPartyTool:      resource.IsThirdPartyTool,
			ThirdPartyKind:        resource.ThirdPartyKind,
			BoundingBox:           resource.BoundingBox,
		})
	}

	grams := co2.FromBytes(totalBytes)
	breakdownByType, breakdownByParty := buildBreakdowns(resources, totalBytes)
	summary := buildSummary(resources, totalBytes, potentialSavings, visualMapped)
	finalScore := score.FromCO2(grams)

	report := Report{
		URL:                   target.NormalizedURL,
		Score:                 finalScore,
		TotalBytesTransferred: totalBytes,
		CO2GramsPerVisit:      grams,
		HostingIsGreen:        hostingResult.IsGreen,
		HostingVerdict:        string(hostingResult.Verdict),
		HostedBy:              hostingResult.HostedBy,
		SiteProfile:           siteProfile,
		Summary:               summary,
		BreakdownByType:       breakdownByType,
		BreakdownByParty:      breakdownByParty,
		VampireElements:       vampires,
		Performance:           perf,
		Analysis:              analysis,
		Screenshot:            screenshot,
		Methodology:           defaultMethodology(),
		Warnings:              warnings,
	}

	report.VampireElements = vampires

	providerResult, err := s.ruleBased.SummarizeReport(ctx, s.BuildReportContext(report))
	if err != nil {
		s.logger.Warn("report_rule_based_insights_failed", "url", target.NormalizedURL, "error", err)
		providerResult = insights.ProviderResult{}
	}
	s.ApplyInsights(&report, providerResult)

	report.Meta = buildMeta(startedAt, time.Now())

	return report, nil
}

func (s *Service) BuildReportContext(report Report) insights.ReportContext {
	return insights.ReportContext{
		URL:                   report.URL,
		Score:                 report.Score,
		TotalBytesTransferred: report.TotalBytesTransferred,
		CO2GramsPerVisit:      report.CO2GramsPerVisit,
		HostingIsGreen:        report.HostingIsGreen,
		HostingVerdict:        report.HostingVerdict,
		HostedBy:              report.HostedBy,
		SiteProfile: insights.SiteProfileContext{
			FrameworkHint: report.SiteProfile.FrameworkHint,
			Evidence:      append([]string(nil), report.SiteProfile.Evidence...),
		},
		Performance: insights.PerformanceContext{
			LoadMS:                   report.Performance.LoadMS,
			DOMContentLoadedMS:       report.Performance.DOMContentLoadedMS,
			ScriptResourceDurationMS: report.Performance.ScriptResourceDurationMS,
			LCPMS:                    report.Performance.LCPMS,
			FCPMS:                    report.Performance.FCPMS,
			RenderMetricsComplete:    report.Performance.RenderMetricsComplete,
			LongTasksTotalMS:         report.Performance.LongTasksTotalMS,
			LongTasksCount:           report.Performance.LongTasksCount,
			LCPResourceURL:           report.Performance.LCPResourceURL,
			LCPResourceTag:           report.Performance.LCPResourceTag,
			LCPSelectorHint:          report.Performance.LCPSelectorHint,
			LCPSize:                  report.Performance.LCPSize,
		},
		Summary: insights.SummaryContext{
			TotalRequests:         report.Summary.TotalRequests,
			SuccessfulRequests:    report.Summary.SuccessfulRequests,
			FailedRequests:        report.Summary.FailedRequests,
			FirstPartyBytes:       report.Summary.FirstPartyBytes,
			ThirdPartyBytes:       report.Summary.ThirdPartyBytes,
			PotentialSavingsBytes: report.Summary.PotentialSavingsBytes,
			VisualMappedVampires:  report.Summary.VisualMappedVampires,
		},
		Analysis:     makeInsightAnalysis(report.Analysis),
		TopResources: makeInsightResources(report.VampireElements),
	}
}

type enrichedResource struct {
	ID               string
	URL              string
	Type             string
	MIMEType         string
	Hostname         string
	Party            Party
	StatusCode       int
	Bytes            int64
	Failed           bool
	FailureReason    string
	BoundingBox      *BoundingBox
	DOMTag           string
	LoadingAttr      string
	FetchPriority    string
	ResponsiveImage  bool
	NaturalWidth     int
	NaturalHeight    int
	VisibleRatio     float64
	SelectorHint     string
	PositionBand     PositionBand
	VisualRole       VisualRole
	IsThirdPartyTool bool
	ThirdPartyKind   ThirdPartyKind
}

func (s *Service) resolveHosting(ctx context.Context, hostname string) (hosting.Result, []string) {
	result, err := s.hostingChecker.Check(ctx, hostname)
	if err != nil {
		s.logger.Warn("greencheck_failed", "hostname", hostname, "error", err)
		return hosting.Result{Verdict: hosting.VerdictUnknown}, []string{"Green hosting check failed; returning unknown verdict."}
	}
	return result, nil
}
func buildMeta(startedAt, finishedAt time.Time) Meta {
	return Meta{
		GeneratedAt:    finishedAt.UTC().Format(time.RFC3339),
		ScanDurationMS: finishedAt.Sub(startedAt).Milliseconds(),
		ScannerVersion: scannerVersion,
	}
}
func defaultMethodology() Methodology {
	return Methodology{
		Model:   "sustainable-web-design-mvp",
		Formula: "(bytes / 1_000_000_000) * 0.8 * 0.75 * 442",
		Source:  "Sustainable Web Design",
		Assumptions: []string{
			"0.75 return-visit factor",
			"442 gCO2e/kWh global average",
		},
	}
}
func normalizeSiteProfile(profile SiteProfile) SiteProfile {
	switch profile.FrameworkHint {
	case "astro", "nextjs", "generic", "unknown":
	default:
		profile.FrameworkHint = "generic"
	}
	if len(profile.Evidence) == 0 {
		profile.Evidence = []string{"No se detectaron marcadores claros de framework; se usa perfil genérico."}
	}
	return profile
}
func makeInsightResources(resources []ResourceSummary) []insights.ResourceContext {
	output := make([]insights.ResourceContext, 0, len(resources))
	for _, resource := range resources {
		output = append(output, insights.ResourceContext{
			ID:                    resource.ID,
			URL:                   resource.URL,
			Type:                  resource.Type,
			MIMEType:              resource.MIMEType,
			Bytes:                 resource.Bytes,
			StatusCode:            resource.StatusCode,
			Failed:                resource.Failed,
			FailureReason:         resource.FailureReason,
			TransferShare:         resource.TransferShare,
			EstimatedSavingsBytes: resource.EstimatedSavingsBytes,
			PositionBand:          string(resource.PositionBand),
			VisualRole:            string(resource.VisualRole),
			DOMTag:                resource.DOMTag,
			LoadingAttr:           resource.LoadingAttr,
			FetchPriority:         resource.FetchPriority,
			ResponsiveImage:       resource.ResponsiveImage,
			NaturalWidth:          resource.NaturalWidth,
			NaturalHeight:         resource.NaturalHeight,
			VisibleRatio:          resource.VisibleRatio,
			IsThirdPartyTool:      resource.IsThirdPartyTool,
			ThirdPartyKind:        string(resource.ThirdPartyKind),
		})
	}
	return output
}
func makeInsightAnalysis(analysis Analysis) insights.AnalysisContext {
	findings := make([]insights.AnalysisFindingContext, 0, len(analysis.Findings))
	for _, finding := range analysis.Findings {
		findings = append(findings, insights.AnalysisFindingContext{
			ID:                    finding.ID,
			Category:              finding.Category,
			Severity:              finding.Severity,
			Confidence:            finding.Confidence,
			Title:                 finding.Title,
			Summary:               finding.Summary,
			Evidence:              append([]string(nil), finding.Evidence...),
			EstimatedSavingsBytes: finding.EstimatedSavingsBytes,
			RelatedResourceIDs:    append([]string(nil), finding.RelatedResourceIDs...),
		})
	}

	groups := make([]insights.ResourceGroupContext, 0, len(analysis.ResourceGroups))
	for _, group := range analysis.ResourceGroups {
		groups = append(groups, insights.ResourceGroupContext{
			ID:                 group.ID,
			Kind:               string(group.Kind),
			Label:              group.Label,
			TotalBytes:         group.TotalBytes,
			ResourceCount:      group.ResourceCount,
			PositionBand:       string(group.PositionBand),
			RelatedResourceIDs: append([]string(nil), group.RelatedResourceIDs...),
		})
	}

	return insights.AnalysisContext{
		Summary: insights.AnalysisSummaryContext{
			AboveFoldVisualBytes: analysis.Summary.AboveFoldVisualBytes,
			BelowFoldBytes:       analysis.Summary.BelowFoldBytes,
			LCPResourceID:        analysis.Summary.LCPResourceID,
			LCPResourceURL:       analysis.Summary.LCPResourceURL,
			LCPResourceBytes:     analysis.Summary.LCPResourceBytes,
			AnalyticsBytes:       analysis.Summary.AnalyticsBytes,
			AnalyticsRequests:    analysis.Summary.AnalyticsRequests,
			FontBytes:            analysis.Summary.FontBytes,
			FontRequests:         analysis.Summary.FontRequests,
			RepeatedGalleryBytes: analysis.Summary.RepeatedGalleryBytes,
			RepeatedGalleryCount: analysis.Summary.RepeatedGalleryCount,
			RenderCriticalBytes:  analysis.Summary.RenderCriticalBytes,
		},
		Findings:       findings,
		ResourceGroups: groups,
	}
}
func sanitizeInsightReport(result insights.ScanInsights, findings []AnalysisFinding, vampires []ResourceSummary) insights.ScanInsights {
	result.TopActions = sanitizeTopActions(result.TopActions, findings, vampires)
	return result
}
func sanitizeTopActions(actions []insights.TopAction, findings []AnalysisFinding, vampires []ResourceSummary) []insights.TopAction {
	if len(actions) == 0 {
		return actions
	}

	visibleIDs := make(map[string]struct{}, len(vampires))
	for _, vampire := range vampires {
		visibleIDs[vampire.ID] = struct{}{}
	}

	findingsByID := make(map[string]AnalysisFinding, len(findings))
	for _, finding := range findings {
		findingsByID[finding.ID] = finding
	}

	output := append([]insights.TopAction(nil), actions...)
	for index := range output {
		related := dedupeActionResourceIDs(output[index].RelatedResourceIDs)
		if len(related) == 0 {
			if finding, ok := findingsByID[output[index].RelatedFindingID]; ok {
				related = dedupeActionResourceIDs(finding.RelatedResourceIDs)
			}
		}
		output[index].RelatedResourceIDs = related
		output[index].VisibleRelatedResourceIDs = filterVisibleActionResourceIDs(related, visibleIDs)
	}

	return output
}
func dedupeActionResourceIDs(ids []string) []string {
	filtered := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		filtered = append(filtered, id)
	}
	return filtered
}
func filterVisibleActionResourceIDs(ids []string, visibleIDs map[string]struct{}) []string {
	filtered := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := visibleIDs[id]; !ok {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		filtered = append(filtered, id)
	}
	return filtered
}
