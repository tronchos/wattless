package insights

import "context"

type ResourceContext struct {
	ID                    string  `json:"id,omitempty"`
	URL                   string  `json:"url,omitempty"`
	Type                  string  `json:"type,omitempty"`
	MIMEType              string  `json:"mime_type,omitempty"`
	Bytes                 int64   `json:"bytes,omitempty"`
	StatusCode            int     `json:"status_code,omitempty"`
	Failed                bool    `json:"failed,omitempty"`
	FailureReason         string  `json:"failure_reason,omitempty"`
	TransferShare         float64 `json:"transfer_share,omitempty"`
	EstimatedSavingsBytes int64   `json:"estimated_savings_bytes,omitempty"`
	PositionBand          string  `json:"position_band,omitempty"`
	VisualRole            string  `json:"visual_role,omitempty"`
	DOMTag                string  `json:"dom_tag,omitempty"`
	LoadingAttr           string  `json:"loading_attr,omitempty"`
	FetchPriority         string  `json:"fetch_priority,omitempty"`
	ResponsiveImage       bool    `json:"responsive_image,omitempty"`
	NaturalWidth          int     `json:"natural_width,omitempty"`
	NaturalHeight         int     `json:"natural_height,omitempty"`
	VisibleRatio          float64 `json:"visible_ratio,omitempty"`
	IsThirdPartyTool      bool    `json:"is_third_party_tool,omitempty"`
	ThirdPartyKind        string  `json:"third_party_kind,omitempty"`
}

type SiteProfileContext struct {
	FrameworkHint string   `json:"framework_hint"`
	Evidence      []string `json:"evidence"`
}

type PerformanceContext struct {
	LoadMS                   int64  `json:"load_ms"`
	DOMContentLoadedMS       int64  `json:"dom_content_loaded_ms"`
	ScriptResourceDurationMS int64  `json:"script_resource_duration_ms"`
	LCPMS                    int64  `json:"lcp_ms"`
	FCPMS                    int64  `json:"fcp_ms"`
	RenderMetricsComplete    bool   `json:"render_metrics_complete"`
	LongTasksTotalMS         int64  `json:"long_tasks_total_ms"`
	LongTasksCount           int    `json:"long_tasks_count"`
	LCPResourceURL           string `json:"lcp_resource_url,omitempty"`
	LCPResourceTag           string `json:"lcp_resource_tag,omitempty"`
	LCPSelectorHint          string `json:"lcp_selector_hint,omitempty"`
	LCPSize                  int64  `json:"lcp_size,omitempty"`
}

type SummaryContext struct {
	TotalRequests         int   `json:"total_requests"`
	SuccessfulRequests    int   `json:"successful_requests"`
	FailedRequests        int   `json:"failed_requests"`
	FirstPartyBytes       int64 `json:"first_party_bytes"`
	ThirdPartyBytes       int64 `json:"third_party_bytes"`
	PotentialSavingsBytes int64 `json:"potential_savings_bytes"`
	VisualMappedVampires  int   `json:"visual_mapped_vampires"`
}

type AnalysisSummaryContext struct {
	AboveFoldVisualBytes int64  `json:"above_fold_visual_bytes"`
	BelowFoldBytes       int64  `json:"below_fold_bytes"`
	LCPResourceID        string `json:"lcp_resource_id,omitempty"`
	LCPResourceURL       string `json:"lcp_resource_url,omitempty"`
	LCPResourceBytes     int64  `json:"lcp_resource_bytes,omitempty"`
	AnalyticsBytes       int64  `json:"analytics_bytes"`
	AnalyticsRequests    int    `json:"analytics_requests"`
	FontBytes            int64  `json:"font_bytes"`
	FontRequests         int    `json:"font_requests"`
	RepeatedGalleryBytes int64  `json:"repeated_gallery_bytes"`
	RepeatedGalleryCount int    `json:"repeated_gallery_count"`
	RenderCriticalBytes  int64  `json:"render_critical_bytes"`
}

type AnalysisFindingContext struct {
	ID                    string   `json:"id"`
	Category              string   `json:"category"`
	Severity              string   `json:"severity"`
	Confidence            string   `json:"confidence"`
	Title                 string   `json:"title"`
	Summary               string   `json:"summary"`
	Evidence              []string `json:"evidence"`
	EstimatedSavingsBytes int64    `json:"estimated_savings_bytes"`
	RelatedResourceIDs    []string `json:"related_resource_ids"`
}

type ResourceGroupContext struct {
	ID                 string   `json:"id"`
	Kind               string   `json:"kind"`
	Label              string   `json:"label"`
	TotalBytes         int64    `json:"total_bytes"`
	ResourceCount      int      `json:"resource_count"`
	PositionBand       string   `json:"position_band"`
	RelatedResourceIDs []string `json:"related_resource_ids"`
}

type AnalysisContext struct {
	Summary        AnalysisSummaryContext   `json:"summary"`
	Findings       []AnalysisFindingContext `json:"findings"`
	ResourceGroups []ResourceGroupContext   `json:"resource_groups"`
}

type ReportContext struct {
	URL                   string             `json:"url"`
	Score                 string             `json:"score"`
	TotalBytesTransferred int64              `json:"total_bytes_transferred"`
	CO2GramsPerVisit      float64            `json:"co2_grams_per_visit"`
	HostingIsGreen        bool               `json:"hosting_is_green"`
	HostingVerdict        string             `json:"hosting_verdict"`
	HostedBy              string             `json:"hosted_by"`
	SiteProfile           SiteProfileContext `json:"site_profile"`
	Performance           PerformanceContext `json:"performance"`
	Summary               SummaryContext     `json:"summary"`
	Analysis              AnalysisContext    `json:"analysis"`
	TopResources          []ResourceContext  `json:"top_resources"`
}

type RecommendedFix struct {
	Summary        string   `json:"summary"`
	OptimizedCode  string   `json:"optimized_code"`
	Changes        []string `json:"changes"`
	ExpectedImpact string   `json:"expected_impact"`
}

type TopAction struct {
	ID                        string          `json:"id"`
	RelatedFindingID          string          `json:"related_finding_id"`
	Title                     string          `json:"title"`
	Reason                    string          `json:"reason"`
	Confidence                string          `json:"confidence"`
	Evidence                  []string        `json:"evidence"`
	EstimatedSavingsBytes     int64           `json:"estimated_savings_bytes"`
	LikelyLCPImpact           string          `json:"likely_lcp_impact"`
	RelatedResourceIDs        []string        `json:"related_resource_ids"`
	VisibleRelatedResourceIDs []string        `json:"visible_related_resource_ids"`
	RecommendedFix            *RecommendedFix `json:"recommended_fix,omitempty"`
}

type AssetInsightDraft struct {
	ResourceID        string          `json:"resource_id"`
	Title             string          `json:"title"`
	ShortProblem      string          `json:"short_problem"`
	WhyItMatters      string          `json:"why_it_matters"`
	RecommendedAction string          `json:"recommended_action"`
	Confidence        string          `json:"confidence"`
	LikelyLCPImpact   string          `json:"likely_lcp_impact"`
	RelatedFindingID  string          `json:"related_finding_id,omitempty"`
	RelatedActionID   string          `json:"related_action_id,omitempty"`
	Evidence          []string        `json:"evidence"`
	RecommendedFix    *RecommendedFix `json:"recommended_fix,omitempty"`
	Scope             string          `json:"scope"`
	Source            string          `json:"source"`
}

type ScanInsights struct {
	Provider         string      `json:"provider"`
	ExecutiveSummary string      `json:"executive_summary"`
	PitchLine        string      `json:"pitch_line"`
	TopActions       []TopAction `json:"top_actions"`
}

type ProviderResult struct {
	Insights      ScanInsights
	AssetInsights []AssetInsightDraft
}

type Provider interface {
	Name() string
	SuggestResource(ResourceContext) string
	SummarizeReport(context.Context, ReportContext) (ProviderResult, error)
}
