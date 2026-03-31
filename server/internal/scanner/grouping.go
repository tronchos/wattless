package scanner

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"sort"
	"strings"
)

func enrichResourcesForAnalysis(resources []enrichedResource, perf PerformanceMetrics, viewportWidth, viewportHeight int) ([]enrichedResource, []ResourceGroup) {
	annotated := append([]enrichedResource(nil), resources...)
	for index := range annotated {
		annotated[index].PositionBand = classifyPositionBand(annotated[index].BoundingBox, annotated[index].VisibleRatio, viewportHeight)
		annotated[index].ThirdPartyKind = classifyThirdPartyKind(annotated[index])
		annotated[index].IsThirdPartyTool = annotated[index].Party == partyThird && annotated[index].ThirdPartyKind != thirdPartyUnknown
	}

	groups := buildResourceGroups(annotated, viewportHeight)
	repeatedGalleryMembers := make(map[string]struct{})
	for _, group := range groups {
		if group.Kind != groupKindRepeatedGallery {
			continue
		}
		for _, resourceID := range group.RelatedResourceIDs {
			repeatedGalleryMembers[resourceID] = struct{}{}
		}
	}

	for index := range annotated {
		annotated[index].VisualRole = classifyVisualRole(annotated[index], perf, viewportWidth, viewportHeight, repeatedGalleryMembers)
	}

	return annotated, groups
}
func buildResourceGroups(resources []enrichedResource, viewportHeight int) []ResourceGroup {
	groups := make([]ResourceGroup, 0)
	repeatedGroups := map[string][]enrichedResource{}

	for _, resource := range resources {
		if resource.Type != "image" || resource.BoundingBox == nil {
			continue
		}
		key := repeatedGalleryKey(resource)
		if key == "" {
			continue
		}
		repeatedGroups[key] = append(repeatedGroups[key], resource)
	}

	for key, members := range repeatedGroups {
		if len(members) < 3 {
			continue
		}
		group := ResourceGroup{
			ID:                 "group-" + sanitizeGroupID(key),
			Kind:               groupKindRepeatedGallery,
			Label:              repeatedGalleryLabel(members, viewportHeight),
			TotalBytes:         sumBytes(members),
			ResourceCount:      len(members),
			PositionBand:       summarizePositionBand(members),
			RelatedResourceIDs: collectResourceIDs(members),
		}
		groups = append(groups, group)
	}

	fonts := filterResources(resources, func(resource enrichedResource) bool {
		return resource.Type == "font"
	})
	if len(fonts) >= 2 {
		groups = append(groups, ResourceGroup{
			ID:                 "group-font-stack",
			Kind:               groupKindFontCluster,
			Label:              "Stack tipográfico",
			TotalBytes:         sumBytes(fonts),
			ResourceCount:      len(fonts),
			PositionBand:       positionUnknown,
			RelatedResourceIDs: collectResourceIDs(fonts),
		})
	}

	thirdPartyByKind := map[string][]enrichedResource{}
	for _, resource := range resources {
		if !resource.IsThirdPartyTool {
			continue
		}
		thirdPartyByKind[resource.ThirdPartyKind] = append(thirdPartyByKind[resource.ThirdPartyKind], resource)
	}
	for kind, members := range thirdPartyByKind {
		if len(members) == 0 {
			continue
		}
		groups = append(groups, ResourceGroup{
			ID:                 "group-third-party-" + sanitizeGroupID(kind),
			Kind:               groupKindThirdParty,
			Label:              thirdPartyGroupLabel(kind),
			TotalBytes:         sumBytes(members),
			ResourceCount:      len(members),
			PositionBand:       summarizePositionBand(members),
			RelatedResourceIDs: collectResourceIDs(members),
		})
	}

	disambiguateDuplicateGroupLabels(groups, resources)

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].TotalBytes == groups[j].TotalBytes {
			return groups[i].ID < groups[j].ID
		}
		return groups[i].TotalBytes > groups[j].TotalBytes
	})

	return groups
}
func disambiguateDuplicateGroupLabels(groups []ResourceGroup, resources []enrichedResource) {
	labelBuckets := make(map[string][]int)
	for index, group := range groups {
		label := strings.TrimSpace(group.Label)
		if label == "" {
			continue
		}
		labelBuckets[label] = append(labelBuckets[label], index)
	}

	for label, indices := range labelBuckets {
		if len(indices) < 2 {
			continue
		}
		sort.Slice(indices, func(i, j int) bool {
			leftY := groupFirstVisualY(groups[indices[i]], resources)
			rightY := groupFirstVisualY(groups[indices[j]], resources)
			if leftY == rightY {
				return groups[indices[i]].ID < groups[indices[j]].ID
			}
			return leftY < rightY
		})
		for offset, groupIndex := range indices {
			groups[groupIndex].Label = fmt.Sprintf("%s (%d)", label, offset+1)
		}
	}
}
func groupFirstVisualY(group ResourceGroup, resources []enrichedResource) float64 {
	minY := math.MaxFloat64
	found := false
	for _, id := range group.RelatedResourceIDs {
		resource := resourceForID(resources, id)
		if resource == nil || resource.BoundingBox == nil {
			continue
		}
		if !found || resource.BoundingBox.Y < minY {
			minY = resource.BoundingBox.Y
			found = true
		}
	}
	if !found {
		return math.MaxFloat64
	}
	return minY
}
func repeatedGalleryKey(resource enrichedResource) string {
	if resource.BoundingBox == nil {
		return ""
	}
	dir := semanticPathPrefix(resource.URL)
	if dir == "" {
		return ""
	}
	widthBucket := int(math.Round(resource.BoundingBox.Width / 48))
	heightBucket := int(math.Round(resource.BoundingBox.Height / 48))
	return fmt.Sprintf("%s|%s|%d|%d", resource.Hostname, dir, widthBucket, heightBucket)
}
func semanticPathPrefix(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	dir := path.Dir(parsed.Path)
	segments := strings.Split(strings.Trim(dir, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		return "/"
	}
	if len(segments) == 1 {
		return "/" + segments[0]
	}
	return "/" + strings.Join(segments[:2], "/")
}
func sanitizeGroupID(value string) string {
	value = strings.ToLower(value)
	value = strings.NewReplacer("/", "-", "|", "-", ".", "-", " ", "-").Replace(value)
	value = strings.Trim(value, "-")
	if value == "" {
		return "group"
	}
	return value
}
func repeatedGalleryLabel(resources []enrichedResource, viewportHeight int) string {
	if len(resources) > 0 {
		if label := semanticGalleryLabel(semanticPathPrefix(resources[0].URL)); label != "" {
			return label
		}
	}
	if isLikelySpeakerGallery(resources) {
		return "Fotos de speakers"
	}
	if isLikelyLogoGallery(resources) {
		return "Logos de sponsors"
	}
	belowFold := 0
	for _, resource := range resources {
		band := resource.PositionBand
		if band == "" {
			band = classifyPositionBand(resource.BoundingBox, resource.VisibleRatio, viewportHeight)
		}
		if band == positionBelowFold {
			belowFold++
		}
	}
	switch {
	case len(resources) >= 4 && belowFold >= len(resources)/2:
		return "Colección de miniaturas"
	case len(resources) >= 3:
		return "Grid de tarjetas"
	default:
		return "Galería repetida de imágenes"
	}
}
func semanticGalleryLabel(prefix string) string {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	switch {
	case containsAny(prefix, "/img/blog", "/blog", "/posts", "/articles"):
		return "Miniaturas del blog"
	case containsAny(prefix, "/sponsors"):
		return "Logos de sponsors"
	case containsAny(prefix, "/speakers", "/speaker", "/lineup", "/talks"):
		return "Fotos de speakers"
	case containsAny(prefix, "/partners", "/clients", "/logos"):
		return "Logos de partners"
	case containsAny(prefix, "/flags"):
		return "Banderas"
	case containsAny(prefix, "/avatars", "/team", "/testimonials"):
		return "Avatares"
	default:
		return ""
	}
}
func isLikelySpeakerGallery(resources []enrichedResource) bool {
	if len(resources) < 8 {
		return false
	}

	portraitLike := 0
	nameLike := 0
	for _, resource := range resources {
		if resource.BoundingBox != nil && resource.BoundingBox.Width > 0 && resource.BoundingBox.Height > 0 {
			aspect := resource.BoundingBox.Height / resource.BoundingBox.Width
			if aspect >= 0.9 && aspect <= 1.4 {
				portraitLike++
			}
		}
		if looksLikePersonalSlug(resource.URL) {
			nameLike++
		}
	}

	required := int(math.Ceil(float64(len(resources)) * 0.6))
	return portraitLike >= required && nameLike >= required
}
func isLikelyLogoGallery(resources []enrichedResource) bool {
	if len(resources) < 4 {
		return false
	}

	wideCount := 0
	lightCount := 0
	for _, resource := range resources {
		if resource.BoundingBox != nil && resource.BoundingBox.Width > 0 && resource.BoundingBox.Height > 0 {
			aspect := resource.BoundingBox.Width / resource.BoundingBox.Height
			if aspect >= 2.5 {
				wideCount++
			}
		}
		if resource.Bytes > 0 && resource.Bytes <= 50_000 {
			lightCount++
		}
	}

	required := int(math.Ceil(float64(len(resources)) * 0.6))
	return wideCount >= required && lightCount >= required
}
func looksLikePersonalSlug(rawURL string) bool {
	base := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(path.Base(resourcePath(rawURL)), path.Ext(resourcePath(rawURL)))))
	if base == "" {
		return false
	}
	segments := strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '_'
	})
	if len(segments) < 2 || len(segments) > 3 {
		return false
	}
	for _, segment := range segments {
		if len(segment) < 2 {
			return false
		}
		for _, r := range segment {
			if r < 'a' || r > 'z' {
				return false
			}
		}
	}
	return true
}
func thirdPartyGroupLabel(kind string) string {
	switch kind {
	case thirdPartyAnalytics:
		return "Cluster de analítica"
	case thirdPartyAds:
		return "Cluster de anuncios"
	case thirdPartySupport:
		return "Cluster de soporte"
	case thirdPartySocial:
		return "Cluster social"
	case thirdPartyVideo:
		return "Embeds de video"
	case thirdPartyPayment:
		return "Cluster de pagos"
	default:
		return "Terceros"
	}
}
func summarizePositionBand(resources []enrichedResource) PositionBand {
	counts := map[PositionBand]int{}
	for _, resource := range resources {
		counts[resource.PositionBand]++
	}
	if len(counts) == 0 {
		return positionUnknown
	}
	if len(counts) == 1 {
		for band := range counts {
			return band
		}
	}
	maxBand := positionUnknown
	maxCount := 0
	for _, band := range []PositionBand{
		positionAboveFold,
		positionNearFold,
		positionBelowFold,
		positionUnknown,
	} {
		count := counts[band]
		if count > maxCount {
			maxBand = band
			maxCount = count
		}
	}
	if maxCount >= len(resources)-1 {
		return maxBand
	}
	return PositionBandMixed
}
func collectRepeatedGalleryMembers(groups []ResourceGroup) map[string]struct{} {
	output := make(map[string]struct{})
	for _, group := range groups {
		if group.Kind != groupKindRepeatedGallery {
			continue
		}
		for _, resourceID := range group.RelatedResourceIDs {
			output[resourceID] = struct{}{}
		}
	}
	return output
}
