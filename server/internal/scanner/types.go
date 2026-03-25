package scanner

import "github.com/tronchos/wattless/server/internal/insights"

type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

type ResourceSummary struct {
	ID                    string       `json:"id"`
	URL                   string       `json:"url"`
	Type                  string       `json:"type"`
	MIMEType              string       `json:"mime_type"`
	Hostname              string       `json:"hostname"`
	Party                 string       `json:"party"`
	StatusCode            int          `json:"status_code"`
	Bytes                 int64        `json:"bytes"`
	Failed                bool         `json:"failed"`
	FailureReason         string       `json:"failure_reason"`
	TransferShare         float64      `json:"transfer_share"`
	EstimatedSavingsBytes int64        `json:"estimated_savings_bytes"`
	Recommendation        string       `json:"recommendation"`
	BoundingBox           *BoundingBox `json:"bounding_box"`
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
	LoadMS             int64 `json:"load_ms"`
	DOMContentLoadedMS int64 `json:"dom_content_loaded_ms"`
	ScriptDurationMS   int64 `json:"script_duration_ms"`
	LCPMS              int64 `json:"lcp_ms"`
	FCPMS              int64 `json:"fcp_ms"`
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
	Summary               Summary               `json:"summary"`
	BreakdownByType       []ResourceBreakdown   `json:"breakdown_by_type"`
	BreakdownByParty      []ResourceBreakdown   `json:"breakdown_by_party"`
	Insights              insights.ScanInsights `json:"insights"`
	VampireElements       []ResourceSummary     `json:"vampire_elements"`
	Performance           PerformanceMetrics    `json:"performance"`
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
	URL    string  `json:"url"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}
