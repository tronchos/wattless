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
	Recommendation        string  `json:"recommendation,omitempty"`
}

type PerformanceContext struct {
	LoadMS             int64 `json:"load_ms"`
	DOMContentLoadedMS int64 `json:"dom_content_loaded_ms"`
	ScriptDurationMS   int64 `json:"script_duration_ms"`
	LCPMS              int64 `json:"lcp_ms"`
	FCPMS              int64 `json:"fcp_ms"`
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

type ReportContext struct {
	URL                   string             `json:"url"`
	Score                 string             `json:"score"`
	TotalBytesTransferred int64              `json:"total_bytes_transferred"`
	CO2GramsPerVisit      float64            `json:"co2_grams_per_visit"`
	HostingIsGreen        bool               `json:"hosting_is_green"`
	HostingVerdict        string             `json:"hosting_verdict"`
	HostedBy              string             `json:"hosted_by"`
	Performance           PerformanceContext `json:"performance"`
	Summary               SummaryContext     `json:"summary"`
	TopResources          []ResourceContext  `json:"top_resources"`
}

type RecommendedFix struct {
	Summary        string   `json:"summary"`
	OptimizedCode  string   `json:"optimized_code"`
	Changes        []string `json:"changes"`
	ExpectedImpact string   `json:"expected_impact"`
}

type TopAction struct {
	ID                    string          `json:"id"`
	Title                 string          `json:"title"`
	Reason                string          `json:"reason"`
	EstimatedSavingsBytes int64           `json:"estimated_savings_bytes"`
	LikelyLCPImpact       string          `json:"likely_lcp_impact"`
	RelatedResourceID     string          `json:"related_resource_id"`
	RecommendedFix        *RecommendedFix `json:"recommended_fix,omitempty"`
}

type ScanInsights struct {
	Provider         string      `json:"provider"`
	ExecutiveSummary string      `json:"executive_summary"`
	PitchLine        string      `json:"pitch_line"`
	TopActions       []TopAction `json:"top_actions"`
}

type Provider interface {
	Name() string
	SuggestResource(ResourceContext) string
	SummarizeReport(context.Context, ReportContext) (ScanInsights, error)
}
