package scanner

import "sort"

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
