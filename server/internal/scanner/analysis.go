package scanner

import (
	"fmt"
	"math"
	"net"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/tronchos/wattless/server/internal/insights"
	"golang.org/x/net/publicsuffix"
)

const (
	partyFirst = "first_party"
	partyThird = "third_party"

	positionAboveFold = "above_fold"
	positionNearFold  = "near_fold"
	positionBelowFold = "below_fold"
	positionUnknown   = "unknown"

	visualRoleLCPCandidate   = "lcp_candidate"
	visualRoleHeroMedia      = "hero_media"
	visualRoleRepeatedCard   = "repeated_card_media"
	visualRoleAboveFoldMedia = "above_fold_media"
	visualRoleBelowFoldMedia = "below_fold_media"
	visualRoleDecorative     = "decorative"
	visualRoleUnknown        = "unknown"

	groupKindRepeatedGallery = "repeated_gallery"
	groupKindThirdParty      = "third_party_cluster"
	groupKindFontCluster     = "font_cluster"

	thirdPartyAnalytics = "analytics"
	thirdPartyAds       = "ads"
	thirdPartySupport   = "support"
	thirdPartySocial    = "social"
	thirdPartyVideo     = "video_embed"
	thirdPartyPayment   = "payment"
	thirdPartyUnknown   = "unknown"
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

func classifyPositionBand(box *BoundingBox, _ float64, viewportHeight int) string {
	if box == nil || viewportHeight <= 0 || box.Height <= 0 {
		return positionUnknown
	}

	top := box.Y
	bottom := box.Y + box.Height
	firstViewportBottom := float64(viewportHeight)
	secondViewportTop := firstViewportBottom
	secondViewportBottom := float64(viewportHeight * 2)
	firstViewportVisibleHeight := math.Min(bottom, firstViewportBottom) - math.Max(top, 0)
	if firstViewportVisibleHeight < 0 {
		firstViewportVisibleHeight = 0
	}

	switch {
	case firstViewportVisibleHeight >= box.Height*0.50:
		return positionAboveFold
	case firstViewportVisibleHeight > 0:
		return positionNearFold
	case top < secondViewportBottom && bottom > secondViewportTop:
		return positionNearFold
	default:
		return positionBelowFold
	}
}

func classifyThirdPartyKind(resource enrichedResource) string {
	if resource.Party != partyThird {
		return thirdPartyUnknown
	}

	haystack := strings.ToLower(resource.Hostname + " " + resource.URL)
	switch {
	case containsAny(haystack, "posthog", "segment", "mixpanel", "plausible", "clarity", "amplitude", "ahrefs", "google-analytics", "googletagmanager", "gtm.js"):
		return thirdPartyAnalytics
	case containsAny(haystack, "doubleclick", "ads", "adservice", "googleadservices"):
		return thirdPartyAds
	case containsAny(haystack, "intercom", "zendesk", "drift", "crisp", "helpscout"):
		return thirdPartySupport
	case containsAny(haystack, "facebook", "twitter", "x.com", "linkedin", "tiktok", "instagram"):
		return thirdPartySocial
	case containsAny(haystack, "youtube", "youtu.be", "vimeo", "loom"):
		return thirdPartyVideo
	case containsAny(haystack, "stripe", "paypal", "mercadopago", "checkout"):
		return thirdPartyPayment
	default:
		return thirdPartyUnknown
	}
}

func containsAny(value string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}

func classifyVisualRole(resource enrichedResource, perf PerformanceMetrics, viewportWidth, viewportHeight int, repeatedGalleryMembers map[string]struct{}) string {
	if perf.LCPResourceURL != "" && sameAsset(resource.URL, perf.LCPResourceURL) {
		return visualRoleLCPCandidate
	}

	if !isVisualResource(resource) {
		return visualRoleUnknown
	}

	if resource.BoundingBox == nil {
		return visualRoleUnknown
	}

	area := resource.BoundingBox.Width * resource.BoundingBox.Height
	viewportArea := float64(maxInt(viewportWidth, 1) * maxInt(viewportHeight, 1))

	if resource.Type == "image" {
		if _, ok := repeatedGalleryMembers[resource.ID]; ok {
			return visualRoleRepeatedCard
		}
	}

	if resource.PositionBand == positionAboveFold &&
		resource.BoundingBox.Width >= float64(viewportWidth)*0.40 &&
		area >= viewportArea*0.20 {
		return visualRoleHeroMedia
	}

	if resource.PositionBand == positionAboveFold {
		return visualRoleAboveFoldMedia
	}

	if resource.PositionBand == positionNearFold || resource.PositionBand == positionBelowFold {
		return visualRoleBelowFoldMedia
	}

	if viewportArea > 0 && area < viewportArea*0.01 {
		return visualRoleDecorative
	}

	return visualRoleUnknown
}

func isVisualResource(resource enrichedResource) bool {
	return resource.Type == "image" || resource.Type == "video"
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

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].TotalBytes == groups[j].TotalBytes {
			return groups[i].ID < groups[j].ID
		}
		return groups[i].TotalBytes > groups[j].TotalBytes
	})

	return groups
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

func summarizePositionBand(resources []enrichedResource) string {
	counts := map[string]int{}
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
	for _, band := range []string{
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
	return "mixed"
}

func buildAnalysis(resources []enrichedResource, perf PerformanceMetrics, groups []ResourceGroup) Analysis {
	summary := AnalysisSummary{}

	for _, resource := range resources {
		switch resource.PositionBand {
		case positionAboveFold:
			summary.AboveFoldBytes += resource.Bytes
		case positionNearFold, positionBelowFold:
			summary.BelowFoldBytes += resource.Bytes
		}

		if resource.Type == "font" {
			summary.FontBytes += resource.Bytes
			summary.FontRequests++
		}
		if resource.IsThirdPartyTool && resource.ThirdPartyKind == thirdPartyAnalytics {
			summary.AnalyticsBytes += resource.Bytes
			summary.AnalyticsRequests++
		}
		if resource.VisualRole == visualRoleLCPCandidate || resource.VisualRole == visualRoleHeroMedia || resource.VisualRole == visualRoleAboveFoldMedia {
			summary.RenderCriticalBytes += resource.Bytes
		}
		if resource.Type == "font" || resource.Type == "stylesheet" {
			summary.RenderCriticalBytes += resource.Bytes
		}
	}

	var lcpResource *enrichedResource
	for index := range resources {
		if perf.LCPResourceURL != "" && sameAsset(resources[index].URL, perf.LCPResourceURL) {
			lcpResource = &resources[index]
			break
		}
	}
	if lcpResource != nil {
		summary.LCPResourceID = lcpResource.ID
		summary.LCPResourceURL = lcpResource.URL
		summary.LCPResourceBytes = lcpResource.Bytes
	} else {
		summary.LCPResourceURL = perf.LCPResourceURL
	}

	for _, group := range groups {
		if group.Kind != groupKindRepeatedGallery {
			continue
		}
		summary.RepeatedGalleryBytes += group.TotalBytes
		summary.RepeatedGalleryCount += group.ResourceCount
	}

	findings := make([]AnalysisFinding, 0, 6)
	if finding := buildLCPFinding(resources, lcpResource, perf); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildMainThreadFinding(resources, perf); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildRepeatedGalleryFinding(groups, resources); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildThirdPartyFinding(resources, summary); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildFontFinding(resources, summary); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildResponsiveImageFinding(resources, groups, summary.LCPResourceID); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildHeavyAboveFoldFinding(resources, perf, summary.LCPResourceID); finding != nil {
		findings = append(findings, *finding)
	}

	sort.Slice(findings, func(i, j int) bool {
		if insights.SeverityRank(findings[i].Severity) == insights.SeverityRank(findings[j].Severity) {
			if insights.ConfidenceRank(findings[i].Confidence) == insights.ConfidenceRank(findings[j].Confidence) {
				if findings[i].EstimatedSavingsBytes == findings[j].EstimatedSavingsBytes {
					return findings[i].ID < findings[j].ID
				}
				return findings[i].EstimatedSavingsBytes > findings[j].EstimatedSavingsBytes
			}
			return insights.ConfidenceRank(findings[i].Confidence) > insights.ConfidenceRank(findings[j].Confidence)
		}
		return insights.SeverityRank(findings[i].Severity) > insights.SeverityRank(findings[j].Severity)
	})

	return Analysis{
		Summary:        summary,
		Findings:       findings,
		ResourceGroups: groups,
	}
}

func buildLCPFinding(resources []enrichedResource, resource *enrichedResource, perf PerformanceMetrics) *AnalysisFinding {
	if resource != nil && resource.Bytes >= 150_000 {
		severity := "medium"
		if perf.LCPMS >= 2_000 {
			severity = "high"
		}

		evidence := []string{
			fmt.Sprintf("El recurso que coincide con el LCP pesa %s.", humanBytes(resource.Bytes)),
			fmt.Sprintf("LCP observado: %d ms.", perf.LCPMS),
		}
		if resource.PositionBand != "" && resource.PositionBand != positionUnknown {
			evidence = append(evidence, fmt.Sprintf("Su posición es %s.", strings.ReplaceAll(resource.PositionBand, "_", " ")))
		}
		if perf.LCPResourceTag != "" {
			evidence = append(evidence, fmt.Sprintf("El nodo LCP es <%s>.", perf.LCPResourceTag))
		}

		return &AnalysisFinding{
			ID:                    "render_lcp_candidate",
			Category:              "render",
			Severity:              severity,
			Confidence:            "high",
			Title:                 "Ataca el recurso que domina el LCP",
			Summary:               fmt.Sprintf("La carga crítica está anclada a un recurso de %s. Es el mejor punto de ataque para recortar el render inicial sin tocar el resto del documento.", humanBytes(resource.Bytes)),
			Evidence:              evidence,
			EstimatedSavingsBytes: estimateResourceSavings(*resource),
			RelatedResourceIDs:    []string{resource.ID},
		}
	}

	if perf.LCPResourceTag == "" && perf.LCPSelectorHint == "" {
		return nil
	}

	related := collectTextLCPResourceCandidates(resources, 3)
	severity := "medium"
	if perf.LCPMS >= 2_000 {
		severity = "high"
	}

	nodeLabel := "nodo del DOM"
	if perf.LCPResourceTag != "" {
		nodeLabel = fmt.Sprintf("nodo <%s>", perf.LCPResourceTag)
	}

	evidence := []string{
		fmt.Sprintf("El LCP observado corresponde a un %s sin asset de red asociado.", nodeLabel),
		fmt.Sprintf("LCP observado: %d ms.", perf.LCPMS),
	}
	if perf.LCPSelectorHint != "" {
		evidence = append(evidence, fmt.Sprintf("Selector observado: %s.", perf.LCPSelectorHint))
	}
	if perf.LCPSize > 0 {
		evidence = append(evidence, fmt.Sprintf("Tamaño reportado por LCP: %d px.", perf.LCPSize))
	}
	if len(related) > 0 {
		evidence = append(evidence, "Las palancas probables están en tipografía, CSS crítico y trabajo de CPU del arranque.")
	}

	return &AnalysisFinding{
		ID:                    "render_lcp_dom_node",
		Category:              "render",
		Severity:              severity,
		Confidence:            "medium",
		Title:                 "Revisa CSS, tipografía y CPU del nodo que domina el LCP",
		Summary:               fmt.Sprintf("El LCP real no apunta a una imagen descargada, sino a un %s. Antes de culpar media pesada, conviene revisar fuentes, CSS crítico y presión de CPU en el primer render.", nodeLabel),
		Evidence:              evidence,
		EstimatedSavingsBytes: sumEstimatedSavings(related),
		RelatedResourceIDs:    collectTopResourceIDs(related, 3),
	}
}

func buildRepeatedGalleryFinding(groups []ResourceGroup, resources []enrichedResource) *AnalysisFinding {
	candidate := dominantRepeatedGalleryGroup(groups)
	if candidate == nil {
		return nil
	}

	members := resourcesForIDs(resources, candidate.RelatedResourceIDs)
	missingResponsive := countNonResponsiveImages(members)
	medianRatio := medianNaturalToRenderedRatio(members)

	title := "Comprime la galería repetida del catálogo"
	summary := fmt.Sprintf("%s suma %s. No todo este bloque es crítico para el viewport inicial, pero sí infla el coste por visita y multiplica el peso del catálogo visual.", candidate.Label, humanBytes(candidate.TotalBytes))

	switch candidate.PositionBand {
	case positionBelowFold:
		title = "Comprime la galería que vive bajo el fold"
		summary = fmt.Sprintf("%s suma %s. No frena el primer render, pero sí infla el coste por visita y multiplica el peso del catálogo visual.", candidate.Label, humanBytes(candidate.TotalBytes))
	case positionNearFold:
		title = "Comprime la galería repetida cerca del fold"
		summary = fmt.Sprintf("%s suma %s. Parte de ese catálogo entra pronto en pantalla, pero su volumen sigue inflando el coste por visita más allá del primer viewport.", candidate.Label, humanBytes(candidate.TotalBytes))
	}

	return &AnalysisFinding{
		ID:         "repeated_gallery_overdelivery",
		Category:   "media",
		Severity:   "medium",
		Confidence: "high",
		Title:      title,
		Summary:    summary,
		Evidence: []string{
			fmt.Sprintf("Grupo detectado: %s.", candidate.Label),
			fmt.Sprintf("Transferencia agregada: %s en %d recursos.", humanBytes(candidate.TotalBytes), candidate.ResourceCount),
			fmt.Sprintf("Posición dominante: %s.", strings.ReplaceAll(candidate.PositionBand, "_", " ")),
			fmt.Sprintf("Imágenes sin srcset/sizes: %d de %d.", missingResponsive, len(members)),
			fmt.Sprintf("Mediana natural/rendered: %s.", humanRatio(medianRatio)),
		},
		EstimatedSavingsBytes: sumEstimatedSavingsForIDs(resources, candidate.RelatedResourceIDs),
		RelatedResourceIDs:    append([]string(nil), candidate.RelatedResourceIDs...),
	}
}

func buildThirdPartyFinding(resources []enrichedResource, summary AnalysisSummary) *AnalysisFinding {
	if summary.AnalyticsRequests < 2 && summary.AnalyticsBytes < 80_000 {
		return nil
	}

	related := collectResourceIDs(filterResources(resources, func(resource enrichedResource) bool {
		return resource.IsThirdPartyTool && resource.ThirdPartyKind == thirdPartyAnalytics
	}))

	return &AnalysisFinding{
		ID:         "third_party_analytics_overhead",
		Category:   "third_party",
		Severity:   "medium",
		Confidence: "high",
		Title:      "Recorta la sobrecarga de analítica",
		Summary:    fmt.Sprintf("La capa de analítica añade %s en %d peticiones. No suele ser el cuello de botella principal, pero sí añade ruido de red y variabilidad.", humanBytes(summary.AnalyticsBytes), summary.AnalyticsRequests),
		Evidence: []string{
			fmt.Sprintf("Bytes de analítica: %s.", humanBytes(summary.AnalyticsBytes)),
			fmt.Sprintf("Peticiones de analítica: %d.", summary.AnalyticsRequests),
		},
		EstimatedSavingsBytes: sumEstimatedSavingsForIDs(resources, related),
		RelatedResourceIDs:    related,
	}
}

func dominantRepeatedGalleryGroup(groups []ResourceGroup) *ResourceGroup {
	var candidate *ResourceGroup
	for index := range groups {
		group := &groups[index]
		if group.Kind != groupKindRepeatedGallery || group.TotalBytes < 400_000 {
			continue
		}
		if candidate == nil || group.TotalBytes > candidate.TotalBytes {
			candidate = group
		}
	}
	return candidate
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

func buildFontFinding(resources []enrichedResource, summary AnalysisSummary) *AnalysisFinding {
	if summary.FontBytes < 250_000 && summary.FontRequests < 4 {
		return nil
	}

	related := collectResourceIDs(filterResources(resources, func(resource enrichedResource) bool {
		return resource.Type == "font"
	}))

	return &AnalysisFinding{
		ID:         "font_stack_overweight",
		Category:   "fonts",
		Severity:   "medium",
		Confidence: "medium",
		Title:      "Recorta el coste tipográfico",
		Summary:    fmt.Sprintf("La pila de fuentes suma %s en %d archivos. Aunque el LCP sea bajo, este coste penaliza la carga inicial y la consistencia del primer render.", humanBytes(summary.FontBytes), summary.FontRequests),
		Evidence: []string{
			fmt.Sprintf("Bytes de fuentes: %s.", humanBytes(summary.FontBytes)),
			fmt.Sprintf("Peticiones de fuentes: %d.", summary.FontRequests),
		},
		EstimatedSavingsBytes: sumEstimatedSavingsForIDs(resources, related),
		RelatedResourceIDs:    related,
	}
}

func buildMainThreadFinding(resources []enrichedResource, perf PerformanceMetrics) *AnalysisFinding {
	if perf.LongTasksTotalMS < 250 {
		return nil
	}

	severity := "medium"
	if perf.LongTasksTotalMS >= 600 {
		severity = "high"
	}

	scripts := filterResources(resources, func(resource enrichedResource) bool {
		return resource.Type == "script"
	})

	return &AnalysisFinding{
		ID:         "main_thread_cpu_pressure",
		Category:   "cpu",
		Severity:   severity,
		Confidence: "high",
		Title:      "Reduce la presión real sobre la hebra principal",
		Summary:    fmt.Sprintf("Se observaron %d long tasks que suman %d ms. Aquí el problema ya no es solo peso de red: hay trabajo real de CPU que compite con el render.", perf.LongTasksCount, perf.LongTasksTotalMS),
		Evidence: []string{
			fmt.Sprintf("Long Tasks totales: %d ms.", perf.LongTasksTotalMS),
			fmt.Sprintf("Cantidad de Long Tasks: %d.", perf.LongTasksCount),
			fmt.Sprintf("Duración acumulada de recursos script: %d ms.", perf.ScriptResourceDurationMS),
		},
		EstimatedSavingsBytes: sumEstimatedSavings(scripts),
		RelatedResourceIDs:    collectTopResourceIDs(scripts, 3),
	}
}

func buildResponsiveImageFinding(resources []enrichedResource, groups []ResourceGroup, lcpResourceID string) *AnalysisFinding {
	repeatedGalleryMembers := collectRepeatedGalleryMembers(groups)
	var candidate *enrichedResource
	for index := range resources {
		resource := &resources[index]
		if resource.Type != "image" || resource.ID == lcpResourceID {
			continue
		}
		if _, ok := repeatedGalleryMembers[resource.ID]; ok {
			continue
		}
		ratio := naturalToRenderedRatio(*resource)
		overdelivery := responsiveOverdeliveryFactor(*resource)
		savings := estimateResourceSavings(*resource)
		if savings < 60_000 {
			continue
		}
		if overdelivery < 0.15 && !(ratio >= 1.5 && !resource.ResponsiveImage) {
			continue
		}
		if candidate == nil || savings > estimateResourceSavings(*candidate) {
			candidate = resource
		}
	}
	if candidate == nil {
		return nil
	}

	confidence := "medium"
	if candidate.NaturalWidth > 0 && candidate.NaturalHeight > 0 {
		confidence = "high"
	}

	title := "Corrige la sobreentrega de una imagen"
	summary := "Esta imagen se sirve bastante más grande de lo que exige su caja renderizada. Ajustar variantes, srcset/sizes o tamaño de salida puede recortar bytes sin sacrificar nitidez visible."
	if candidate.ResponsiveImage {
		summary = "Aunque ya se detectan variantes responsive, la imagen elegida sigue siendo bastante más grande de lo que exige su caja renderizada. Conviene ajustar sizes, breakpoints o tamaños de salida."
	} else {
		summary = "Esta imagen se sirve bastante más grande de lo que exige su caja renderizada y no se detectan variantes responsive claras. Ajustar variantes, srcset/sizes o tamaño de salida puede recortar bytes sin sacrificar nitidez visible."
	}

	return &AnalysisFinding{
		ID:         "responsive_image_overdelivery",
		Category:   "media",
		Severity:   "medium",
		Confidence: confidence,
		Title:      title,
		Summary:    summary,
		Evidence: []string{
			fmt.Sprintf("Transfiere %s.", humanBytes(candidate.Bytes)),
			fmt.Sprintf("Tamaño natural: %dx%d para una caja aproximada de %.0fx%.0f.", candidate.NaturalWidth, candidate.NaturalHeight, candidate.BoundingBox.Width, candidate.BoundingBox.Height),
			fmt.Sprintf("Ratio natural/rendered: %s.", humanRatio(naturalToRenderedRatio(*candidate))),
			fmt.Sprintf("Responsive imaging detectado: %t.", candidate.ResponsiveImage),
		},
		EstimatedSavingsBytes: estimateResourceSavings(*candidate),
		RelatedResourceIDs:    []string{candidate.ID},
	}
}

func buildHeavyAboveFoldFinding(resources []enrichedResource, perf PerformanceMetrics, lcpResourceID string) *AnalysisFinding {
	var candidate *enrichedResource
	for index := range resources {
		resource := &resources[index]
		if resource.ID == lcpResourceID {
			continue
		}
		if resource.Bytes < 150_000 {
			continue
		}
		if resource.VisualRole != visualRoleHeroMedia && resource.VisualRole != visualRoleAboveFoldMedia {
			continue
		}
		if candidate == nil || resource.Bytes > candidate.Bytes {
			candidate = resource
		}
	}
	if candidate == nil {
		return nil
	}

	severity := "medium"
	if perf.LCPMS >= 2_000 {
		severity = "high"
	}

	return &AnalysisFinding{
		ID:         "heavy_above_fold_media",
		Category:   "media",
		Severity:   severity,
		Confidence: "medium",
		Title:      "Reduce la media visible en el primer viewport",
		Summary:    fmt.Sprintf("Hay media above the fold de %s que compite con el primer render. No necesariamente domina el LCP, pero sí engorda el arranque visible.", humanBytes(candidate.Bytes)),
		Evidence: []string{
			fmt.Sprintf("Recurso above the fold: %s.", humanBytes(candidate.Bytes)),
			fmt.Sprintf("Rol visual detectado: %s.", strings.ReplaceAll(candidate.VisualRole, "_", " ")),
		},
		EstimatedSavingsBytes: estimateResourceSavings(*candidate),
		RelatedResourceIDs:    []string{candidate.ID},
	}
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
		leftPriority := vampirePriority(candidates[i])
		rightPriority := vampirePriority(candidates[j])
		if leftPriority == rightPriority {
			leftSavings := estimateResourceSavings(candidates[i])
			rightSavings := estimateResourceSavings(candidates[j])
			if leftSavings == rightSavings {
				if candidates[i].Bytes == candidates[j].Bytes {
					return candidates[i].ID < candidates[j].ID
				}
				return candidates[i].Bytes > candidates[j].Bytes
			}
			return leftSavings > rightSavings
		}
		return leftPriority > rightPriority
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

func countNonResponsiveImages(resources []enrichedResource) int {
	total := 0
	for _, resource := range resources {
		if resource.Type == "image" && !resource.ResponsiveImage {
			total++
		}
	}
	return total
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
