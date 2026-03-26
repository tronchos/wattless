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
	case resource.Type == "image":
		return "Comprime y dimensiona este asset según su rol visual real, no solo por su peso bruto."
	default:
		return "Reduce transferencia o carga este recurso de forma más diferida."
	}
}

func (provider RuleBasedProvider) SummarizeReport(_ context.Context, report ReportContext) (ScanInsights, error) {
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

		if index == 0 && (finding.Confidence == "high" || finding.Confidence == "medium") {
			action.RecommendedFix = recommendedFixForFinding(finding)
		}

		actions = append(actions, action)
	}

	summary := buildExecutiveSummary(report)
	pitchLine := buildPitchLine(report)

	return ScanInsights{
		Provider:         provider.Name(),
		ExecutiveSummary: summary,
		PitchLine:        pitchLine,
		TopActions:       actions,
	}, nil
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
	case "main_thread_pressure":
		if finding.Severity == "high" {
			return "high"
		}
		return "medium"
	case "heavy_above_fold_media":
		return "medium"
	default:
		return "low"
	}
}

func recommendedFixForFinding(finding AnalysisFindingContext) *RecommendedFix {
	switch finding.ID {
	case "render_lcp_candidate", "heavy_above_fold_media":
		return &RecommendedFix{
			Summary: "Plantilla base para reducir el peso del media crítico y ajustar prioridad de carga sin romper el layout.",
			OptimizedCode: `import Image from "next/image";

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
}`,
			Changes: []string{
				"Dimensiones explícitas para evitar trabajo extra de layout",
				"Calidad controlada y formato moderno para bajar bytes",
				"Prioridad reservada solo al asset crítico de verdad",
			},
			ExpectedImpact: "Menos peso en el render crítico y mejor margen para el LCP.",
		}
	case "render_lcp_dom_node":
		return &RecommendedFix{
			Summary: "Punto de partida para un LCP textual: fuente disciplinada, CSS estable y menos trabajo bloqueante en el primer render.",
			OptimizedCode: `import { Inter } from "next/font/google";

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
}`,
			Changes: []string{
				"Se reduce incertidumbre tipográfica en el nodo que domina el LCP",
				"Se favorece una pintura estable sin depender de un asset visual pesado",
				"El siguiente paso es revisar CSS crítico y Long Tasks del arranque",
			},
			ExpectedImpact: "Menos espera en el nodo textual que domina el render inicial.",
		}
	case "below_fold_gallery_waste":
		return &RecommendedFix{
			Summary: "Patrón para listas visuales repetidas del catálogo: versiones más pequeñas y carga diferida por defecto.",
			OptimizedCode: `import Image from "next/image";

export function CourseCard({ course }) {
  return (
    <article>
      <Image
        src={course.cover}
        alt={course.title}
        width={480}
        height={270}
        loading="lazy"
        sizes="(max-width: 768px) 100vw, 33vw"
      />
    </article>
  );
}`,
			Changes: []string{
				"Lazy loading explícito para el grid repetido no crítico",
				"Tamaño realista para miniaturas y tarjetas",
				"Responsive sizes para no servir desktop en móvil",
			},
			ExpectedImpact: "Menor coste por visita sin tocar el primer render.",
		}
	case "third_party_analytics_overhead":
		return &RecommendedFix{
			Summary: "Patrón conservador para retrasar tags de terceros y evitar que compitan con el arranque.",
			OptimizedCode: `import Script from "next/script";

export function DeferredAnalytics() {
  return (
    <Script
      src="https://analytics.example.com/tag.js"
      strategy="lazyOnload"
    />
  );
}`,
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
	case "main_thread_pressure":
		return &RecommendedFix{
			Summary: "Ejemplo de importación diferida para bajar trabajo de CPU del arranque.",
			OptimizedCode: `import dynamic from "next/dynamic";

const HeavyWidget = dynamic(() => import("./heavy-widget"), {
  ssr: false,
});

export function DeferredWidget() {
  return <HeavyWidget />;
}`,
			Changes: []string{
				"El código pesado deja de competir con la hebra principal al inicio",
				"Se aísla JS costoso fuera del camino crítico",
			},
			ExpectedImpact: "Menos Long Tasks y mejor respuesta percibida.",
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
	switch {
	case gallery != nil && gallery.TotalBytes >= 400_000 && report.Performance.LCPMS < 2_000 && gallery.PositionBand == "below_fold":
		return "Tu arranque ya va bien; ahora toca recortar el coste visual acumulado bajo el fold para bajar bytes sin sacrificar UX."
	case gallery != nil && gallery.TotalBytes >= 400_000 && report.Performance.LCPMS < 2_000:
		return "Tu arranque ya va bien; ahora toca recortar el coste visual repetido del catálogo para bajar bytes sin sacrificar UX."
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

func conciseReason(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	return value
}
