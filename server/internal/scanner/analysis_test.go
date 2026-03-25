package scanner

import "testing"

func TestNormalizeTypePrefersURLHints(t *testing.T) {
	got := normalizeType("Other", "text/html", "https://example.com/favicon.ico")
	if got != "image" {
		t.Fatalf("expected image, got %s", got)
	}
}

func TestClassifyPartyUsesSiteRoot(t *testing.T) {
	if got := classifyParty("app.example.com", "cdn.example.com"); got != partyFirst {
		t.Fatalf("expected first party, got %s", got)
	}
	if got := classifyParty("example.com", "tracker.other.net"); got != partyThird {
		t.Fatalf("expected third party, got %s", got)
	}
	if got := classifyParty("docs.example.github.io", "cdn.other.github.io"); got != partyThird {
		t.Fatalf("expected github.io tenants to remain third party, got %s", got)
	}
}

func TestRankVampireResourcesKeepsFailedRequestsWhenTheyTransferBytes(t *testing.T) {
	resources := []enrichedResource{
		{ID: "req-1", URL: "https://example.com/favicon.ico", Bytes: 9000, StatusCode: 404, Failed: true, Party: partyFirst, Type: "image"},
		{ID: "req-2", URL: "https://example.com/app.js", Bytes: 5000, StatusCode: 200, Party: partyFirst, Type: "script"},
	}

	ranked, warnings := rankVampireResources(resources, 14_000)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 ranked resources, got %d", len(ranked))
	}
	if ranked[0].URL != "https://example.com/favicon.ico" {
		t.Fatalf("unexpected ranked resource: %s", ranked[0].URL)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings")
	}
}

func TestBuildSummaryCountsNetworkFailuresWithoutStatusCodes(t *testing.T) {
	summary := buildSummary([]enrichedResource{
		{ID: "req-1", URL: "https://example.com/app.js", Bytes: 1200, Failed: true, FailureReason: "net::ERR_BLOCKED_BY_CLIENT", Party: partyThird},
		{ID: "req-2", URL: "https://example.com/style.css", Bytes: 800, StatusCode: 200, Party: partyFirst},
	}, 2_000, 300, 1)

	if summary.FailedRequests != 1 {
		t.Fatalf("expected 1 failed request, got %d", summary.FailedRequests)
	}
	if summary.SuccessfulRequests != 1 {
		t.Fatalf("expected 1 successful request, got %d", summary.SuccessfulRequests)
	}
}
