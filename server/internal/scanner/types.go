package scanner

import "github.com/tronchos/wattless/server/internal/insights"

type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type SiteProfile struct {
	FrameworkHint string   `json:"framework_hint"`
	Evidence      []string `json:"evidence"`
}

type ResourceSummary struct {
	ID                    string         `json:"id"`
	URL                   string         `json:"url"`
	Type                  string         `json:"type"`
	MIMEType              string         `json:"mime_type"`
	Hostname              string         `json:"hostname"`
	Party                 Party          `json:"party"`
	StatusCode            int            `json:"status_code"`
	Bytes                 int64          `json:"bytes"`
	Failed                bool           `json:"failed"`
	FailureReason         string         `json:"failure_reason"`
	TransferShare         float64        `json:"transfer_share"`
	EstimatedSavingsBytes int64          `json:"estimated_savings_bytes"`
	PositionBand          PositionBand   `json:"position_band"`
	VisualRole            VisualRole     `json:"visual_role"`
	DOMTag                string         `json:"dom_tag"`
	LoadingAttr           string         `json:"loading_attr"`
	FetchPriority         string         `json:"fetch_priority"`
	ResponsiveImage       bool           `json:"responsive_image"`
	NaturalWidth          int            `json:"natural_width,omitempty"`
	NaturalHeight         int            `json:"natural_height,omitempty"`
	VisibleRatio          float64        `json:"visible_ratio,omitempty"`
	IsThirdPartyTool      bool           `json:"is_third_party_tool"`
	ThirdPartyKind        ThirdPartyKind `json:"third_party_kind"`
	AssetInsight          AssetInsight   `json:"asset_insight"`
	BoundingBox           *BoundingBox   `json:"bounding_box"`
}

type FixSuggestion = insights.RecommendedFix

type AssetInsight struct {
	Source            string         `json:"source"`
	Scope             string         `json:"scope"`
	Title             string         `json:"title"`
	ShortProblem      string         `json:"short_problem"`
	WhyItMatters      string         `json:"why_it_matters"`
	RecommendedAction string         `json:"recommended_action"`
	Confidence        string         `json:"confidence"`
	LikelyLCPImpact   string         `json:"likely_lcp_impact"`
	RelatedFindingID  string         `json:"related_finding_id,omitempty"`
	RelatedActionID   string         `json:"related_action_id,omitempty"`
	Evidence          []string       `json:"evidence"`
	RecommendedFix    *FixSuggestion `json:"recommended_fix,omitempty"`
}

type ResourceBreakdown struct {
	Label      string  `json:"label"`
	Bytes      int64   `json:"bytes"`
	Requests   int     `json:"requests"`
	Percentage float64 `json:"percentage"`
}

type Summary struct {
	TotalRequests         int   `json:"total_requests"`
	SuccessfulRequests    int   `json:"successful_requests"`
	FailedRequests        int   `json:"failed_requests"`
	FirstPartyBytes       int64 `json:"first_party_bytes"`
	ThirdPartyBytes       int64 `json:"third_party_bytes"`
	PotentialSavingsBytes int64 `json:"potential_savings_bytes"`
	VisualMappedVampires  int   `json:"visual_mapped_vampires"`
}

type PerformanceMetrics struct {
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

type AnalysisSummary struct {
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

type AnalysisFinding struct {
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

type ResourceGroup struct {
	ID                 string       `json:"id"`
	Kind               GroupKind    `json:"kind"`
	Label              string       `json:"label"`
	TotalBytes         int64        `json:"total_bytes"`
	ResourceCount      int          `json:"resource_count"`
	PositionBand       PositionBand `json:"position_band"`
	RelatedResourceIDs []string     `json:"related_resource_ids"`
}

type Analysis struct {
	Summary        AnalysisSummary   `json:"summary"`
	Findings       []AnalysisFinding `json:"findings"`
	ResourceGroups []ResourceGroup   `json:"resource_groups"`
}

type ScreenshotTile struct {
	ID         string `json:"id"`
	Y          int    `json:"y"`
	Width      int    `json:"width"`
	Height     int    `json:"height"`
	DataBase64 string `json:"data_base64"`
}

type Screenshot struct {
	MimeType       string           `json:"mime_type"`
	Strategy       string           `json:"strategy"`
	ViewportWidth  int              `json:"viewport_width"`
	ViewportHeight int              `json:"viewport_height"`
	DocumentWidth  int              `json:"document_width"`
	DocumentHeight int              `json:"document_height"`
	CapturedHeight int              `json:"captured_height"`
	Tiles          []ScreenshotTile `json:"tiles"`
}

type Meta struct {
	GeneratedAt    string `json:"generated_at"`
	ScanDurationMS int64  `json:"scan_duration_ms"`
	ScannerVersion string `json:"scanner_version"`
}

type Methodology struct {
	Model       string   `json:"model"`
	Formula     string   `json:"formula"`
	Source      string   `json:"source"`
	Assumptions []string `json:"assumptions"`
}

type Report struct {
	URL                   string                `json:"url"`
	Score                 string                `json:"score"`
	TotalBytesTransferred int64                 `json:"total_bytes_transferred"`
	CO2GramsPerVisit      float64               `json:"co2_grams_per_visit"`
	HostingIsGreen        bool                  `json:"hosting_is_green"`
	HostingVerdict        string                `json:"hosting_verdict"`
	HostedBy              string                `json:"hosted_by"`
	SiteProfile           SiteProfile           `json:"site_profile"`
	Summary               Summary               `json:"summary"`
	BreakdownByType       []ResourceBreakdown   `json:"breakdown_by_type"`
	BreakdownByParty      []ResourceBreakdown   `json:"breakdown_by_party"`
	Insights              insights.ScanInsights `json:"insights"`
	VampireElements       []ResourceSummary     `json:"vampire_elements"`
	Performance           PerformanceMetrics    `json:"performance"`
	Analysis              Analysis              `json:"analysis"`
	Screenshot            Screenshot            `json:"screenshot"`
	Meta                  Meta                  `json:"meta"`
	Methodology           Methodology           `json:"methodology"`
	Warnings              []string              `json:"warnings"`
}

type rawResource struct {
	RequestID     string
	URL           string
	Type          string
	MIMEType      string
	Bytes         int64
	StatusCode    int
	Failed        bool
	FailureReason string
}

type domElement struct {
	URL             string  `json:"url"`
	Tag             string  `json:"tag"`
	LoadingAttr     string  `json:"loading"`
	FetchPriority   string  `json:"fetch_priority"`
	ResponsiveImage bool    `json:"responsive_image"`
	NaturalWidth    int     `json:"natural_width"`
	NaturalHeight   int     `json:"natural_height"`
	VisibleRatio    float64 `json:"visible_ratio"`
	SelectorHint    string  `json:"selector_hint"`
	X               float64 `json:"x"`
	Y               float64 `json:"y"`
	Width           float64 `json:"width"`
	Height          float64 `json:"height"`
}

type domSnapshot struct {
	Elements    []domElement `json:"elements"`
	SiteProfile SiteProfile  `json:"site_profile"`
}
