package scanner

import (
	"math"
	"net"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/publicsuffix"
)

const (
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

type Party string

const (
	PartyFirst Party = "first_party"
	PartyThird Party = "third_party"

	partyFirst = PartyFirst
	partyThird = PartyThird
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
func classifyParty(pageHostname, assetHostname string) Party {
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
	pageRoot := siteRoot(pageHostname)
	etld, _ := publicsuffix.PublicSuffix(pageRoot)
	pageBrand := strings.TrimSuffix(pageRoot, "."+etld)

	pageTokens := brandTokens(pageBrand)
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
	if host == "" {
		return nil
	}

	stopwords := map[string]struct{}{
		"www": {}, "cdn": {}, "static": {}, "assets": {}, "asset": {}, "img": {}, "images": {},
		"media": {}, "files": {}, "file": {}, "content": {}, "cloud": {}, "edge": {}, "object": {},
		"objects": {}, "objetos": {}, "estaticos": {}, "statico": {}, "staticos": {}, "xlk": {},
		"uecdn": {}, "www2": {}, "app": {}, "apps": {}, "github": {}, "gitlab": {}, "pages": {},
	}
	tokens := make(map[string]struct{})
	for _, token := range strings.FieldsFunc(host, func(r rune) bool {
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
	case containsAny(host, "doubleclick", "googlesyndication", "googleadservices", "gampad", "adnxs", "adsrvr", "taboola", "outbrain", "criteo", "pubmatic", "amazon-adsystem") || containsAny(resourceURLPath, "/ads.js", "/adunit", "/advertising/"):
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
