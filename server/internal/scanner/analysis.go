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

	nearThresholdLongTasksMS    int64 = 200
	mainThreadLongTasksMS       int64 = 250
	materialVampireBytes        int64 = 32_000
	materialVampireSavings      int64 = 16_000
	minGuaranteedVisualVamps          = 2
	dominantImageMinBytes       int64 = 1_000_000
	dominantImageHighBytes      int64 = 2_000_000
	dominantImageMinSavings     int64 = 250_000
	dominantImageMinShare             = 25.0
	dominantImageHighShare            = 40.0
	dominantGroupShareThreshold       = 0.70
	socialFindingMinBytes       int64 = 250_000
	socialFindingMinRequests          = 8
	adsFindingMinBytes          int64 = 300_000
	adsFindingMinRequests             = 10
	paymentFindingMinBytes      int64 = 150_000
	paymentFindingMinRequests         = 5
	videoFindingMinBytes        int64 = 250_000
	videoFindingMinRequests           = 4
	deferClusterSavingsFactor         = 0.75
	legacyImageFindingMinBytes  int64 = 300_000
	legacyFontFindingMinBytes   int64 = 80_000
	promotedAnchorMinSavings    int64 = 60_000
	lazyMajorityThreshold             = 0.70
)

func normalizeType(resourceType, mimeType, rawURL string) string {
	resourceType = strings.ToLower(resourceType)
	mimeType = strings.ToLower(mimeType)
	extension := strings.ToLower(path.Ext(resourcePath(rawURL)))

	switch {
	case strings.Contains(mimeType, "text/html"), strings.Contains(mimeType, "application/xhtml"), strings.Contains(resourceType, "document"):
		return "document"
	}

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
	if sharesFirstPartyBrand(pageHostname, assetHostname) && !isKnownVendorHost(assetHostname) {
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

func sharesFirstPartyBrand(pageHostname, assetHostname string) bool {
	pageTokens := brandTokens(pageHostname)
	if len(pageTokens) == 0 {
		return false
	}
	assetTokens := brandTokens(assetHostname)
	if len(assetTokens) == 0 {
		return false
	}

	for token := range pageTokens {
		if _, ok := assetTokens[token]; ok {
			return true
		}
	}
	return false
}

func brandTokens(host string) map[string]struct{} {
	root := siteRoot(host)
	if root == "" {
		return nil
	}

	stopwords := map[string]struct{}{
		"www": {}, "cdn": {}, "static": {}, "assets": {}, "asset": {}, "img": {}, "images": {},
		"media": {}, "files": {}, "file": {}, "content": {}, "cloud": {}, "edge": {}, "object": {},
		"objects": {}, "objetos": {}, "estaticos": {}, "statico": {}, "staticos": {}, "xlk": {},
		"uecdn": {}, "www2": {}, "app": {}, "apps": {}, "github": {}, "gitlab": {}, "pages": {},
	}
	tokens := make(map[string]struct{})
	for _, token := range strings.FieldsFunc(root, func(r rune) bool {
		switch {
		case r >= 'a' && r <= 'z':
			return false
		case r >= 'A' && r <= 'Z':
			return false
		default:
			return true
		}
	}) {
		token = strings.ToLower(strings.TrimSpace(token))
		if len(token) < 4 {
			continue
		}
		if _, blocked := stopwords[token]; blocked {
			continue
		}
		tokens[token] = struct{}{}
	}
	return tokens
}

func isKnownVendorHost(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	return containsAny(
		host,
		"google", "doubleclick", "googlesyndication", "googleadservices", "googletagmanager",
		"google-analytics", "youtube", "ytimg", "vimeo", "stripe", "paypal", "mercadopago",
		"tickettailor", "facebook", "instagram", "linkedin", "tiktok", "x.com", "twitter",
		"cloudflare", "akamai", "fastly", "newrelic", "datadog", "adobedtm", "omtrdc", "demdex",
		"taboola", "outbrain", "criteo", "pubmatic", "adsrvr", "adnxs", "amazon-adsystem",
	)
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

	host := strings.ToLower(strings.TrimSpace(resource.Hostname))
	fullURL := strings.ToLower(strings.TrimSpace(resource.URL))
	resourceURLPath := strings.ToLower(strings.TrimSpace(resourcePath(resource.URL)))
	switch {
	case containsAny(host, "posthog", "segment", "mixpanel", "plausible", "clarity", "amplitude", "ahrefs", "google-analytics", "googletagmanager", "omtrdc", "demdex", "adobedtm", "2o7", "tt.omtrdc"):
		return thirdPartyAnalytics
	case containsAny(fullURL, "gtm.js", "assets.adobedtm"):
		return thirdPartyAnalytics
	case containsAny(host, "doubleclick", "googlesyndication", "googleadservices", "gampad", "adnxs", "adsrvr", "taboola", "outbrain", "criteo", "pubmatic", "amazon-adsystem") || containsAny(resourceURLPath, "/ads/", "/adunit", "/advertising/"):
		return thirdPartyAds
	case containsAny(host, "intercom", "zendesk", "drift", "crisp", "helpscout"):
		return thirdPartySupport
	case containsAny(host, "facebook", "twitter", "x.com", "linkedin", "tiktok", "instagram"):
		return thirdPartySocial
	case containsAny(host, "youtube", "youtu.be", "youtube-nocookie", "ytimg", "vimeo", "loom"):
		return thirdPartyVideo
	case containsAny(host, "stripe", "paypal", "mercadopago", "tickettailor") || containsAny(fullURL, "checkout"):
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
			summary.AboveFoldVisualBytes += resource.Bytes
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

	findings := make([]AnalysisFinding, 0, 8)
	dominantImageFinding := buildDominantImageFinding(resources, groups)
	if finding := buildLCPFinding(resources, lcpResource, perf); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildMainThreadFinding(resources, perf); finding != nil {
		findings = append(findings, *finding)
	}
	if dominantImageFinding != nil {
		findings = append(findings, *dominantImageFinding)
	}
	if finding := buildRepeatedGalleryFinding(groups, resources, dominantImageFinding); finding != nil {
		findings = append(findings, *finding)
	}
	for _, finding := range buildThirdPartyFindings(resources, groups) {
		findings = append(findings, finding)
	}
	if finding := buildLegacyImageFormatFinding(resources); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildFontFinding(resources, summary); finding != nil {
		findings = append(findings, *finding)
	}
	if finding := buildLegacyFontFormatFinding(resources); finding != nil {
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
				if insights.FindingPriorityRank(findings[i].ID) == insights.FindingPriorityRank(findings[j].ID) {
					if findings[i].EstimatedSavingsBytes == findings[j].EstimatedSavingsBytes {
						return findings[i].ID < findings[j].ID
					}
					return findings[i].EstimatedSavingsBytes > findings[j].EstimatedSavingsBytes
				}
				return insights.FindingPriorityRank(findings[i].ID) > insights.FindingPriorityRank(findings[j].ID)
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

func buildDominantImageFinding(resources []enrichedResource, groups []ResourceGroup) *AnalysisFinding {
	totalBytes := sumBytes(resources)
	var candidate *enrichedResource
	for index := range resources {
		resource := &resources[index]
		if resource.Type != "image" {
			continue
		}
		share := shareOf(resource.Bytes, totalBytes)
		if resource.Bytes < dominantImageMinBytes && share < dominantImageMinShare {
			continue
		}
		if estimateResourceSavings(*resource) < dominantImageMinSavings {
			continue
		}
		if !hasMeasuredRenderedBox(*resource) {
			continue
		}
		ratio := naturalToRenderedRatio(*resource)
		if ratio <= 0 {
			continue
		}
		if ratio < 8 && resource.ResponsiveImage {
			continue
		}
		if candidate == nil || resource.Bytes > candidate.Bytes || (resource.Bytes == candidate.Bytes && estimateResourceSavings(*resource) > estimateResourceSavings(*candidate)) {
			candidate = resource
		}
	}
	if candidate == nil {
		return nil
	}

	severity := "medium"
	share := shareOf(candidate.Bytes, totalBytes)
	if candidate.Bytes >= dominantImageHighBytes || share >= dominantImageHighShare {
		severity = "high"
	}

	group := repeatedGalleryGroupForResource(groups, candidate.ID)
	members := []enrichedResource(nil)
	if group != nil {
		members = resourcesForIDs(resources, group.RelatedResourceIDs)
	}

	title := "Corrige una imagen dominante sobredimensionada"
	summary := fmt.Sprintf("Una sola imagen transfiere %s y se sirve mucho más grande de lo que exige su caja real. Aquí hay una oportunidad desproporcionada para bajar bytes sin tocar el resto del sitio.", humanBytes(candidate.Bytes))
	if group != nil && candidateDominatesGroup(*candidate, *group) {
		summary = fmt.Sprintf("Una sola imagen concentra %s de %s. Aunque forma parte de %s, su peso individual ya justifica atacarla primero.", humanBytes(candidate.Bytes), humanBytes(group.TotalBytes), strings.ToLower(withArticle(group.Label)))
	}

	evidence := []string{
		fmt.Sprintf("Transfiere %s.", humanBytes(candidate.Bytes)),
		fmt.Sprintf("Representa %.1f%% de todos los bytes transferidos.", share),
		fmt.Sprintf("Tamaño natural: %dx%d para una caja aproximada de %.0fx%.0f.", candidate.NaturalWidth, candidate.NaturalHeight, boundingWidth(candidate.BoundingBox), boundingHeight(candidate.BoundingBox)),
	}
	if candidate.PositionBand != "" && candidate.PositionBand != positionUnknown {
		evidence = append(evidence, fmt.Sprintf("Posición visual: %s.", strings.ReplaceAll(candidate.PositionBand, "_", " ")))
	}
	if group != nil && groupHasMixedFormats(members) {
		evidence = append(evidence, "Dentro del mismo bloque conviven formatos legacy y modernos.")
	}
	if group != nil && legacyAssetOutweighsModernSibling(*candidate, members) {
		evidence = append(evidence, "Dentro del mismo bloque ya existen variantes modernas mucho más ligeras.")
	}

	return &AnalysisFinding{
		ID:                    "dominant_image_overdelivery",
		Category:              "media",
		Severity:              severity,
		Confidence:            "high",
		Title:                 title,
		Summary:               summary,
		Evidence:              evidence,
		EstimatedSavingsBytes: estimateResourceSavings(*candidate),
		RelatedResourceIDs:    []string{candidate.ID},
	}
}

func buildLCPFinding(resources []enrichedResource, resource *enrichedResource, perf PerformanceMetrics) *AnalysisFinding {
	if resource != nil {
		severity := "medium"
		if perf.RenderMetricsComplete && perf.LCPMS >= 2_000 {
			severity = "high"
		}
		if perf.RenderMetricsComplete && resource.Bytes < 150_000 && perf.LCPMS > 0 && perf.LCPMS < 1_500 {
			severity = "low"
		}

		evidence := []string{
			fmt.Sprintf("El recurso que coincide con el LCP pesa %s.", humanBytes(resource.Bytes)),
		}
		if perf.RenderMetricsComplete {
			evidence = append(evidence, fmt.Sprintf("LCP observado: %d ms.", perf.LCPMS))
		} else {
			evidence = append(evidence, "El recurso LCP quedó mapeado, pero las métricas de render no se capturaron completas en este scan.")
		}
		if resource.PositionBand != "" && resource.PositionBand != positionUnknown {
			evidence = append(evidence, fmt.Sprintf("Su posición es %s.", strings.ReplaceAll(resource.PositionBand, "_", " ")))
		}
		if perf.LCPResourceTag != "" {
			evidence = append(evidence, fmt.Sprintf("El nodo LCP es <%s>.", perf.LCPResourceTag))
		}

		summary := fmt.Sprintf("La carga crítica está anclada a un recurso real de %s. Es el mejor punto de ataque para recortar el render inicial sin tocar el resto del documento.", humanBytes(resource.Bytes))
		if resource.Bytes < 150_000 {
			summary = fmt.Sprintf("El LCP sí apunta a un recurso real de %s, pero no parece gigantesco por peso bruto. Aquí conviene revisar prioridad, dimensionado correcto y el contexto de CPU/CSS que lo rodea, no solo compresión.", humanBytes(resource.Bytes))
		}

		return &AnalysisFinding{
			ID:                    "render_lcp_candidate",
			Category:              "render",
			Severity:              severity,
			Confidence:            "high",
			Title:                 "Ataca el recurso que domina el LCP",
			Summary:               summary,
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
	}
	if perf.RenderMetricsComplete {
		evidence = append(evidence, fmt.Sprintf("LCP observado: %d ms.", perf.LCPMS))
	} else {
		evidence = append(evidence, "Las métricas de render no se capturaron completas en este scan.")
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

func buildRepeatedGalleryFinding(groups []ResourceGroup, resources []enrichedResource, dominantImageFinding *AnalysisFinding) *AnalysisFinding {
	candidate := dominantRepeatedGalleryGroup(groups)
	if candidate == nil {
		return nil
	}

	members := resourcesForIDs(resources, candidate.RelatedResourceIDs)
	dominantResourceID := ""
	if dominantImageFinding != nil && len(dominantImageFinding.RelatedResourceIDs) > 0 {
		dominantResourceID = dominantImageFinding.RelatedResourceIDs[0]
	}
	if dominantResourceID != "" {
		if dominant := resourceForID(resources, dominantResourceID); dominant != nil && candidateContainsResource(*candidate, dominantResourceID) && candidateDominatesGroup(*dominant, *candidate) {
			if candidate.TotalBytes-dominant.Bytes < 400_000 {
				return nil
			}
		}
	}
	missingResponsive := countNonResponsiveImages(members)
	medianRatio := medianNaturalToRenderedRatio(members)
	mixedFormats := groupHasMixedFormats(members)
	lazyCount := countLazyLoadedImages(members)
	lazyMajority := len(members) > 0 && float64(lazyCount)/float64(len(members)) >= lazyMajorityThreshold

	title := fmt.Sprintf("Reduce el peso de %s", withArticle(candidate.Label))
	summary := fmt.Sprintf("%s suma %s. No todo este bloque es crítico para el viewport inicial, pero sí infla el coste por visita y multiplica el peso del bloque visual.", candidate.Label, humanBytes(candidate.TotalBytes))

	switch candidate.PositionBand {
	case positionBelowFold:
		title = fmt.Sprintf("Reduce el peso de %s bajo el fold", withArticle(candidate.Label))
		summary = fmt.Sprintf("%s suma %s. No frena el primer render, pero sí infla el coste por visita y multiplica el peso del bloque visual.", candidate.Label, humanBytes(candidate.TotalBytes))
	case positionNearFold:
		title = fmt.Sprintf("Reduce el peso de %s cerca del fold", withArticle(candidate.Label))
		summary = fmt.Sprintf("%s suma %s. Parte de ese bloque entra pronto en pantalla, pero su volumen sigue inflando el coste por visita más allá del primer viewport.", candidate.Label, humanBytes(candidate.TotalBytes))
	}
	if lazyMajority {
		summary = fmt.Sprintf("%s suma %s. La mayoría de sus imágenes ya usan lazy loading, así que el margen principal está en sizes, tamaño de salida y formatos más eficientes.", candidate.Label, humanBytes(candidate.TotalBytes))
	}

	evidence := []string{
		fmt.Sprintf("Grupo detectado: %s.", candidate.Label),
		fmt.Sprintf("Transferencia agregada: %s en %d recursos.", humanBytes(candidate.TotalBytes), candidate.ResourceCount),
		fmt.Sprintf("Posición dominante: %s.", strings.ReplaceAll(candidate.PositionBand, "_", " ")),
		fmt.Sprintf("Imágenes sin srcset/sizes: %d de %d.", missingResponsive, len(members)),
		fmt.Sprintf("Mediana natural/rendered: %s.", humanRatio(medianRatio)),
	}
	if lazyCount > 0 {
		evidence = append(evidence, fmt.Sprintf("Imágenes con loading=\"lazy\": %d de %d.", lazyCount, len(members)))
	}
	if lazyMajority {
		evidence = append(evidence, fmt.Sprintf("Lazy loading ya presente en la mayoría: %d de %d.", lazyCount, len(members)))
	}
	if mixedFormats {
		evidence = append(evidence, "El grupo mezcla formatos legacy y modernos.")
	}
	if dominantResourceID != "" {
		if dominant := resourceForID(resources, dominantResourceID); dominant != nil && candidateContainsResource(*candidate, dominantResourceID) && legacyAssetOutweighsModernSibling(*dominant, members) {
			evidence = append(evidence, "Dentro del grupo, un JPEG/PNG pesa mucho más que un sibling moderno comparable.")
		}
	}

	return &AnalysisFinding{
		ID:                    "repeated_gallery_overdelivery",
		Category:              "media",
		Severity:              "medium",
		Confidence:            "high",
		Title:                 title,
		Summary:               summary,
		Evidence:              evidence,
		EstimatedSavingsBytes: sumEstimatedSavingsForIDs(resources, candidate.RelatedResourceIDs),
		RelatedResourceIDs:    append([]string(nil), candidate.RelatedResourceIDs...),
	}
}

func buildThirdPartyFindings(resources []enrichedResource, groups []ResourceGroup) []AnalysisFinding {
	findings := make([]AnalysisFinding, 0, 5)
	for _, group := range groups {
		if group.Kind != groupKindThirdParty {
			continue
		}
		members := resourcesForIDs(resources, group.RelatedResourceIDs)
		switch dominantThirdPartyKind(members) {
		case thirdPartyAnalytics:
			if group.ResourceCount < 2 && group.TotalBytes < 80_000 {
				continue
			}
			findings = append(findings, AnalysisFinding{
				ID:         "third_party_analytics_overhead",
				Category:   "third_party",
				Severity:   "medium",
				Confidence: "high",
				Title:      "Recorta la sobrecarga de analítica",
				Summary:    fmt.Sprintf("La capa de analítica añade %s en %d peticiones. No suele ser el cuello de botella principal, pero sí añade ruido de red y variabilidad.", humanBytes(group.TotalBytes), group.ResourceCount),
				Evidence: []string{
					fmt.Sprintf("Bytes de analítica: %s.", humanBytes(group.TotalBytes)),
					fmt.Sprintf("Peticiones de analítica: %d.", group.ResourceCount),
				},
				EstimatedSavingsBytes: sumEstimatedSavingsForIDs(resources, group.RelatedResourceIDs),
				RelatedResourceIDs:    append([]string(nil), group.RelatedResourceIDs...),
			})
		case thirdPartySocial:
			if group.TotalBytes < socialFindingMinBytes && group.ResourceCount < socialFindingMinRequests {
				continue
			}
			findings = append(findings, AnalysisFinding{
				ID:         "third_party_social_overhead",
				Category:   "third_party",
				Severity:   "medium",
				Confidence: "high",
				Title:      "Retrasa embeds y widgets sociales",
				Summary:    fmt.Sprintf("Los recursos sociales añaden %s en %d peticiones. No suelen ayudar al contenido inicial, pero sí meten ruido de red, scripts externos y variabilidad.", humanBytes(group.TotalBytes), group.ResourceCount),
				Evidence: []string{
					fmt.Sprintf("Bytes del cluster social: %s.", humanBytes(group.TotalBytes)),
					fmt.Sprintf("Peticiones del cluster social: %d.", group.ResourceCount),
				},
				EstimatedSavingsBytes: sumEstimatedSavingsForIDs(resources, group.RelatedResourceIDs),
				RelatedResourceIDs:    append([]string(nil), group.RelatedResourceIDs...),
			})
		case thirdPartyAds:
			if group.TotalBytes < adsFindingMinBytes && group.ResourceCount < adsFindingMinRequests {
				continue
			}
			findings = append(findings, AnalysisFinding{
				ID:         "third_party_ads_overhead",
				Category:   "third_party",
				Severity:   "medium",
				Confidence: "high",
				Title:      "Recorta la sobrecarga del stack publicitario",
				Summary:    fmt.Sprintf("El stack publicitario añade %s en %d peticiones. Entre scripts, auction calls, iframes y creatividades, mete variabilidad y coste antes de aportar valor editorial.", humanBytes(group.TotalBytes), group.ResourceCount),
				Evidence: []string{
					fmt.Sprintf("Bytes del cluster de anuncios: %s.", humanBytes(group.TotalBytes)),
					fmt.Sprintf("Peticiones del cluster de anuncios: %d.", group.ResourceCount),
				},
				EstimatedSavingsBytes: estimateDeferredClusterSavings(members),
				RelatedResourceIDs:    append([]string(nil), group.RelatedResourceIDs...),
			})
		case thirdPartyPayment:
			if group.TotalBytes < paymentFindingMinBytes && group.ResourceCount < paymentFindingMinRequests {
				continue
			}
			findings = append(findings, AnalysisFinding{
				ID:         "third_party_payment_overhead",
				Category:   "third_party",
				Severity:   "medium",
				Confidence: "high",
				Title:      "Difiere ticketing y widgets de pago",
				Summary:    fmt.Sprintf("Los widgets de ticketing o pago añaden %s en %d peticiones. No suelen aportar al primer render, pero sí meten terceros, iframes y trabajo extra durante la carga.", humanBytes(group.TotalBytes), group.ResourceCount),
				Evidence: []string{
					fmt.Sprintf("Bytes del cluster de pagos: %s.", humanBytes(group.TotalBytes)),
					fmt.Sprintf("Peticiones del cluster de pagos: %d.", group.ResourceCount),
				},
				EstimatedSavingsBytes: estimateDeferredClusterSavings(members),
				RelatedResourceIDs:    append([]string(nil), group.RelatedResourceIDs...),
			})
		case thirdPartyVideo:
			if group.TotalBytes < videoFindingMinBytes && group.ResourceCount < videoFindingMinRequests {
				continue
			}
			findings = append(findings, AnalysisFinding{
				ID:         "third_party_video_overhead",
				Category:   "third_party",
				Severity:   "medium",
				Confidence: "high",
				Title:      "Difiere embeds y players de video",
				Summary:    fmt.Sprintf("Los embeds de video añaden %s en %d peticiones. Suelen aportar mucho peso en iframes, thumbnails o scripts externos antes de que el usuario interactúe.", humanBytes(group.TotalBytes), group.ResourceCount),
				Evidence: []string{
					fmt.Sprintf("Bytes del cluster de video: %s.", humanBytes(group.TotalBytes)),
					fmt.Sprintf("Peticiones del cluster de video: %d.", group.ResourceCount),
				},
				EstimatedSavingsBytes: estimateDeferredClusterSavings(members),
				RelatedResourceIDs:    append([]string(nil), group.RelatedResourceIDs...),
			})
		}
	}
	return findings
}

func buildLegacyImageFormatFinding(resources []enrichedResource) *AnalysisFinding {
	images := filterResources(resources, func(resource enrichedResource) bool {
		return resource.Type == "image" && resource.Bytes > 0
	})
	if len(images) < 5 {
		return nil
	}

	legacyImages := filterResources(images, func(resource enrichedResource) bool {
		return isLegacyImageFormat(resource.MIMEType)
	})
	if len(legacyImages) == 0 {
		return nil
	}

	totalImageBytes := sumBytes(images)
	legacyBytes := sumBytes(legacyImages)
	modernBytes := totalImageBytes - legacyBytes
	if legacyBytes < legacyImageFindingMinBytes {
		return nil
	}
	if float64(legacyBytes) < float64(totalImageBytes)*0.60 {
		return nil
	}
	if modernBytes > 0 && float64(modernBytes) > float64(totalImageBytes)*0.25 {
		return nil
	}

	return &AnalysisFinding{
		ID:         "legacy_image_format_overhead",
		Category:   "media",
		Severity:   "medium",
		Confidence: "high",
		Title:      "Migra la mayor parte de las imágenes a formatos modernos",
		Summary:    fmt.Sprintf("Una parte material del peso de imagen sigue en JPEG/PNG: %s sobre %s. Aquí hay margen claro para recortar transferencia con AVIF/WebP y variantes más disciplinadas.", humanBytes(legacyBytes), humanBytes(totalImageBytes)),
		Evidence: []string{
			fmt.Sprintf("Bytes en JPEG/PNG: %s.", humanBytes(legacyBytes)),
			fmt.Sprintf("Bytes de imagen totales: %s.", humanBytes(totalImageBytes)),
			fmt.Sprintf("Recursos legacy detectados: %d de %d imágenes.", len(legacyImages), len(images)),
		},
		EstimatedSavingsBytes: sumEstimatedSavings(legacyImages),
		RelatedResourceIDs:    collectTopResourceIDs(legacyImages, 5),
	}
}

func buildLegacyFontFormatFinding(resources []enrichedResource) *AnalysisFinding {
	modernSignatures := collectModernFontSignatures(resources)
	legacyFonts := filterResources(resources, func(resource enrichedResource) bool {
		return resource.Type == "font" && isLegacyFontFormat(resource) && !hasModernFontEquivalent(resource, modernSignatures)
	})
	if len(legacyFonts) == 0 {
		return nil
	}
	legacyBytes := sumBytes(legacyFonts)
	if legacyBytes < legacyFontFindingMinBytes {
		return nil
	}

	return &AnalysisFinding{
		ID:         "legacy_font_format_overhead",
		Category:   "fonts",
		Severity:   "medium",
		Confidence: "high",
		Title:      "Sirve las fuentes pesadas en WOFF2",
		Summary:    fmt.Sprintf("Todavía se sirven fuentes legacy por %s en formatos como WOFF/TTF/OTF. Aquí el recorte suele ser limpio: menos bytes sin tocar el look & feel.", humanBytes(legacyBytes)),
		Evidence: []string{
			fmt.Sprintf("Bytes en formatos legacy de fuente: %s.", humanBytes(legacyBytes)),
			fmt.Sprintf("Archivos legacy detectados: %d.", len(legacyFonts)),
		},
		EstimatedSavingsBytes: estimateLegacyFontSavings(legacyBytes),
		RelatedResourceIDs:    collectTopResourceIDs(legacyFonts, 5),
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

func repeatedGalleryGroupForResource(groups []ResourceGroup, resourceID string) *ResourceGroup {
	for index := range groups {
		group := &groups[index]
		if group.Kind != groupKindRepeatedGallery {
			continue
		}
		if candidateContainsResource(*group, resourceID) {
			return group
		}
	}
	return nil
}

func candidateContainsResource(group ResourceGroup, resourceID string) bool {
	for _, id := range group.RelatedResourceIDs {
		if id == resourceID {
			return true
		}
	}
	return false
}

func candidateDominatesGroup(resource enrichedResource, group ResourceGroup) bool {
	if group.TotalBytes <= 0 {
		return false
	}
	return float64(resource.Bytes)/float64(group.TotalBytes) >= dominantGroupShareThreshold
}

func hasMeasuredRenderedBox(resource enrichedResource) bool {
	return resource.BoundingBox != nil && resource.BoundingBox.Width > 0 && resource.BoundingBox.Height > 0
}

func dominantThirdPartyKind(resources []enrichedResource) string {
	bestKind := thirdPartyUnknown
	bestBytes := int64(0)
	for _, resource := range resources {
		if resource.Bytes > bestBytes {
			bestBytes = resource.Bytes
			bestKind = resource.ThirdPartyKind
		}
	}
	return bestKind
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

func buildFontFinding(resources []enrichedResource, summary AnalysisSummary) *AnalysisFinding {
	if summary.FontBytes < 250_000 && summary.FontRequests < 4 {
		return nil
	}

	fontResources := filterResources(resources, func(resource enrichedResource) bool {
		return resource.Type == "font"
	})
	related := collectResourceIDs(fontResources)

	title := "Recorta el coste tipográfico"
	summaryText := fmt.Sprintf("La pila de fuentes suma %s en %d archivos. Aunque el LCP sea bajo, este coste penaliza la carga inicial y la consistencia del primer render.", humanBytes(summary.FontBytes), summary.FontRequests)
	evidence := []string{
		fmt.Sprintf("Bytes de fuentes: %s.", humanBytes(summary.FontBytes)),
		fmt.Sprintf("Peticiones de fuentes: %d.", summary.FontRequests),
	}
	if dominantIconFont(fontResources) != nil {
		title = "Recorta el coste de la fuente de iconos"
		summaryText = fmt.Sprintf("La pila tipográfica suma %s en %d archivos y una parte material viene de una icon font genérica. Aquí suele rendir más pasar a SVGs individuales o subsetting agresivo que tratarla como texto común.", humanBytes(summary.FontBytes), summary.FontRequests)
		evidence = append(evidence, "Se detectó una fuente de iconos pesada entre los recursos tipográficos.")
	}

	return &AnalysisFinding{
		ID:                    "font_stack_overweight",
		Category:              "fonts",
		Severity:              "medium",
		Confidence:            "medium",
		Title:                 title,
		Summary:               summaryText,
		Evidence:              evidence,
		EstimatedSavingsBytes: sumEstimatedSavingsForIDs(resources, related),
		RelatedResourceIDs:    related,
	}
}

func buildMainThreadFinding(resources []enrichedResource, perf PerformanceMetrics) *AnalysisFinding {
	if perf.LongTasksTotalMS < nearThresholdLongTasksMS {
		return nil
	}

	confidence := "high"
	severity := "medium"
	summary := fmt.Sprintf("Se observaron %d long tasks que suman %d ms. Aquí el problema ya no es solo peso de red: hay trabajo real de CPU que compite con el render.", perf.LongTasksCount, perf.LongTasksTotalMS)
	if perf.LongTasksTotalMS < mainThreadLongTasksMS {
		confidence = "low"
		severity = "low"
		summary = fmt.Sprintf("Se observaron %d long tasks que suman %d ms. Está cerca del umbral donde la CPU empieza a competir con el render inicial, así que conviene vigilarlo porque puede oscilar entre scans.", perf.LongTasksCount, perf.LongTasksTotalMS)
	}
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
		Confidence: confidence,
		Title:      "Reduce la presión real sobre la hebra principal",
		Summary:    summary,
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

	evidence := []string{
		fmt.Sprintf("Transfiere %s.", humanBytes(candidate.Bytes)),
		fmt.Sprintf("Tamaño natural: %dx%d para una caja aproximada de %.0fx%.0f.", candidate.NaturalWidth, candidate.NaturalHeight, candidate.BoundingBox.Width, candidate.BoundingBox.Height),
		fmt.Sprintf("Ratio natural/rendered: %s.", humanRatio(naturalToRenderedRatio(*candidate))),
		fmt.Sprintf("Responsive imaging detectado: %t.", candidate.ResponsiveImage),
	}
	if candidate.PositionBand != "" && candidate.PositionBand != positionUnknown {
		evidence = append(evidence, fmt.Sprintf("Posición visual: %s.", strings.ReplaceAll(candidate.PositionBand, "_", " ")))
	}

	return &AnalysisFinding{
		ID:                    "responsive_image_overdelivery",
		Category:              "media",
		Severity:              "medium",
		Confidence:            confidence,
		Title:                 title,
		Summary:               summary,
		Evidence:              evidence,
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
	return strings.EqualFold(resource.DOMTag, "img") || resource.Type == "image"
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
