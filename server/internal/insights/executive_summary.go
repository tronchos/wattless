package insights

import (
	"fmt"
	"strings"
)

func buildExecutiveSummary(report ReportContext) string {
	top := firstFinding(report.Analysis.Findings)
	gallery := dominantRepeatedGalleryGroup(report.Analysis.ResourceGroups)
	textualLead := textualFirstRenderLead(report)
	dominantThirdParty := dominantEditorialThirdPartyGroup(report)
	hasMeasuredLCP := report.Performance.RenderMetricsComplete && report.Performance.LCPMS > 0
	switch {
	case top != nil && top.ID == "dominant_image_overdelivery":
		return textualLead + "un solo asset visual concentra demasiado peso para el valor que aporta. Antes de afinar el resto del sitio, conviene corregir esa sobreentrega desproporcionada."
	case top != nil && top.ID == "render_lcp_candidate":
		return textualLead + "el cuello de botella principal de esta página sigue estando en la carga inicial: Wattless detectó una oportunidad de recorte en el recurso que gobierna el LCP."
	case top != nil && top.ID == "render_lcp_dom_node":
		return textualLead + "el render crítico tiene margen claro de mejora: Wattless detectó un texto o estructura bloqueando el LCP. Conviene revisar CSS, tipografía y tiempo de ejecución antes de culpar imágenes."
	case top != nil && top.ID == "main_thread_cpu_pressure":
		if top.Confidence == "low" {
			return textualLead + "la presión de CPU aparece cerca del umbral en este scan. No es la señal más estable del informe, pero conviene vigilarla porque puede competir con el render inicial."
		}
		return textualLead + "el peso de red no explica todo el problema: Wattless detectó presión real de CPU y Long Tasks que compiten con la experiencia inicial."
	case top != nil && top.ID == "third_party_ads_overhead":
		return textualLead + "el mayor coste evitable está en el stack publicitario: suma scripts, auctions e iframes externos antes de aportar valor editorial."
	case top != nil && top.ID == "third_party_payment_overhead":
		return textualLead + "el mayor coste total viene del ticketing/pago embebido. Antes de afinar grids secundarios, conviene reducir esa dependencia de terceros."
	case top != nil && top.ID == "third_party_video_overhead":
		return textualLead + "una parte material del coste viene de players, iframes y thumbnails de video externos. Diferirlos devuelve margen sin tocar el contenido crítico."
	case top != nil && top.ID == "third_party_analytics_overhead":
		return textualLead + "la capa de analítica ya mete suficiente ruido de red como para merecer recorte antes de seguir afinando detalles secundarios."
	case top != nil && top.ID == "legacy_image_format_overhead":
		return textualLead + "una parte material del peso de imagen sigue en formatos legacy. Aquí hay un recorte bastante limpio sin tocar el layout ni la narrativa visual."
	case top != nil && top.ID == "legacy_font_format_overhead":
		return textualLead + "parte del coste tipográfico sigue viniendo de formatos legacy. Servir WOFF2 como camino principal da un recorte limpio en el arranque."
	case dominantThirdParty != nil:
		return textualLead + fmt.Sprintf("el mayor coste total viene de %s: %s. Antes de afinar grids secundarios, conviene reducir esa dependencia de terceros.", strings.ToLower(withArticle(dominantThirdParty.Label)), formatBytes(dominantThirdParty.TotalBytes))
	case report.Summary.ThirdPartyBytes*2 >= report.TotalBytesTransferred && report.TotalBytesTransferred > 0:
		return textualLead + "la experiencia inicial se sostiene, pero la mayor parte del peso total viene de terceros y eso infla bytes, variabilidad y coste por visita."
	case gallery != nil && gallery.TotalBytes >= 400_000 && hasMeasuredLCP && report.Performance.LCPMS < 2_000 && gallery.PositionBand == "below_fold":
		return textualLead + fmt.Sprintf("la home visual es ágil, pero %s bajo el fold siguen inflando el coste por visita más de lo que parece.", withArticle(gallery.Label))
	case gallery != nil && gallery.TotalBytes >= 400_000 && hasMeasuredLCP && report.Performance.LCPMS < 2_000:
		return textualLead + fmt.Sprintf("la home visual arranca bien, pero %s siguen inflando el coste por visita más de lo que parece.", withArticle(gallery.Label))
	case report.Performance.LongTasksTotalMS >= 250:
		return textualLead + "el peso de red no explica todo el problema: hay presión real de CPU y Long Tasks que compiten con la experiencia inicial."
	case report.Analysis.Summary.FontBytes >= 250_000:
		return textualLead + "la base es razonable, pero la pila tipográfica sigue siendo más cara de lo necesario para una carga inicial eficiente."
	case !report.Performance.RenderMetricsComplete:
		return textualLead + "la transferencia ya da señales útiles, pero este scan no capturó todas las métricas de render; conviene apoyarse más en bytes críticos, CPU y terceros."
	default:
		return "El informe separa transferencia, render crítico y peso bajo el fold para que el siguiente arreglo tenga retorno real y no solo cambie un número aislado."
	}
}
func buildPitchLine(report ReportContext) string {
	gallery := dominantRepeatedGalleryGroup(report.Analysis.ResourceGroups)
	top := firstFinding(report.Analysis.Findings)
	textualLead := textualFirstRenderPitchLead(report)
	dominantThirdParty := dominantEditorialThirdPartyGroup(report)
	hasMeasuredLCP := report.Performance.RenderMetricsComplete && report.Performance.LCPMS > 0
	switch {
	case top != nil && top.ID == "dominant_image_overdelivery":
		return textualLead + "aquí hay un win claro: corregir una sola imagen dominante puede bajar muchísimo el peso total sin tocar el resto del sitio."
	case top != nil && top.ID == "render_lcp_dom_node":
		return textualLead + "el siguiente salto no está en otra imagen: conviene ordenar CSS, tipografía y CPU del nodo que domina el LCP."
	case top != nil && top.ID == "render_lcp_candidate":
		return textualLead + "el mejor retorno inmediato está en el recurso que coincide con el LCP."
	case top != nil && top.ID == "main_thread_cpu_pressure" && top.Confidence == "low":
		return textualLead + "la CPU ya enseña fricción cerca del umbral; vigilarla ahora evita que esa variabilidad termine empeorando el arranque."
	case top != nil && top.ID == "main_thread_cpu_pressure":
		return textualLead + "reducir trabajo real de CPU es el siguiente paso para devolver fluidez al arranque."
	case top != nil && top.ID == "third_party_ads_overhead":
		return textualLead + "ahora toca recortar el stack publicitario temprano para bajar variabilidad, scripts externos y bytes sin tocar la parte editorial."
	case top != nil && top.ID == "third_party_payment_overhead":
		return textualLead + "ahora toca diferir ticketing y pago para bajar variabilidad y bytes sin tocar el contenido crítico."
	case top != nil && top.ID == "third_party_video_overhead":
		return textualLead + "ahora toca recortar los embeds de video para bajar variabilidad y bytes sin tocar el contenido crítico."
	case top != nil && top.ID == "third_party_analytics_overhead":
		return textualLead + "ahora toca limpiar la sobrecarga de analítica para que no siga compitiendo con el arranque."
	case top != nil && top.ID == "legacy_image_format_overhead":
		return textualLead + "el siguiente recorte limpio viene de migrar imagen legacy a formatos modernos antes que de seguir afinando piezas sueltas."
	case top != nil && top.ID == "legacy_font_format_overhead":
		return textualLead + "migrar las fuentes pesadas a WOFF2 es un recorte limpio que no debería tocar la identidad visual."
	case dominantThirdParty != nil:
		return textualLead + fmt.Sprintf("ahora toca recortar %s para bajar variabilidad y bytes sin tocar el contenido crítico.", strings.ToLower(withArticle(dominantThirdParty.Label)))
	case report.Summary.ThirdPartyBytes*2 >= report.TotalBytesTransferred && report.TotalBytesTransferred > 0:
		return textualLead + "el siguiente salto viene de reducir terceros antes que de seguir afinando solo media propia."
	case gallery != nil && gallery.TotalBytes >= 400_000 && hasMeasuredLCP && report.Performance.LCPMS < 2_000 && gallery.PositionBand == "below_fold":
		return textualLead + fmt.Sprintf("ahora toca recortar el coste acumulado de %s bajo el fold para bajar bytes sin sacrificar UX.", withArticle(gallery.Label))
	case gallery != nil && gallery.TotalBytes >= 400_000 && hasMeasuredLCP && report.Performance.LCPMS < 2_000:
		return textualLead + fmt.Sprintf("ahora toca recortar el coste repetido de %s para bajar bytes sin sacrificar UX.", withArticle(gallery.Label))
	case report.Analysis.Summary.AnalyticsBytes >= 80_000:
		return "Separar render crítico de sobrecarga de terceros te da una mejora doble: menos variabilidad y menos CO2 por visita."
	case report.Performance.LongTasksTotalMS >= 250:
		return textualLead + "Aquí no basta con comprimir archivos: reducir trabajo real de CPU es lo que devolverá fluidez al arranque."
	case !report.Performance.RenderMetricsComplete:
		return textualLead + "con métricas de render incompletas, el siguiente paso fiable está en recortar bytes críticos, terceros y trabajo de CPU antes que en leer demasiado un LCP ausente."
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
func withArticle(label string) string {
	switch strings.ToLower(strings.TrimSpace(label)) {
	case "miniaturas del blog":
		return "las miniaturas del blog"
	case "fotos de speakers":
		return "las fotos de speakers"
	case "logos de partners":
		return "los logos de partners"
	case "logos de sponsors":
		return "los logos de sponsors"
	case "banderas":
		return "las banderas"
	case "avatares":
		return "los avatares"
	case "cluster de pagos":
		return "el cluster de pagos"
	case "cluster de anuncios":
		return "el cluster de anuncios"
	case "embeds de video":
		return "los embeds de video"
	case "colección de miniaturas":
		return "la colección de miniaturas"
	case "grid de tarjetas":
		return "el grid de tarjetas"
	default:
		return strings.ToLower(strings.TrimSpace(label))
	}
}
func reportHasTextualFirstRender(report ReportContext) bool {
	if report.Analysis.Summary.AboveFoldVisualBytes != 0 || report.Analysis.Summary.RenderCriticalBytes <= 0 {
		return false
	}
	for _, finding := range report.Analysis.Findings {
		if finding.ID == "render_lcp_dom_node" {
			return true
		}
	}
	return false
}
func textualFirstRenderLead(report ReportContext) string {
	if !reportHasTextualFirstRender(report) {
		return ""
	}
	return "El primer render depende sobre todo de texto, fuentes y CSS; "
}
func textualFirstRenderPitchLead(report ReportContext) string {
	if !reportHasTextualFirstRender(report) {
		return ""
	}
	return "El primer render ya vive en texto, fuentes y CSS; "
}
func dominantEditorialThirdPartyGroup(report ReportContext) *ResourceGroupContext {
	if report.TotalBytesTransferred <= 0 {
		return nil
	}
	gallery := dominantRepeatedGalleryGroup(report.Analysis.ResourceGroups)
	var candidate *ResourceGroupContext
	for index := range report.Analysis.ResourceGroups {
		group := &report.Analysis.ResourceGroups[index]
		if group.Kind != "third_party_cluster" || !isEditorialThirdPartyLabel(group.Label) {
			continue
		}
		if candidate == nil || group.TotalBytes > candidate.TotalBytes {
			candidate = group
		}
	}
	if candidate == nil {
		return nil
	}
	if report.Summary.ThirdPartyBytes*2 >= report.TotalBytesTransferred {
		return candidate
	}
	if gallery != nil && candidate.TotalBytes > gallery.TotalBytes {
		return candidate
	}
	return nil
}
func isEditorialThirdPartyLabel(label string) bool {
	haystack := strings.ToLower(strings.TrimSpace(label))
	return strings.Contains(haystack, "pagos") || strings.Contains(haystack, "video") || strings.Contains(haystack, "anuncios")
}
