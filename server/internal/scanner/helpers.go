package scanner

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

func shareOf(bytes, totalBytes int64) float64 {
	if totalBytes <= 0 {
		return 0
	}
	return math.Round((float64(bytes)/float64(totalBytes))*1000) / 10
}
func resourceFailed(resource enrichedResource) bool {
	return resource.Failed || resource.StatusCode >= 400
}
func sumBytes(resources []enrichedResource) int64 {
	var total int64
	for _, resource := range resources {
		total += resource.Bytes
	}
	return total
}
func sumEstimatedSavings(resources []enrichedResource) int64 {
	var total int64
	for _, resource := range resources {
		total += estimateResourceSavings(resource)
	}
	return total
}
func estimateDeferredClusterSavings(resources []enrichedResource) int64 {
	return int64(math.Round(float64(sumBytes(resources)) * deferClusterSavingsFactor))
}
func sumEstimatedSavingsForIDs(resources []enrichedResource, ids []string) int64 {
	if len(ids) == 0 {
		return 0
	}
	lookup := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		lookup[id] = struct{}{}
	}
	var total int64
	for _, resource := range resources {
		if _, ok := lookup[resource.ID]; ok {
			total += estimateResourceSavings(resource)
		}
	}
	return total
}
func collectResourceIDs(resources []enrichedResource) []string {
	ids := make([]string, 0, len(resources))
	for _, resource := range resources {
		ids = append(ids, resource.ID)
	}
	return ids
}
func resourceForID(resources []enrichedResource, id string) *enrichedResource {
	for index := range resources {
		if resources[index].ID == id {
			return &resources[index]
		}
	}
	return nil
}
func collectTopResourceIDs(resources []enrichedResource, limit int) []string {
	if len(resources) == 0 || limit <= 0 {
		return nil
	}
	sorted := append([]enrichedResource(nil), resources...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Bytes == sorted[j].Bytes {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].Bytes > sorted[j].Bytes
	})
	if len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return collectResourceIDs(sorted)
}
func filterResources(resources []enrichedResource, predicate func(enrichedResource) bool) []enrichedResource {
	filtered := make([]enrichedResource, 0, len(resources))
	for _, resource := range resources {
		if predicate(resource) {
			filtered = append(filtered, resource)
		}
	}
	return filtered
}
func humanBytes(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	}

	units := []string{"KB", "MB", "GB"}
	value := float64(bytes) / 1024
	unitIndex := 0
	for value >= 1024 && unitIndex < len(units)-1 {
		value /= 1024
		unitIndex++
	}
	precision := 1
	if value >= 100 {
		precision = 0
	}
	return fmt.Sprintf("%.*f %s", precision, value, units[unitIndex])
}
func resourcesForIDs(resources []enrichedResource, ids []string) []enrichedResource {
	if len(ids) == 0 {
		return nil
	}
	lookup := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		lookup[id] = struct{}{}
	}
	selected := make([]enrichedResource, 0, len(ids))
	for _, resource := range resources {
		if _, ok := lookup[resource.ID]; ok {
			selected = append(selected, resource)
		}
	}
	return selected
}
func withArticle(label string) string {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "miniaturas del blog":
		return "las miniaturas del blog"
	case "logos de partners":
		return "los logos de partners"
	case "banderas":
		return "las banderas"
	case "avatares":
		return "los avatares"
	case "colección de miniaturas":
		return "la colección de miniaturas"
	case "grid de tarjetas":
		return "el grid de tarjetas"
	default:
		return strings.ToLower(strings.TrimSpace(label))
	}
}
