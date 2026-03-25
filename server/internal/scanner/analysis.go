package scanner

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"path"
	"sort"
	"strings"

	"golang.org/x/net/publicsuffix"
)

const (
	partyFirst = "first_party"
	partyThird = "third_party"
)

func normalizeType(resourceType, mimeType, rawURL string) string {
	resourceType = strings.ToLower(resourceType)
	mimeType = strings.ToLower(mimeType)
	extension := strings.ToLower(path.Ext(resourcePath(rawURL)))

	switch extension {
	case ".avif", ".gif", ".ico", ".jpeg", ".jpg", ".png", ".svg", ".webp":
		return "image"
	case ".mp4", ".webm", ".mov", ".m4v", ".m3u8":
		return "video"
	case ".woff", ".woff2", ".ttf", ".otf":
		return "font"
	case ".css":
		return "stylesheet"
	case ".js", ".mjs", ".cjs":
		return "script"
	case ".html", ".htm":
		return "document"
	}

	switch {
	case strings.Contains(resourceType, "image") || strings.HasPrefix(mimeType, "image/"):
		return "image"
	case strings.Contains(resourceType, "media") || strings.HasPrefix(mimeType, "video/"):
		return "video"
	case strings.Contains(resourceType, "font") || strings.Contains(mimeType, "font"):
		return "font"
	case strings.Contains(resourceType, "stylesheet") || strings.Contains(mimeType, "css"):
		return "stylesheet"
	case strings.Contains(resourceType, "script") || strings.Contains(mimeType, "javascript"):
		return "script"
	case strings.Contains(resourceType, "document") || strings.Contains(mimeType, "html"):
		return "document"
	default:
		return "other"
	}
}

func resourceHostname(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}

func resourcePath(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	return parsed.Path
}

func classifyParty(pageHostname, assetHostname string) string {
	if pageHostname == "" || assetHostname == "" {
		return partyThird
	}
	if siteRoot(pageHostname) == siteRoot(assetHostname) {
		return partyFirst
	}
	return partyThird
}

func siteRoot(host string) string {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil || host == "localhost" {
		return host
	}

	root, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return host
	}
	return root
}

func estimateSavingsBytes(resourceType string, bytes int64) int64 {
	factor := 0.15
	switch resourceType {
	case "image":
		factor = 0.35
	case "video":
		factor = 0.60
	case "script":
		factor = 0.25
	case "stylesheet":
		factor = 0.20
	case "font":
		factor = 0.30
	case "document":
		factor = 0.10
	}
	return int64(math.Round(float64(bytes) * factor))
}

func buildBreakdowns(resources []enrichedResource, totalBytes int64) ([]ResourceBreakdown, []ResourceBreakdown) {
	typeBuckets := map[string]*bucket{}
	partyBuckets := map[string]*bucket{}

	for _, resource := range resources {
		addToBucket(typeBuckets, resource.Type, resource.Bytes)
		addToBucket(partyBuckets, resource.Party, resource.Bytes)
	}

	return sortBuckets(typeBuckets, totalBytes), sortBuckets(partyBuckets, totalBytes)
}

func addToBucket(buckets map[string]*bucket, label string, bytes int64) {
	if label == "" {
		label = "other"
	}
	entry, ok := buckets[label]
	if !ok {
		entry = &bucket{label: label}
		buckets[label] = entry
	}
	entry.bytes += bytes
	entry.requests++
}

type bucket struct {
	label    string
	bytes    int64
	requests int
}

func sortBuckets(buckets map[string]*bucket, totalBytes int64) []ResourceBreakdown {
	items := make([]ResourceBreakdown, 0, len(buckets))
	for _, entry := range buckets {
		items = append(items, ResourceBreakdown{
			Label:      entry.label,
			Bytes:      entry.bytes,
			Requests:   entry.requests,
			Percentage: shareOf(entry.bytes, totalBytes),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Bytes > items[j].Bytes
	})
	return items
}

func buildSummary(resources []enrichedResource, totalBytes int64, potentialSavings int64, visualMapped int) Summary {
	summary := Summary{
		TotalRequests:         len(resources),
		PotentialSavingsBytes: potentialSavings,
		VisualMappedVampires:  visualMapped,
	}

	for _, resource := range resources {
		if resourceFailed(resource) {
			summary.FailedRequests++
		} else {
			summary.SuccessfulRequests++
		}

		if resource.Party == partyThird {
			summary.ThirdPartyBytes += resource.Bytes
		} else {
			summary.FirstPartyBytes += resource.Bytes
		}
	}

	return summary
}

func rankVampireResources(resources []enrichedResource, totalBytes int64) ([]enrichedResource, []string) {
	candidates := append([]enrichedResource(nil), resources...)

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Bytes == candidates[j].Bytes {
			return candidates[i].ID < candidates[j].ID
		}
		return candidates[i].Bytes > candidates[j].Bytes
	})

	if len(candidates) > 5 {
		candidates = candidates[:5]
	}

	warnings := []string{}
	failedRequests := 0
	failedRanked := 0
	for _, resource := range resources {
		if resourceFailed(resource) {
			failedRequests++
		}
	}
	for _, resource := range candidates {
		if resourceFailed(resource) {
			failedRanked++
		}
	}
	if failedRequests > 0 {
		warnings = append(warnings, fmt.Sprintf("%d requests failed during the scan.", failedRequests))
	}
	if failedRanked > 0 {
		warnings = append(warnings, fmt.Sprintf("%d failed requests remain in the vampire ranking because they still transferred measurable bytes.", failedRanked))
	}

	thirdPartyBytes := int64(0)
	for _, resource := range resources {
		if resource.Party == partyThird {
			thirdPartyBytes += resource.Bytes
		}
	}
	if share := shareOf(thirdPartyBytes, totalBytes); share >= 50 {
		warnings = append(warnings, fmt.Sprintf("Third-party resources account for %.1f%% of transferred bytes.", share))
	}

	visualMapped := 0
	for _, resource := range candidates {
		if resource.BoundingBox != nil {
			visualMapped++
		}
	}
	if visualMapped == 0 && len(candidates) > 0 {
		warnings = append(warnings, "The heaviest resources could not be mapped to visible DOM boxes.")
	}

	return candidates, warnings
}

func shareOf(bytes, totalBytes int64) float64 {
	if totalBytes <= 0 {
		return 0
	}
	return math.Round((float64(bytes)/float64(totalBytes))*1000) / 10
}

func resourceFailed(resource enrichedResource) bool {
	return resource.Failed || resource.StatusCode >= 400
}
