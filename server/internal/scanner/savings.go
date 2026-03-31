package scanner

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strings"
)

func estimateSavingsBytes(resourceType, mimeType string, bytes int64) int64 {
	factor := 0.15
	switch resourceType {
	case "image":
		factor = baseImageSavingsFactor(mimeType)
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
func estimateResourceSavings(resource enrichedResource) int64 {
	if resource.Type != "image" {
		return estimateSavingsBytes(resource.Type, resource.MIMEType, resource.Bytes)
	}

	factor := imageSavingsFactor(resource)
	return int64(math.Round(float64(resource.Bytes) * factor))
}
func baseImageSavingsFactor(mimeType string) float64 {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.Contains(mimeType, "svg"):
		return 0
	case strings.Contains(mimeType, "avif"):
		return 0.10
	case strings.Contains(mimeType, "webp"):
		return 0.20
	case strings.Contains(mimeType, "png"), strings.Contains(mimeType, "jpeg"), strings.Contains(mimeType, "jpg"):
		return 0.35
	default:
		return 0.20
	}
}
func maxImageSavingsFactor(mimeType string) float64 {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.Contains(mimeType, "svg"):
		return 0
	case strings.Contains(mimeType, "avif"):
		return 0.50
	case strings.Contains(mimeType, "webp"):
		return 0.50
	case strings.Contains(mimeType, "png"), strings.Contains(mimeType, "jpeg"), strings.Contains(mimeType, "jpg"):
		return 0.60
	default:
		return 0.50
	}
}
func imageSavingsFactor(resource enrichedResource) float64 {
	factor := baseImageSavingsFactor(resource.MIMEType)
	overdelivery := responsiveOverdeliveryFactor(resource)
	if overdelivery > factor {
		factor = overdelivery
	}
	maxFactor := maxImageSavingsFactor(resource.MIMEType)
	if factor > maxFactor {
		return maxFactor
	}
	return factor
}
func responsiveOverdeliveryFactor(resource enrichedResource) float64 {
	if resource.BoundingBox == nil || resource.NaturalWidth <= 0 || resource.NaturalHeight <= 0 {
		return 0
	}

	renderedArea := resource.BoundingBox.Width * resource.BoundingBox.Height
	if renderedArea <= 0 {
		return 0
	}

	naturalArea := float64(resource.NaturalWidth * resource.NaturalHeight)
	if naturalArea <= 0 {
		return 0
	}

	idealArea := renderedArea * 4
	if naturalArea <= idealArea {
		return 0
	}

	return 1 - (idealArea / naturalArea)
}
func hasMeasuredRenderedBox(resource enrichedResource) bool {
	return resource.BoundingBox != nil && resource.BoundingBox.Width > 0 && resource.BoundingBox.Height > 0
}
func groupHasMixedFormats(resources []enrichedResource) bool {
	hasLegacy := false
	hasModern := false
	for _, resource := range resources {
		switch {
		case isLegacyImageFormat(resource.MIMEType):
			hasLegacy = true
		case isModernImageFormat(resource.MIMEType):
			hasModern = true
		}
	}
	return hasLegacy && hasModern
}
func legacyAssetOutweighsModernSibling(candidate enrichedResource, resources []enrichedResource) bool {
	if !isLegacyImageFormat(candidate.MIMEType) {
		return false
	}
	candidateKey := canonicalImageVariantKey(candidate.URL)
	if candidateKey == "" {
		return false
	}
	for _, resource := range resources {
		if resource.ID == candidate.ID || !isModernImageFormat(resource.MIMEType) {
			continue
		}
		if !isComparableModernVariant(candidate, resource, candidateKey) {
			continue
		}
		if candidate.Bytes >= resource.Bytes*4 {
			return true
		}
	}
	return false
}
func canonicalImageVariantKey(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	base := path.Base(parsed.Path)
	ext := path.Ext(base)
	if ext == "" {
		return ""
	}
	base = strings.TrimSuffix(base, ext)
	base = strings.ToLower(strings.TrimSpace(base))
	if base == "" {
		return ""
	}
	dir := strings.ToLower(strings.TrimSpace(path.Dir(parsed.Path)))
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	return host + "|" + dir + "|" + base
}
func isComparableModernVariant(candidate, other enrichedResource, candidateKey string) bool {
	if !hasMeasuredRenderedBox(candidate) || !hasMeasuredRenderedBox(other) {
		return false
	}
	if canonicalImageVariantKey(other.URL) != candidateKey {
		return false
	}
	if !boxesAreComparable(candidate.BoundingBox, other.BoundingBox) {
		return false
	}
	return naturalAspectRatiosComparable(candidate, other)
}
func boxesAreComparable(left, right *BoundingBox) bool {
	if left == nil || right == nil || left.Width <= 0 || left.Height <= 0 || right.Width <= 0 || right.Height <= 0 {
		return false
	}
	return roughlyComparableRatio(left.Width, right.Width, 0.25) && roughlyComparableRatio(left.Height, right.Height, 0.25)
}
func naturalAspectRatiosComparable(left, right enrichedResource) bool {
	if left.NaturalWidth <= 0 || left.NaturalHeight <= 0 || right.NaturalWidth <= 0 || right.NaturalHeight <= 0 {
		return false
	}
	leftAspect := float64(left.NaturalWidth) / float64(left.NaturalHeight)
	rightAspect := float64(right.NaturalWidth) / float64(right.NaturalHeight)
	return roughlyComparableRatio(leftAspect, rightAspect, 0.15)
}
func roughlyComparableRatio(left, right, tolerance float64) bool {
	if left <= 0 || right <= 0 {
		return false
	}
	delta := math.Abs(left-right) / math.Max(left, right)
	return delta <= tolerance
}
func isLegacyImageFormat(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	return strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") || strings.Contains(mimeType, "png")
}
func isModernImageFormat(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	return strings.Contains(mimeType, "avif") || strings.Contains(mimeType, "webp")
}
func isLegacyFontFormat(resource enrichedResource) bool {
	if resource.Type != "font" {
		return false
	}
	haystack := strings.ToLower(strings.TrimSpace(resource.MIMEType + " " + resource.URL))
	if strings.Contains(haystack, "woff2") {
		return false
	}
	return containsAny(haystack, "woff", "ttf", "otf", "truetype", "opentype")
}
func collectModernFontSignatures(resources []enrichedResource) map[string]struct{} {
	signatures := make(map[string]struct{})
	for _, resource := range resources {
		if resource.Type != "font" {
			continue
		}
		haystack := strings.ToLower(strings.TrimSpace(resource.MIMEType + " " + resource.URL))
		if !strings.Contains(haystack, "woff2") {
			continue
		}
		signature := fontResourceSignature(resource)
		if signature == "" {
			continue
		}
		signatures[signature] = struct{}{}
	}
	return signatures
}
func hasModernFontEquivalent(resource enrichedResource, modernSignatures map[string]struct{}) bool {
	signature := fontResourceSignature(resource)
	if signature == "" {
		return false
	}
	_, ok := modernSignatures[signature]
	return ok
}
func fontResourceSignature(resource enrichedResource) string {
	filename := strings.ToLower(strings.TrimSpace(path.Base(resourcePath(resource.URL))))
	if filename == "" || filename == "." || filename == "/" {
		return ""
	}
	extension := path.Ext(filename)
	if extension != "" {
		filename = strings.TrimSuffix(filename, extension)
	}
	return filename
}
func estimateLegacyFontSavings(bytes int64) int64 {
	return int64(math.Round(float64(bytes) * 0.30))
}
func collectTextLCPResourceCandidates(resources []enrichedResource, limit int) []enrichedResource {
	candidates := filterResources(resources, func(resource enrichedResource) bool {
		return resource.Type == "font" || resource.Type == "stylesheet" || resource.Type == "script"
	})

	sort.Slice(candidates, func(i, j int) bool {
		leftPriority := textLCPResourcePriority(candidates[i])
		rightPriority := textLCPResourcePriority(candidates[j])
		if leftPriority == rightPriority {
			if candidates[i].Bytes == candidates[j].Bytes {
				return candidates[i].ID < candidates[j].ID
			}
			return candidates[i].Bytes > candidates[j].Bytes
		}
		return leftPriority > rightPriority
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	return candidates
}
func textLCPResourcePriority(resource enrichedResource) int {
	switch resource.Type {
	case "font":
		return 3
	case "stylesheet":
		return 2
	case "script":
		return 1
	default:
		return 0
	}
}
func countNonResponsiveImages(resources []enrichedResource) int {
	total := 0
	for _, resource := range resources {
		if resource.Type == "image" && !resource.ResponsiveImage {
			total++
		}
	}
	return total
}
func countLazyLoadedImages(resources []enrichedResource) int {
	total := 0
	for _, resource := range resources {
		if resource.Type == "image" && strings.EqualFold(strings.TrimSpace(resource.LoadingAttr), "lazy") {
			total++
		}
	}
	return total
}
func dominantIconFont(resources []enrichedResource) *enrichedResource {
	var candidate *enrichedResource
	for index := range resources {
		resource := &resources[index]
		if !isIconFontResource(*resource) {
			continue
		}
		if candidate == nil || resource.Bytes > candidate.Bytes {
			candidate = resource
		}
	}
	return candidate
}
func isIconFontResource(resource enrichedResource) bool {
	if resource.Type != "font" {
		return false
	}
	haystack := strings.ToLower(strings.TrimSpace(resource.URL + " " + resource.Hostname))
	return containsAny(haystack, "font-awesome", "fontawesome", "fa-solid", "fa-regular", "fa-brands", "materialicons", "material-icons")
}
func naturalToRenderedRatio(resource enrichedResource) float64 {
	if resource.BoundingBox == nil || resource.BoundingBox.Width <= 0 || resource.BoundingBox.Height <= 0 {
		return 0
	}
	widthRatio := 0.0
	heightRatio := 0.0
	if resource.NaturalWidth > 0 {
		widthRatio = float64(resource.NaturalWidth) / resource.BoundingBox.Width
	}
	if resource.NaturalHeight > 0 {
		heightRatio = float64(resource.NaturalHeight) / resource.BoundingBox.Height
	}
	return math.Max(widthRatio, heightRatio)
}
func medianNaturalToRenderedRatio(resources []enrichedResource) float64 {
	ratios := make([]float64, 0, len(resources))
	for _, resource := range resources {
		ratio := naturalToRenderedRatio(resource)
		if ratio > 0 {
			ratios = append(ratios, ratio)
		}
	}
	if len(ratios) == 0 {
		return 0
	}
	sort.Float64s(ratios)
	middle := len(ratios) / 2
	if len(ratios)%2 == 1 {
		return ratios[middle]
	}
	return (ratios[middle-1] + ratios[middle]) / 2
}
func humanRatio(value float64) string {
	if value <= 0 {
		return "sin datos"
	}
	return fmt.Sprintf("%.1fx", value)
}
func boundingWidth(box *BoundingBox) float64 {
	if box == nil {
		return 0
	}
	return box.Width
}
func boundingHeight(box *BoundingBox) float64 {
	if box == nil {
		return 0
	}
	return box.Height
}
