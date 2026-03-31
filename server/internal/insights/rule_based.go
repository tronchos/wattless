package insights

import (
	"context"
	"fmt"
	"sort"
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
	case resource.IsThirdPartyTool && resource.ThirdPartyKind == "ads":
		return "Reduce el stack publicitario temprano, limita auctions/iframes y difiere piezas no esenciales fuera del arranque."
	case resource.IsThirdPartyTool && resource.ThirdPartyKind == "social":
		return "Difiere embeds y widgets sociales hasta interacción o viewport para evitar que compitan con la carga inicial."
	case resource.IsThirdPartyTool && resource.ThirdPartyKind == "payment":
		return "Carga el widget de ticketing o pago solo cuando el usuario lo necesite para que no compita con el arranque."
	case resource.IsThirdPartyTool && resource.ThirdPartyKind == "video_embed":
		return "Usa un placeholder ligero y monta el iframe o player de video solo al interactuar o al entrar en viewport."
	case isIconFontResourceContext(resource):
		return "Sustituye la fuente de iconos por SVGs individuales o genera un subset con solo los glifos que realmente usa la interfaz."
	case resource.Type == "font":
		return "Subconjunta la fuente, limita variantes y sirve solo WOFF2."
	case resource.Type == "script":
		return "Divide el script, difiere el código no crítico y vigila Long Tasks además del peso transferido."
	case isLogoLikeRaster(resource):
		return "Si este logo puede ser vectorial, pásalo a SVG. Si debe seguir siendo bitmap, sirve una variante raster 2x del tamaño visible en lugar del original completo."
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
func SuggestRuleBasedResource(resource ResourceContext) string {
	return NewRuleBasedProvider().SuggestResource(resource)
}
func prioritizeFindings(findings []AnalysisFindingContext) []AnalysisFindingContext {
	sorted := append([]AnalysisFindingContext(nil), findings...)
	sort.Slice(sorted, func(i, j int) bool {
		if SeverityRank(sorted[i].Severity) == SeverityRank(sorted[j].Severity) {
			if ConfidenceRank(sorted[i].Confidence) == ConfidenceRank(sorted[j].Confidence) {
				if FindingPriorityRank(sorted[i].ID) == FindingPriorityRank(sorted[j].ID) {
					if sorted[i].EstimatedSavingsBytes == sorted[j].EstimatedSavingsBytes {
						return sorted[i].ID < sorted[j].ID
					}
					return sorted[i].EstimatedSavingsBytes > sorted[j].EstimatedSavingsBytes
				}
				return FindingPriorityRank(sorted[i].ID) > FindingPriorityRank(sorted[j].ID)
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
	case "dominant_image_overdelivery":
		return "low"
	case "third_party_ads_overhead":
		return "low"
	case "third_party_social_overhead":
		return "low"
	default:
		return "low"
	}
}
