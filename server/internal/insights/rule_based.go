package insights

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type RuleBasedProvider struct{}

func NewRuleBasedProvider() RuleBasedProvider {
	return RuleBasedProvider{}
}

func (RuleBasedProvider) Name() string {
	return "rule_based"
}

func (RuleBasedProvider) SuggestResource(resource ResourceContext) string {
	switch {
	case resource.Failed:
		return "Corrige el fallo de esta petición o elimina la dependencia si ya no aporta valor."
	case resource.VisualRole == "lcp_candidate":
		return "Este recurso coincide con el LCP. Prioriza compresión, dimensionado correcto y una carga crítica más disciplinada."
	case resource.VisualRole == "hero_media":
		return "Esta media vive above the fold. Si la recortas o la sirves con mejor responsive imaging, reduces presión en el arranque visible."
	case resource.VisualRole == "repeated_card_media":
		return "Forma parte de una galería repetida. Optimízala en lote y sirve versiones más pequeñas para bajar el coste por visita."
	case resource.IsThirdPartyTool && resource.ThirdPartyKind == "analytics":
		return "Retrasa la analítica no esencial hasta interacción o agrupa proveedores para bajar ruido de red."
	case resource.Type == "font":
		return "Subconjunta la fuente, limita variantes y sirve solo WOFF2."
	case resource.Type == "script":
		return "Divide el script, difiere el código no crítico y vigila Long Tasks además del peso transferido."
	case lightlyOversizedImage(resource):
		return "Sirve una variante más pequeña para esta imagen. El impacto total es limitado, pero evita entregar mucha más resolución de la necesaria."
	case resource.Type == "image":
		return "Comprime y dimensiona este asset según su rol visual real, no solo por su peso bruto."
	default:
		return "Reduce transferencia o carga este recurso de forma más diferida."
	}
}

func (provider RuleBasedProvider) SummarizeReport(_ context.Context, report ReportContext) (ProviderResult, error) {
	findings := prioritizeFindings(report.Analysis.Findings)
	actions := make([]TopAction, 0, 3)

	for index, finding := range findings {
		if index >= 3 {
			break
		}

		action := TopAction{
			ID:                    fmt.Sprintf("act-%d", index+1),
			RelatedFindingID:      finding.ID,
			Title:                 finding.Title,
			Reason:                finding.Summary,
			Confidence:            finding.Confidence,
			Evidence:              append([]string(nil), finding.Evidence...),
			EstimatedSavingsBytes: finding.EstimatedSavingsBytes,
			LikelyLCPImpact:       findingImpact(finding),
			RelatedResourceIDs:    append([]string(nil), finding.RelatedResourceIDs...),
		}

		if finding.Confidence == "high" || finding.Confidence == "medium" {
			action.RecommendedFix = recommendedFixForFinding(report, finding)
		}

		actions = append(actions, action)
	}

	summary := buildExecutiveSummary(report)
	pitchLine := buildPitchLine(report)

	assetInsights := make([]AssetInsightDraft, 0, len(report.TopResources))
	for _, asset := range report.TopResources {
		assetInsights = append(assetInsights, BuildRuleBasedAssetInsight(asset, report.Analysis.Findings, actions))
	}

	return ProviderResult{
		Insights: ScanInsights{
			Provider:         provider.Name(),
			ExecutiveSummary: summary,
			PitchLine:        pitchLine,
			TopActions:       actions,
		},
		AssetInsights: assetInsights,
	}, nil
}

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

func SuggestRuleBasedResource(resource ResourceContext) string {
	return NewRuleBasedProvider().SuggestResource(resource)
}

func prioritizeFindings(findings []AnalysisFindingContext) []AnalysisFindingContext {
	sorted := append([]AnalysisFindingContext(nil), findings...)
	sort.Slice(sorted, func(i, j int) bool {
		if SeverityRank(sorted[i].Severity) == SeverityRank(sorted[j].Severity) {
			if ConfidenceRank(sorted[i].Confidence) == ConfidenceRank(sorted[j].Confidence) {
				if sorted[i].EstimatedSavingsBytes == sorted[j].EstimatedSavingsBytes {
					return sorted[i].ID < sorted[j].ID
				}
				return sorted[i].EstimatedSavingsBytes > sorted[j].EstimatedSavingsBytes
			}
			return ConfidenceRank(sorted[i].Confidence) > ConfidenceRank(sorted[j].Confidence)
		}
		return SeverityRank(sorted[i].Severity) > SeverityRank(sorted[j].Severity)
	})
	return sorted
}

func findingImpact(finding AnalysisFindingContext) string {
	switch finding.ID {
	case "render_lcp_candidate":
		return "high"
	case "render_lcp_dom_node":
		if finding.Severity == "high" {
			return "high"
		}
		return "medium"
	case "main_thread_cpu_pressure":
		if finding.Severity == "low" || finding.Confidence == "low" {
			return "low"
		}
		if finding.Severity == "high" {
			return "high"
		}
		return "medium"
	case "responsive_image_overdelivery":
		return "medium"
	case "heavy_above_fold_media":
		return "medium"
	default:
		return "low"
	}
}

func recommendedFixForFinding(report ReportContext, finding AnalysisFindingContext) *RecommendedFix {
	framework := normalizedFrameworkHint(report.SiteProfile.FrameworkHint)
	switch finding.ID {
	case "render_lcp_candidate", "heavy_above_fold_media":
		return &RecommendedFix{
			Summary:       "Plantilla base para reducir el peso del media crítico y ajustar prioridad de carga sin romper el layout.",
			OptimizedCode: heroMediaOptimizedCode(framework),
			Changes: []string{
				"Dimensiones explícitas para evitar trabajo extra de layout",
				"Calidad controlada y formato moderno para bajar bytes",
				"Prioridad reservada solo al asset crítico de verdad",
			},
			ExpectedImpact: "Menos peso en el render crítico y mejor margen para el LCP.",
		}
	case "render_lcp_dom_node":
		return &RecommendedFix{
			Summary:       "Punto de partida para un LCP textual: fuente disciplinada, CSS estable y menos trabajo bloqueante en el primer render.",
			OptimizedCode: textLCPOtimizedCode(framework),
			Changes: []string{
				"Se reduce incertidumbre tipográfica en el nodo que domina el LCP",
				"Se favorece una pintura estable sin depender de un asset visual pesado",
				"El siguiente paso es revisar CSS crítico y Long Tasks del arranque",
			},
			ExpectedImpact: "Menos espera en el nodo textual que domina el render inicial.",
		}
	case "repeated_gallery_overdelivery":
		return &RecommendedFix{
			Summary:       "Patrón para listas visuales repetidas del catálogo: primeras tarjetas visibles con prioridad y el resto con variantes pequeñas y carga diferida.",
			OptimizedCode: repeatedGalleryOptimizedCode(framework),
			Changes: []string{
				"Eager y prioridad solo para la primera fila visible",
				"Variantes pequeñas y sizes realistas para cada tarjeta",
				"Lazy loading para el resto del grid repetido",
			},
			ExpectedImpact: "Menor coste por visita sin tocar el primer render.",
		}
	case "third_party_analytics_overhead":
		return &RecommendedFix{
			Summary:       "Patrón conservador para retrasar tags de terceros y evitar que compitan con el arranque.",
			OptimizedCode: deferredAnalyticsCode(framework),
			Changes: []string{
				"Se difiere el proveedor hasta que la página ya cargó",
				"Se reduce ruido de red durante el render inicial",
			},
			ExpectedImpact: "Menos variabilidad y menos presión de terceros en el arranque.",
		}
	case "font_stack_overweight":
		return &RecommendedFix{
			Summary: "Punto de partida para limitar variantes y servir una pila tipográfica más pequeña.",
			OptimizedCode: `@font-face {
  font-family: "Brand Sans";
  src: url("/fonts/brand-sans-subset.woff2") format("woff2");
  font-display: swap;
  font-weight: 400 700;
}`,
			Changes: []string{
				"Subset de glifos y una sola variante moderna",
				"font-display: swap para no bloquear contenido",
			},
			ExpectedImpact: "Menos transferencia tipográfica y render inicial más estable.",
		}
	case "main_thread_cpu_pressure":
		return &RecommendedFix{
			Summary:       "Ejemplo de importación diferida para bajar trabajo de CPU del arranque.",
			OptimizedCode: deferredCPUCode(framework),
			Changes: []string{
				"El código pesado deja de competir con la hebra principal al inicio",
				"Se aísla JS costoso fuera del camino crítico",
			},
			ExpectedImpact: "Menos Long Tasks y mejor respuesta percibida.",
		}
	case "responsive_image_overdelivery":
		return &RecommendedFix{
			Summary:       "Sirve una imagen adaptada a su caja renderizada y declara variantes para no mandar desktop completo a cajas pequeñas.",
			OptimizedCode: responsiveImageCode(framework),
			Changes: []string{
				"srcset/sizes o equivalente del framework para ajustar el tamaño real",
				"Variantes pensadas para la caja visible y para pantallas 2x",
				"Menos bytes sin perder nitidez perceptible",
			},
			ExpectedImpact: "Menos transferencia por imagen sin degradar la experiencia visual.",
		}
	default:
		return nil
	}
}

func buildExecutiveSummary(report ReportContext) string {
	top := firstFinding(report.Analysis.Findings)
	gallery := dominantRepeatedGalleryGroup(report.Analysis.ResourceGroups)
	switch {
	case top != nil && top.ID == "render_lcp_candidate":
		return "El cuello de botella principal está en el render crítico: Wattless detectó un recurso que coincide con el LCP y concentra el mejor retorno inmediato."
	case top != nil && top.ID == "render_lcp_dom_node":
		return "El cuello de botella principal está en el render crítico: Wattless detectó un nodo del DOM que domina el LCP y conviene revisar CSS, tipografía y CPU antes de culpar assets visuales."
	case top != nil && top.ID == "main_thread_cpu_pressure":
		if top.Confidence == "low" {
			return "La presión de CPU aparece cerca del umbral en este scan. No es la señal más estable del informe, pero conviene vigilarla porque puede competir con el render inicial."
		}
		return "El peso de red no explica todo el problema: Wattless detectó presión real de CPU y Long Tasks que compiten con la experiencia inicial."
	case gallery != nil && gallery.TotalBytes >= 400_000 && report.Performance.LCPMS < 2_000 && gallery.PositionBand == "below_fold":
		return "La home es rápida en el primer render, pero el catálogo visual por debajo del fold infla el coste por visita más de lo que parece."
	case gallery != nil && gallery.TotalBytes >= 400_000 && report.Performance.LCPMS < 2_000:
		return "La home es rápida en el primer render, pero el catálogo visual repetido sigue inflando el coste por visita más de lo que parece."
	case report.Performance.LongTasksTotalMS >= 250:
		return "El peso de red no explica todo el problema: hay presión real de CPU y Long Tasks que compiten con la experiencia inicial."
	case report.Analysis.Summary.FontBytes >= 250_000:
		return "La base es razonable, pero la pila tipográfica sigue siendo más cara de lo necesario para una carga inicial eficiente."
	default:
		return "El informe separa transferencia, render crítico y peso bajo el fold para que el siguiente arreglo tenga retorno real y no solo cambie un número aislado."
	}
}

func buildPitchLine(report ReportContext) string {
	gallery := dominantRepeatedGalleryGroup(report.Analysis.ResourceGroups)
	top := firstFinding(report.Analysis.Findings)
	switch {
	case gallery != nil && gallery.TotalBytes >= 400_000 && report.Performance.LCPMS < 2_000 && gallery.PositionBand == "below_fold":
		return "Tu arranque ya va bien; ahora toca recortar el coste visual acumulado bajo el fold para bajar bytes sin sacrificar UX."
	case gallery != nil && gallery.TotalBytes >= 400_000 && report.Performance.LCPMS < 2_000:
		return "Tu arranque ya va bien; ahora toca recortar el coste visual repetido del catálogo para bajar bytes sin sacrificar UX."
	case top != nil && top.ID == "main_thread_cpu_pressure" && top.Confidence == "low":
		return "La CPU ya enseña fricción cerca del umbral; vigilarla ahora evita que esa variabilidad termine empeorando el arranque."
	case report.Analysis.Summary.AnalyticsBytes >= 80_000:
		return "Separar render crítico de sobrecarga de terceros te da una mejora doble: menos variabilidad y menos CO2 por visita."
	case report.Performance.LongTasksTotalMS >= 250:
		return "Aquí no basta con comprimir archivos: reducir trabajo real de CPU es lo que devolverá fluidez al arranque."
	default:
		return "Menos bytes donde importan y menos ruido donde no aportan valor: esa es la diferencia entre una web ligera y una web defendible."
	}
}

func firstFinding(findings []AnalysisFindingContext) *AnalysisFindingContext {
	if len(findings) == 0 {
		return nil
	}
	return &findings[0]
}

func dominantRepeatedGalleryGroup(groups []ResourceGroupContext) *ResourceGroupContext {
	var candidate *ResourceGroupContext
	for index := range groups {
		group := &groups[index]
		if group.Kind != "repeated_gallery" || group.TotalBytes < 400_000 {
			continue
		}
		if candidate == nil || group.TotalBytes > candidate.TotalBytes {
			candidate = group
		}
	}
	return candidate
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
		case asset.VisualRole == "repeated_card_media" && finding.ID == "repeated_gallery_overdelivery":
			candidates = append(candidates, finding)
		case asset.VisualRole == "lcp_candidate" && finding.ID == "render_lcp_candidate":
			candidates = append(candidates, finding)
		case (asset.VisualRole == "hero_media" || asset.VisualRole == "above_fold_media") && finding.ID == "heavy_above_fold_media":
			candidates = append(candidates, finding)
		case asset.Type == "font" && finding.ID == "font_stack_overweight":
			candidates = append(candidates, finding)
		case asset.IsThirdPartyTool && asset.ThirdPartyKind == "analytics" && finding.ID == "third_party_analytics_overhead":
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
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "analytics" && finding.ID == "third_party_analytics_overhead":
		return 3
	case asset.Type == "font" && finding.ID == "font_stack_overweight":
		return 3
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
	case asset.Type == "font":
		return "Coste tipográfico concentrado"
	case asset.IsThirdPartyTool && asset.ThirdPartyKind == "analytics":
		return "Sobrecarga de analítica"
	case asset.VisualRole == "lcp_candidate":
		return "Candidato real al LCP"
	case asset.VisualRole == "repeated_card_media":
		return "Media repetida en el catálogo"
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
		case "font_stack_overweight":
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

func formatBytes(bytes int64) string {
	switch {
	case bytes >= 1_000_000:
		return fmt.Sprintf("%.2f MB", float64(bytes)/1_000_000)
	case bytes >= 1_000:
		return fmt.Sprintf("%.0f KB", float64(bytes)/1_000)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
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

func normalizedFrameworkHint(value string) string {
	switch value {
	case "astro", "nextjs", "generic", "unknown":
		return value
	default:
		return "generic"
	}
}

func heroMediaOptimizedCode(framework string) string {
	switch framework {
	case "astro":
		return `---
import { Image } from "astro:assets";
import hero from "../assets/hero.webp";
---

<Image
  src={hero}
  alt="Asset crítico"
  widths={[640, 960, 1280]}
  sizes="100vw"
  loading="eager"
  fetchpriority="high"
  format="avif"
/>`
	case "generic", "unknown":
		return `<img
  src="/asset-critico-1280.webp"
  srcset="/asset-critico-640.webp 640w, /asset-critico-960.webp 960w, /asset-critico-1280.webp 1280w"
  sizes="100vw"
  width="1280"
  height="720"
  loading="eager"
  fetchpriority="high"
  alt="Asset crítico"
/>`
	default:
		return `import Image from "next/image";

export function HeroAsset() {
  return (
    <Image
      src="/asset-critico.webp"
      alt="Asset crítico"
      width={1280}
      height={720}
      priority
      sizes="100vw"
      quality={72}
    />
  );
}`
	}
}

func textLCPOtimizedCode(framework string) string {
	if framework == "nextjs" {
		return `import { Inter } from "next/font/google";

const inter = Inter({
  subsets: ["latin"],
  display: "swap",
  preload: true,
});

export function HeroCopy() {
  return (
    <section>
      <h1 className={inter.className}>Contenido crítico listo para pintar</h1>
    </section>
  );
}`
	}

	return `@font-face {
  font-family: "Brand Sans";
  src: url("/fonts/brand-sans-subset.woff2") format("woff2");
  font-display: swap;
  font-weight: 400 700;
}

.hero-title {
  font-family: "Brand Sans", system-ui, sans-serif;
}`
}

func repeatedGalleryOptimizedCode(framework string) string {
	switch framework {
	case "astro":
		return `---
import { Image } from "astro:assets";
const firstRowCount = Math.max(1, Astro.props.firstRowCount ?? 1);
---

{courses.map((course, index) => (
  <Image
    src={course.cover}
    alt={course.title}
    widths={[320, 480, 640]}
    sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw"
    loading={index < firstRowCount ? "eager" : "lazy"}
    fetchpriority={index < firstRowCount ? "high" : "auto"}
  />
))}`
	case "generic", "unknown":
		return `function renderCourseGrid(courses, options) {
  options = options || {};
  const firstRowCount = Math.max(1, options.firstRowCount || 1);
  return courses.map(function (course, index) {
    const isVisibleRow = index < firstRowCount;
    return '<img ' +
      'src="' + course.cover480 + '" ' +
      'srcset="' + course.cover320 + ' 320w, ' + course.cover480 + ' 480w, ' + course.cover640 + ' 640w" ' +
      'sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw" ' +
      'width="480" height="270" ' +
      'loading="' + (isVisibleRow ? "eager" : "lazy") + '" ' +
      'fetchpriority="' + (isVisibleRow ? "high" : "auto") + '" ' +
      'alt="' + course.title + '">' ;
  }).join("");
}`
	default:
		return `import Image from "next/image";

export function CourseGrid({ courses, firstRowCount = 1 }) {
  return courses.map((course, index) => (
    <Image
      key={course.slug}
      src={course.cover}
      alt={course.title}
      width={480}
      height={270}
      sizes="(max-width: 768px) 100vw, (max-width: 1200px) 50vw, 33vw"
      loading={index < firstRowCount ? "eager" : "lazy"}
      fetchPriority={index < firstRowCount ? "high" : "auto"}
    />
  ));
}`
	}
}

func deferredAnalyticsCode(framework string) string {
	if framework == "nextjs" {
		return `import Script from "next/script";

export function DeferredAnalytics() {
  return (
    <Script
      src="https://analytics.example.com/tag.js"
      strategy="lazyOnload"
    />
  );
}`
	}

	return `<script>
  window.addEventListener("load", () => {
    requestIdleCallback(() => {
      const script = document.createElement("script");
      script.src = "https://analytics.example.com/tag.js";
      script.async = true;
      document.head.appendChild(script);
    });
  });
</script>`
}

func deferredCPUCode(framework string) string {
	if framework == "nextjs" {
		return `import dynamic from "next/dynamic";

const HeavyWidget = dynamic(() => import("./heavy-widget"), {
  ssr: false,
});

export function DeferredWidget() {
  return <HeavyWidget />;
}`
	}

	return `<script type="module">
  requestIdleCallback(async () => {
    const { mountHeavyWidget } = await import("/scripts/heavy-widget.js");
    mountHeavyWidget(document.querySelector("[data-heavy-widget]"));
  });
</script>`
}

func responsiveImageCode(framework string) string {
	switch framework {
	case "astro":
		return `---
import { Image } from "astro:assets";
import cardCover from "../assets/card-cover.webp";
---

<Image
  src={cardCover}
  alt="Portada optimizada"
  widths={[320, 480, 640]}
  sizes="(max-width: 768px) 100vw, 480px"
  loading="eager"
/>`
	case "generic", "unknown":
		return `<img
  src="/card-cover-480.webp"
  srcset="/card-cover-320.webp 320w, /card-cover-480.webp 480w, /card-cover-640.webp 640w"
  sizes="(max-width: 768px) 100vw, 480px"
  width="480"
  height="270"
  alt="Portada optimizada"
/>`
	default:
		return `import Image from "next/image";

export function ResponsiveCardImage() {
  return (
    <Image
      src="/card-cover.webp"
      alt="Portada optimizada"
      width={480}
      height={270}
      sizes="(max-width: 768px) 100vw, 480px"
    />
  );
}`
	}
}
