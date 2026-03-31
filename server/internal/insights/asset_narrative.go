package insights

import (
	"fmt"
	"strings"
)

func BuildRuleBasedAssetInsight(
	asset ResourceContext,
	findings []AnalysisFindingContext,
	actions []TopAction,
) AssetInsightDraft {
	finding := matchAssetFinding(asset, findings)
	action := matchAssetAction(asset, actions, finding)

	title := assetTitle(asset, finding)
	shortProblem := assetShortProblem(asset, finding)
	whyItMatters := assetWhyItMatters(asset, finding)
	recommendedAction := strings.TrimSpace(SuggestRuleBasedResource(asset))
	if action != nil && strings.TrimSpace(action.Reason) != "" {
		recommendedAction = strings.TrimSpace(action.Reason)
	}

	confidence := "low"
	if finding != nil && finding.Confidence != "" {
		confidence = finding.Confidence
	} else if action != nil && action.Confidence != "" {
		confidence = action.Confidence
	}

	impact := inferAssetLCPImpact(asset, finding, action)
	evidence := assetEvidence(asset, finding, action)

	scope := "asset"
	if finding != nil && len(finding.RelatedResourceIDs) > 1 {
		scope = "group"
	}

	draft := AssetInsightDraft{
		ResourceID:        asset.ID,
		Title:             title,
		ShortProblem:      shortProblem,
		WhyItMatters:      whyItMatters,
		RecommendedAction: recommendedAction,
		Confidence:        confidence,
		LikelyLCPImpact:   impact,
		Evidence:          evidence,
		Scope:             scope,
		Source:            "rule_based",
	}
	if finding != nil {
		draft.RelatedFindingID = finding.ID
	}
	if action != nil {
		draft.RelatedActionID = action.ID
		draft.RecommendedFix = action.RecommendedFix
	}
	return draft
}
func trimEvidence(evidence []string, limit int) []string {
	if len(evidence) <= limit {
		return append([]string(nil), evidence...)
	}
	return append([]string(nil), evidence[:limit]...)
}
func matchAssetFinding(asset ResourceContext, findings []AnalysisFindingContext) *AnalysisFindingContext {
	var best *AnalysisFindingContext
	for index := range findings {
		finding := &findings[index]
		if containsString(finding.RelatedResourceIDs, asset.ID) {
			if best == nil || findingPreferredForAsset(asset, *finding, *best) {
				best = finding
			}
		}
	}
	if best != nil {
		return best
	}

	candidates := make([]*AnalysisFindingContext, 0, len(findings))
	for index := range findings {
		finding := &findings[index]
		switch {
		case asset.VisualRole == "lcp_candidate" && finding.ID == "render_lcp_candidate":
			candidates = append(candidates, finding)
		case asset.Type == "font" && finding.ID == "font_stack_overweight":
			candidates = append(candidates, finding)
		case asset.Type == "font" && finding.ID == "legacy_font_format_overhead":
			candidates = append(candidates, finding)
		case asset.IsThirdPartyTool && asset.ThirdPartyKind == "analytics" && finding.ID == "third_party_analytics_overhead":
			candidates = append(candidates, finding)
		case asset.IsThirdPartyTool && asset.ThirdPartyKind == "ads" && finding.ID == "third_party_ads_overhead":
			candidates = append(candidates, finding)
		case asset.IsThirdPartyTool && asset.ThirdPartyKind == "social" && finding.ID == "third_party_social_overhead":
			candidates = append(candidates, finding)
		case asset.IsThirdPartyTool && asset.ThirdPartyKind == "payment" && finding.ID == "third_party_payment_overhead":
			candidates = append(candidates, finding)
		case asset.IsThirdPartyTool && asset.ThirdPartyKind == "video_embed" && finding.ID == "third_party_video_overhead":
			candidates = append(candidates, finding)
		}
	}
	for _, finding := range candidates {
		if best == nil || findingBetter(*finding, *best) {
			best = finding
		}
	}
	return best
}
func matchAssetAction(asset ResourceContext, actions []TopAction, finding *AnalysisFindingContext) *TopAction {
	if finding != nil {
		for index := range actions {
			if containsString(actions[index].RelatedResourceIDs, asset.ID) && actions[index].RelatedFindingID == finding.ID {
				return &actions[index]
			}
		}
	}
	for index := range actions {
		if containsString(actions[index].RelatedResourceIDs, asset.ID) {
			return &actions[index]
		}
	}
	_ = finding
	return nil
}
func findingPreferredForAsset(asset ResourceContext, left, right AnalysisFindingContext) bool {
	leftScore := assetFindingPreferenceScore(asset, left)
	rightScore := assetFindingPreferenceScore(asset, right)
	if leftScore == rightScore {
		return findingBetter(left, right)
	}
	return leftScore > rightScore
}
func assetFindingPreferenceScore(asset ResourceContext, finding AnalysisFindingContext) int {
	switch {
	case finding.ID == "dominant_image_overdelivery":
		return 4
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "ads" && finding.ID == "third_party_ads_overhead":
		return 3
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "analytics" && finding.ID == "third_party_analytics_overhead":
		return 3
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "social" && finding.ID == "third_party_social_overhead":
		return 3
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "payment" && finding.ID == "third_party_payment_overhead":
		return 3
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "video_embed" && finding.ID == "third_party_video_overhead":
		return 3
	case asset.Type == "font" && finding.ID == "font_stack_overweight":
		return 3
	case asset.Type == "font" && finding.ID == "legacy_font_format_overhead":
		return 3
	case asset.Type == "image" && finding.ID == "legacy_image_format_overhead":
		return 2
	case asset.VisualRole == "repeated_card_media" && finding.ID == "repeated_gallery_overdelivery":
		return 3
	default:
		return 0
	}
}
func findingBetter(left, right AnalysisFindingContext) bool {
	if SeverityRank(left.Severity) == SeverityRank(right.Severity) {
		if ConfidenceRank(left.Confidence) == ConfidenceRank(right.Confidence) {
			if left.EstimatedSavingsBytes == right.EstimatedSavingsBytes {
				return left.ID < right.ID
			}
			return left.EstimatedSavingsBytes > right.EstimatedSavingsBytes
		}
		return ConfidenceRank(left.Confidence) > ConfidenceRank(right.Confidence)
	}
	return SeverityRank(left.Severity) > SeverityRank(right.Severity)
}
func assetTitle(asset ResourceContext, finding *AnalysisFindingContext) string {
	if finding != nil && strings.TrimSpace(finding.Title) != "" {
		return finding.Title
	}
	switch {
	case isIconFontResourceContext(asset):
		return "Fuente de iconos sobredimensionada"
	case asset.Type == "font":
		return "Coste tipográfico concentrado"
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "analytics":
		return "Sobrecarga de analítica"
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "social":
		return "Sobrecarga de embeds sociales"
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "payment":
		return "Sobrecarga de ticketing o pago"
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "video_embed":
		return "Sobrecarga de embeds de video"
	case asset.VisualRole == "lcp_candidate":
		return "Candidato real al LCP"
	case asset.VisualRole == "repeated_card_media":
		return "Media repetida del grid"
	case isLogoLikeRaster(asset):
		return "Logo raster sobredimensionado"
	case materiallyOversizedImage(asset):
		return "Imagen sobredimensionada"
	case lightlyOversizedImage(asset):
		return "Imagen sobredimensionada, pero de bajo impacto"
	case asset.Type == "script":
		return "Script con presión innecesaria"
	default:
		return "Activo dominante a revisar"
	}
}
func assetShortProblem(asset ResourceContext, finding *AnalysisFindingContext) string {
	if finding != nil && strings.TrimSpace(finding.Summary) != "" {
		switch finding.ID {
		case "repeated_gallery_overdelivery":
			switch asset.PositionBand {
			case "below_fold":
				return "Este asset forma parte de una galería repetida bajo el fold que acumula más peso del necesario."
			case "near_fold":
				return "Este asset forma parte de una galería repetida cerca del fold que suma transferencia poco rentable."
			default:
				return "Este asset forma parte de una galería repetida que dispara el coste visual acumulado."
			}
		case "render_lcp_candidate":
			return "Este recurso coincide con el LCP observado y concentra uno de los mejores puntos de ataque."
		case "heavy_above_fold_media":
			return "Este media vive en la zona visible inicial y pesa más de lo razonable para su rol."
		case "third_party_analytics_overhead":
			return "Esta dependencia de analítica añade coste de red y variabilidad sin aportar al render inicial."
		case "third_party_social_overhead":
			return "Estos embeds o widgets sociales añaden peso y variabilidad sin ayudar al contenido inicial."
		case "third_party_payment_overhead":
			return "Este widget de ticketing o pago añade terceros e iframes antes de que el usuario los necesite."
		case "third_party_video_overhead":
			return "Este embed de video añade player, thumbnail o scripts externos antes de la reproducción."
		case "dominant_image_overdelivery":
			return "Esta imagen por sí sola ya pesa mucho más de lo razonable para su caja real."
		case "font_stack_overweight":
			if isIconFontResourceContext(asset) {
				return "Esta fuente de iconos arrastra muchos glifos que la interfaz probablemente no usa."
			}
			return "Este archivo forma parte de una pila tipográfica más pesada de lo necesario."
		case "main_thread_cpu_pressure":
			return "Este script participa en un arranque con presión real de CPU."
		case "responsive_image_overdelivery":
			return "Esta imagen se sirve más grande de lo que exige su caja visible."
		case "render_lcp_dom_node":
			return "El LCP observado es textual o del DOM; este asset se relaciona más con soporte que con media crítica."
		}
	}
	switch {
	case asset.Failed:
		return "La petición falló y sigue contando como deuda técnica del flujo auditado."
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "analytics":
		return "La analítica suma carga de red sin ayudar al contenido visible."
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "social":
		return "El embed social añade peso y JS externo sin mejorar el contenido inicial."
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "payment":
		return "El widget de ticketing o pago mete terceros y iframes antes de que el usuario empiece el checkout."
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "video_embed":
		return "El player externo mete thumbnails, iframes o scripts antes de que el usuario reproduzca el video."
	case isIconFontResourceContext(asset):
		return "Las icon fonts genéricas suelen traer miles de glifos no usados y penalizan la carga igual que una fuente pesada."
	case isLogoLikeRaster(asset):
		return "El logo se sirve como raster con mucha más resolución de la que necesita su caja visible."
	case asset.VisualRole == "repeated_card_media":
		return "Este asset se repite en un listado visual y escala mal cuando crece el catálogo."
	case asset.VisualRole == "lcp_candidate":
		return "Este asset domina el render crítico observado."
	case asset.Type == "font":
		return "El coste tipográfico de este archivo es alto para una primera visita."
	case lightlyOversizedImage(asset):
		return "Se sirve con más resolución de la necesaria, aunque su impacto total es limitado."
	default:
		return "Este asset concentra demasiado peso para el valor que aporta."
	}
}
func assetWhyItMatters(asset ResourceContext, finding *AnalysisFindingContext) string {
	if finding != nil {
		switch finding.Category {
		case "render":
			return "Afecta directamente al tiempo hasta que el usuario percibe el contenido principal o a la estabilidad del arranque."
		case "media":
			return "No siempre frena el primer render, pero sí eleva transferencia, CO2 y coste acumulado por visita."
		case "third_party":
			return "Introduce variabilidad externa y compite con recursos propios durante la carga."
		case "fonts":
			return "La tipografía pesa en la carga inicial aunque el sitio no se vea lento a simple vista."
		case "cpu":
			return "El coste no es solo de bytes: también se traduce en más trabajo y más incertidumbre en producción."
		}
	}
	switch {
	case asset.Type == "font":
		return "Las fuentes impactan el primer render incluso cuando su peso parece pequeño frente a las imágenes."
	case asset.Type == "script":
		return "Los scripts pueden competir con el render y elevar Long Tasks aunque no sean el archivo más pesado."
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "social":
		return "Los widgets sociales introducen terceros extra, más red y más variabilidad de la necesaria."
	case isLogoLikeRaster(asset):
		return "No suele dominar el sitio por sí solo, pero es una optimización limpia y repetible para una pieza visible de branding."
	case asset.VisualRole == "repeated_card_media":
		return "Multiplicado por todo el grid, este patrón sube rápido el coste total por visita."
	case lightlyOversizedImage(asset):
		return "No cambia por sí sola la narrativa del informe, pero evita sobreentrega innecesaria y mantiene el sitio más disciplinado."
	default:
		return "Reducir este recurso mejora la eficiencia sin tocar partes del sitio menos relevantes."
	}
}
func assetEvidence(asset ResourceContext, finding *AnalysisFindingContext, action *TopAction) []string {
	evidence := make([]string, 0, 3)
	if asset.Bytes > 0 {
		evidence = append(evidence, fmt.Sprintf("Transfiere %s.", formatBytes(asset.Bytes)))
	}
	if asset.PositionBand != "" && asset.PositionBand != "unknown" {
		evidence = append(evidence, fmt.Sprintf("Se ubica en %s.", strings.ReplaceAll(asset.PositionBand, "_", " ")))
	}
	if asset.VisualRole != "" && asset.VisualRole != "unknown" {
		evidence = append(evidence, fmt.Sprintf("Rol visual: %s.", strings.ReplaceAll(asset.VisualRole, "_", " ")))
	}
	if len(evidence) < 3 && asset.NaturalWidth > 0 && asset.NaturalHeight > 0 {
		evidence = append(evidence, fmt.Sprintf("Tamaño natural: %dx%d.", asset.NaturalWidth, asset.NaturalHeight))
	}
	if len(evidence) < 3 && asset.VisibleRatio > 0 {
		evidence = append(evidence, fmt.Sprintf("Visible en el primer viewport: %.1f%%.", asset.VisibleRatio*100))
	}
	if len(evidence) < 3 && finding != nil {
		for _, item := range trimEvidence(finding.Evidence, 3) {
			if item == "" || containsString(evidence, item) {
				continue
			}
			evidence = append(evidence, item)
			if len(evidence) == 3 {
				break
			}
		}
	}
	if len(evidence) < 3 && action != nil {
		for _, item := range trimEvidence(action.Evidence, 3) {
			if item == "" || containsString(evidence, item) {
				continue
			}
			evidence = append(evidence, item)
			if len(evidence) == 3 {
				break
			}
		}
	}
	return evidence
}
func inferAssetLCPImpact(asset ResourceContext, finding *AnalysisFindingContext, action *TopAction) string {
	if action != nil && action.LikelyLCPImpact != "" {
		return action.LikelyLCPImpact
	}
	if finding != nil {
		switch finding.ID {
		case "render_lcp_candidate":
			return "high"
		case "render_lcp_dom_node", "heavy_above_fold_media", "main_thread_cpu_pressure", "responsive_image_overdelivery":
			return "medium"
		}
	}
	switch asset.VisualRole {
	case "lcp_candidate":
		return "high"
	case "hero_media", "above_fold_media":
		return "medium"
	default:
		return "low"
	}
}
func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func materiallyOversizedImage(asset ResourceContext) bool {
	if asset.Type != "image" || asset.ResponsiveImage || asset.NaturalWidth <= 0 {
		return false
	}
	return asset.Bytes >= 32_000 || asset.EstimatedSavingsBytes >= 16_000
}
func lightlyOversizedImage(asset ResourceContext) bool {
	if asset.Type != "image" || asset.ResponsiveImage || asset.NaturalWidth <= 0 {
		return false
	}
	return !materiallyOversizedImage(asset)
}
func isLogoLikeRaster(asset ResourceContext) bool {
	if asset.Type != "image" {
		return false
	}
	mimeType := strings.ToLower(asset.MIMEType)
	if !(strings.Contains(mimeType, "png") || strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg")) {
		return false
	}
	if asset.NaturalWidth <= 0 || asset.NaturalHeight <= 0 {
		return false
	}
	haystack := strings.ToLower(asset.URL)
	if !containsAny(haystack, "logo", "brand", "wordmark", "logotype") {
		return false
	}
	if asset.VisualRole != "above_fold_media" && asset.VisualRole != "hero_media" {
		return false
	}
	return float64(asset.NaturalWidth)/float64(asset.NaturalHeight) >= 1.5
}
func containsAny(value string, tokens ...string) bool {
	for _, token := range tokens {
		if strings.Contains(value, token) {
			return true
		}
	}
	return false
}
func isIconFontResourceContext(asset ResourceContext) bool {
	if asset.Type != "font" {
		return false
	}
	haystack := strings.ToLower(strings.TrimSpace(asset.URL))
	return containsAny(haystack, "font-awesome", "fontawesome", "fa-solid", "fa-regular", "fa-brands", "materialicons", "material-icons")
}
