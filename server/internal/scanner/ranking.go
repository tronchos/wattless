package scanner

import (
	"fmt"
	"sort"
	"strings"
)

const (
	materialVampireBytes     int64 = 32_000
	materialVampireSavings   int64 = 16_000
	minGuaranteedVisualVamps       = 2
	promotedAnchorMinSavings int64 = 60_000
)

func rankVampireResources(resources []enrichedResource, groups []ResourceGroup, findings []AnalysisFinding, totalBytes int64) ([]enrichedResource, []string) {
	sorted := make([]enrichedResource, 0, len(resources))
	for _, resource := range resources {
		if isSuppressedVampireResource(resource) {
			continue
		}
		sorted = append(sorted, resource)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return vampireRankLess(sorted[i], sorted[j])
	})

	resourceByID := make(map[string]enrichedResource, len(resources))
	rankIndex := make(map[string]int, len(sorted))
	for index, resource := range sorted {
		resourceByID[resource.ID] = resource
		rankIndex[resource.ID] = index
	}

	groupRefsByResourceID := buildVampireGroupRefs(groups)
	selected := make([]enrichedResource, 0, 5)
	selectedIDs := make(map[string]struct{}, 5)
	selectedGroupCounts := make(map[string]int, len(groups))
	seededIDs := make(map[string]struct{}, 3)

	addSelected := func(resource enrichedResource) bool {
		if _, ok := selectedIDs[resource.ID]; ok {
			return false
		}
		if !canSelectVampire(resource, groupRefsByResourceID, selectedGroupCounts) {
			return false
		}
		selected = append(selected, resource)
		selectedIDs[resource.ID] = struct{}{}
		incrementSelectedGroupCounts(resource, groupRefsByResourceID, selectedGroupCounts)
		return true
	}

	seedRepresentative := func(resource *enrichedResource) {
		if resource == nil {
			return
		}
		if addSelected(*resource) {
			seededIDs[resource.ID] = struct{}{}
		}
	}

	seedRepresentative(selectRepeatedGalleryRepresentative(dominantGroupByKind(groups, groupKindRepeatedGallery), sorted))
	seedRepresentative(selectHighestBytesGroupMember(dominantGroupByKind(groups, groupKindFontCluster), sorted, resourceByID, nil))
	seedRepresentative(selectHighestBytesGroupMember(dominantAnalyticsGroup(groups, resourceByID), sorted, resourceByID, func(resource enrichedResource) bool {
		return resource.IsThirdPartyTool && resource.ThirdPartyKind == thirdPartyAnalytics
	}))

	for _, resource := range sorted {
		if len(selected) >= 5 {
			break
		}
		if len(selected) >= 4 {
			if _, ok := seededIDs[resource.ID]; !ok && !isMaterialVampireCandidate(resource) {
				continue
			}
		}
		if !addSelected(resource) {
			continue
		}
	}

	desiredVisualCount := minInt(minGuaranteedVisualVamps, countMaterialVisualResources(sorted))
	for countVisualResources(selected) < desiredVisualCount {
		replaced := false
		for _, visual := range sorted {
			if visual.BoundingBox == nil {
				continue
			}
			if !isMaterialVampireCandidate(visual) {
				continue
			}
			if _, ok := selectedIDs[visual.ID]; ok {
				continue
			}

			removeIndex := worstSelectedIndex(selected, rankIndex, seededIDs, true)
			if removeIndex < 0 {
				removeIndex = worstSelectedIndex(selected, rankIndex, nil, true)
			}
			if removeIndex < 0 {
				break
			}

			removed := selected[removeIndex]
			decrementSelectedGroupCounts(removed, groupRefsByResourceID, selectedGroupCounts)
			delete(selectedIDs, removed.ID)

			if !canSelectVampire(visual, groupRefsByResourceID, selectedGroupCounts) {
				selectedIDs[removed.ID] = struct{}{}
				incrementSelectedGroupCounts(removed, groupRefsByResourceID, selectedGroupCounts)
				continue
			}

			selected[removeIndex] = visual
			selectedIDs[visual.ID] = struct{}{}
			incrementSelectedGroupCounts(visual, groupRefsByResourceID, selectedGroupCounts)
			replaced = true
			break
		}
		if !replaced {
			break
		}
	}

	selected, selectedIDs, selectedGroupCounts = promoteFindingAnchors(
		selected,
		selectedIDs,
		selectedGroupCounts,
		seededIDs,
		sorted,
		rankIndex,
		resourceByID,
		groupRefsByResourceID,
		findings,
	)

	sort.Slice(selected, func(i, j int) bool {
		return vampireRankLess(selected[i], selected[j])
	})
	if len(selected) > 5 {
		selected = selected[:5]
	}

	warnings := []string{}
	failedRequests := 0
	failedRanked := 0
	for _, resource := range resources {
		if resourceFailed(resource) {
			failedRequests++
		}
	}
	for _, resource := range selected {
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

	visualCandidates := countVisualResources(sorted)
	visualMapped := countVisualResources(selected)
	if visualCandidates == 0 && len(selected) > 0 {
		warnings = append(warnings, "The heaviest resources could not be mapped to visible DOM boxes.")
	} else if visualCandidates > 0 && visualMapped == 0 {
		warnings = append(warnings, "The vampire ranking is dominated by non-visual resources; visual anchors are limited in the overlay.")
	}

	return selected, warnings
}
func isMaterialVampireCandidate(resource enrichedResource) bool {
	if isSuppressedVampireResource(resource) {
		return false
	}
	if vampirePriority(resource) >= 55 {
		return true
	}
	return resource.Bytes >= materialVampireBytes || estimateResourceSavings(resource) >= materialVampireSavings
}
func isSuppressedVampireResource(resource enrichedResource) bool {
	if isSuppressedHTMLVampireResource(resource) {
		return true
	}
	if resource.BoundingBox != nil && resource.BoundingBox.Width <= 2 && resource.BoundingBox.Height <= 2 {
		return true
	}
	if resource.Type == "image" && resource.BoundingBox != nil && resource.Bytes < 8_000 &&
		resource.BoundingBox.Width*resource.BoundingBox.Height < 3_000 &&
		resource.VisualRole != visualRoleLCPCandidate &&
		resource.VisualRole != visualRoleHeroMedia {
		return true
	}
	return false
}
func isSuppressedHTMLVampireResource(resource enrichedResource) bool {
	mimeType := strings.ToLower(strings.TrimSpace(resource.MIMEType))
	if !strings.Contains(mimeType, "html") {
		return false
	}
	return resource.Type == "fetch" || resource.Type == "other"
}
func vampirePriority(resource enrichedResource) int {
	priority := 10
	switch resource.VisualRole {
	case visualRoleLCPCandidate:
		priority = 90
	case visualRoleHeroMedia:
		priority = 80
	case visualRoleAboveFoldMedia:
		priority = 70
	case visualRoleRepeatedCard:
		priority = 55
	case visualRoleBelowFoldMedia:
		priority = 45
	}

	if resource.Type == "font" {
		priority = maxInt(priority, 65)
	}
	if resource.IsThirdPartyTool && resource.ThirdPartyKind == thirdPartyAnalytics {
		priority = maxInt(priority, 60)
	}
	if resource.Type == "script" && resource.PositionBand == positionAboveFold {
		priority = maxInt(priority, 62)
	}

	return priority
}
func vampireRankLess(left, right enrichedResource) bool {
	leftPriority := vampirePriority(left)
	rightPriority := vampirePriority(right)
	if leftPriority == rightPriority {
		leftSavings := estimateResourceSavings(left)
		rightSavings := estimateResourceSavings(right)
		if leftSavings == rightSavings {
			if left.Bytes == right.Bytes {
				return left.ID < right.ID
			}
			return left.Bytes > right.Bytes
		}
		return leftSavings > rightSavings
	}
	return leftPriority > rightPriority
}

type vampireGroupRef struct {
	ID   string
	Kind string
}

func buildVampireGroupRefs(groups []ResourceGroup) map[string][]vampireGroupRef {
	refsByResourceID := make(map[string][]vampireGroupRef)
	for _, group := range groups {
		ref := vampireGroupRef{ID: group.ID, Kind: group.Kind}
		for _, resourceID := range group.RelatedResourceIDs {
			refsByResourceID[resourceID] = append(refsByResourceID[resourceID], ref)
		}
	}
	return refsByResourceID
}
func dominantGroupByKind(groups []ResourceGroup, kind string) *ResourceGroup {
	var candidate *ResourceGroup
	for index := range groups {
		group := &groups[index]
		if group.Kind != kind {
			continue
		}
		if candidate == nil || group.TotalBytes > candidate.TotalBytes {
			candidate = group
		}
	}
	return candidate
}
func dominantAnalyticsGroup(groups []ResourceGroup, resourcesByID map[string]enrichedResource) *ResourceGroup {
	var candidate *ResourceGroup
	for index := range groups {
		group := &groups[index]
		if group.Kind != groupKindThirdParty {
			continue
		}
		hasAnalytics := false
		for _, resourceID := range group.RelatedResourceIDs {
			resource, ok := resourcesByID[resourceID]
			if ok && resource.IsThirdPartyTool && resource.ThirdPartyKind == thirdPartyAnalytics {
				hasAnalytics = true
				break
			}
		}
		if !hasAnalytics {
			continue
		}
		if candidate == nil || group.TotalBytes > candidate.TotalBytes {
			candidate = group
		}
	}
	return candidate
}
func selectRepeatedGalleryRepresentative(group *ResourceGroup, sorted []enrichedResource) *enrichedResource {
	if group == nil {
		return nil
	}
	ids := make(map[string]struct{}, len(group.RelatedResourceIDs))
	for _, resourceID := range group.RelatedResourceIDs {
		ids[resourceID] = struct{}{}
	}
	for _, resource := range sorted {
		if _, ok := ids[resource.ID]; !ok {
			continue
		}
		if resource.BoundingBox != nil {
			candidate := resource
			return &candidate
		}
	}
	for _, resource := range sorted {
		if _, ok := ids[resource.ID]; ok {
			candidate := resource
			return &candidate
		}
	}
	return nil
}
func selectHighestBytesGroupMember(group *ResourceGroup, sorted []enrichedResource, resourcesByID map[string]enrichedResource, predicate func(enrichedResource) bool) *enrichedResource {
	if group == nil {
		return nil
	}

	rankIndex := make(map[string]int, len(sorted))
	for index, resource := range sorted {
		rankIndex[resource.ID] = index
	}

	var candidate *enrichedResource
	for _, resourceID := range group.RelatedResourceIDs {
		resource, ok := resourcesByID[resourceID]
		if !ok {
			continue
		}
		if predicate != nil && !predicate(resource) {
			continue
		}
		if candidate == nil || resource.Bytes > candidate.Bytes || (resource.Bytes == candidate.Bytes && rankIndex[resource.ID] < rankIndex[candidate.ID]) {
			copy := resource
			candidate = &copy
		}
	}
	return candidate
}
func canSelectVampire(resource enrichedResource, groupRefsByResourceID map[string][]vampireGroupRef, selectedGroupCounts map[string]int) bool {
	for _, ref := range groupRefsByResourceID[resource.ID] {
		cap := vampireGroupCap(ref.Kind)
		if cap > 0 && selectedGroupCounts[ref.ID] >= cap {
			return false
		}
	}
	return true
}
func incrementSelectedGroupCounts(resource enrichedResource, groupRefsByResourceID map[string][]vampireGroupRef, selectedGroupCounts map[string]int) {
	for _, ref := range groupRefsByResourceID[resource.ID] {
		selectedGroupCounts[ref.ID]++
	}
}
func decrementSelectedGroupCounts(resource enrichedResource, groupRefsByResourceID map[string][]vampireGroupRef, selectedGroupCounts map[string]int) {
	for _, ref := range groupRefsByResourceID[resource.ID] {
		if selectedGroupCounts[ref.ID] <= 1 {
			delete(selectedGroupCounts, ref.ID)
			continue
		}
		selectedGroupCounts[ref.ID]--
	}
}
func vampireGroupCap(kind string) int {
	switch kind {
	case groupKindRepeatedGallery:
		return 2
	case groupKindFontCluster, groupKindThirdParty:
		return 1
	default:
		return 0
	}
}
func countVisualResources(resources []enrichedResource) int {
	count := 0
	for _, resource := range resources {
		if resource.BoundingBox != nil && !isSuppressedVampireResource(resource) {
			count++
		}
	}
	return count
}
func countMaterialVisualResources(resources []enrichedResource) int {
	count := 0
	for _, resource := range resources {
		if resource.BoundingBox == nil {
			continue
		}
		if isMaterialVampireCandidate(resource) {
			count++
		}
	}
	return count
}
func worstSelectedIndex(selected []enrichedResource, rankIndex map[string]int, protectedIDs map[string]struct{}, nonVisualOnly bool) int {
	worstIndex := -1
	worstRank := -1
	for index, resource := range selected {
		if nonVisualOnly && resource.BoundingBox != nil {
			continue
		}
		if protectedIDs != nil {
			if _, ok := protectedIDs[resource.ID]; ok {
				continue
			}
		}
		resourceRank := rankIndex[resource.ID]
		if resourceRank > worstRank {
			worstRank = resourceRank
			worstIndex = index
		}
	}
	return worstIndex
}
func promoteFindingAnchors(
	selected []enrichedResource,
	selectedIDs map[string]struct{},
	selectedGroupCounts map[string]int,
	seededIDs map[string]struct{},
	sorted []enrichedResource,
	rankIndex map[string]int,
	resourceByID map[string]enrichedResource,
	groupRefsByResourceID map[string][]vampireGroupRef,
	findings []AnalysisFinding,
) ([]enrichedResource, map[string]struct{}, map[string]int) {
	protectedIDs := make(map[string]struct{}, len(seededIDs))
	for id := range seededIDs {
		protectedIDs[id] = struct{}{}
	}

	for _, finding := range findings {
		if !isPromotableAnchorFinding(finding.ID) {
			continue
		}
		if selectedContainsAnyID(selectedIDs, finding.RelatedResourceIDs) {
			continue
		}

		candidate := selectPromotableFindingResource(finding, sorted, resourceByID)
		if candidate == nil {
			continue
		}
		if _, ok := selectedIDs[candidate.ID]; ok {
			protectedIDs[candidate.ID] = struct{}{}
			continue
		}

		if len(selected) < 5 && canSelectVampire(*candidate, groupRefsByResourceID, selectedGroupCounts) {
			selected = append(selected, *candidate)
			selectedIDs[candidate.ID] = struct{}{}
			incrementSelectedGroupCounts(*candidate, groupRefsByResourceID, selectedGroupCounts)
			protectedIDs[candidate.ID] = struct{}{}
			continue
		}

		removalIndices := promotionRemovalIndices(selected, rankIndex, protectedIDs, *candidate, groupRefsByResourceID)
		for _, removeIndex := range removalIndices {
			removed := selected[removeIndex]
			decrementSelectedGroupCounts(removed, groupRefsByResourceID, selectedGroupCounts)
			delete(selectedIDs, removed.ID)

			if !canSelectVampire(*candidate, groupRefsByResourceID, selectedGroupCounts) {
				selectedIDs[removed.ID] = struct{}{}
				incrementSelectedGroupCounts(removed, groupRefsByResourceID, selectedGroupCounts)
				continue
			}

			selected[removeIndex] = *candidate
			selectedIDs[candidate.ID] = struct{}{}
			incrementSelectedGroupCounts(*candidate, groupRefsByResourceID, selectedGroupCounts)
			protectedIDs[candidate.ID] = struct{}{}
			break
		}
	}

	return selected, selectedIDs, selectedGroupCounts
}
func isPromotableAnchorFinding(id string) bool {
	switch id {
	case "responsive_image_overdelivery", "repeated_gallery_overdelivery", "dominant_image_overdelivery":
		return true
	default:
		return false
	}
}
func selectedContainsAnyID(selectedIDs map[string]struct{}, ids []string) bool {
	for _, id := range ids {
		if _, ok := selectedIDs[id]; ok {
			return true
		}
	}
	return false
}
func selectPromotableFindingResource(finding AnalysisFinding, sorted []enrichedResource, resourceByID map[string]enrichedResource) *enrichedResource {
	if len(finding.RelatedResourceIDs) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(finding.RelatedResourceIDs))
	for _, id := range finding.RelatedResourceIDs {
		allowed[id] = struct{}{}
	}

	var fallback *enrichedResource
	for _, resource := range sorted {
		if _, ok := allowed[resource.ID]; !ok {
			continue
		}
		if resource.BoundingBox == nil {
			continue
		}
		if estimateResourceSavings(resource) < promotedAnchorMinSavings {
			continue
		}
		copy := resource
		return &copy
	}

	for _, id := range finding.RelatedResourceIDs {
		resource, ok := resourceByID[id]
		if !ok || resource.BoundingBox == nil {
			continue
		}
		if estimateResourceSavings(resource) < promotedAnchorMinSavings {
			continue
		}
		copy := resource
		fallback = &copy
		break
	}

	return fallback
}
func promotionRemovalIndices(
	selected []enrichedResource,
	rankIndex map[string]int,
	protectedIDs map[string]struct{},
	candidate enrichedResource,
	groupRefsByResourceID map[string][]vampireGroupRef,
) []int {
	indices := make([]int, 0, len(selected))
	for index, resource := range selected {
		if _, ok := protectedIDs[resource.ID]; ok {
			continue
		}
		indices = append(indices, index)
	}

	sort.Slice(indices, func(i, j int) bool {
		left := selected[indices[i]]
		right := selected[indices[j]]
		leftShared := sharesVampireGroup(left, candidate, groupRefsByResourceID)
		rightShared := sharesVampireGroup(right, candidate, groupRefsByResourceID)
		if leftShared != rightShared {
			return leftShared
		}
		leftVisual := left.BoundingBox != nil
		rightVisual := right.BoundingBox != nil
		if leftVisual != rightVisual {
			return !leftVisual
		}
		return rankIndex[left.ID] > rankIndex[right.ID]
	})

	return indices
}
func sharesVampireGroup(left, right enrichedResource, groupRefsByResourceID map[string][]vampireGroupRef) bool {
	leftRefs := groupRefsByResourceID[left.ID]
	rightRefs := groupRefsByResourceID[right.ID]
	if len(leftRefs) == 0 || len(rightRefs) == 0 {
		return false
	}
	lookup := make(map[string]struct{}, len(leftRefs))
	for _, ref := range leftRefs {
		lookup[ref.ID] = struct{}{}
	}
	for _, ref := range rightRefs {
		if _, ok := lookup[ref.ID]; ok {
			return true
		}
	}
	return false
}
