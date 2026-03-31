package scanner

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tronchos/wattless/server/internal/insights"
)

const (
	nearThresholdLongTasksMS    int64 = 200
	mainThreadLongTasksMS       int64 = 250
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
	lazyMajorityThreshold             = 0.70
)

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
		evidence = append(evidence, fmt.Sprintf("Posición visual: %s.", strings.ReplaceAll(string(candidate.PositionBand), "_", " ")))
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
			return nil
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
			evidence = append(evidence, fmt.Sprintf("Su posición es %s.", strings.ReplaceAll(string(resource.PositionBand), "_", " ")))
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
		fmt.Sprintf("Posición dominante: %s.", strings.ReplaceAll(string(candidate.PositionBand), "_", " ")),
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
		evidence = append(evidence, fmt.Sprintf("Posición visual: %s.", strings.ReplaceAll(string(candidate.PositionBand), "_", " ")))
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
			fmt.Sprintf("Rol visual detectado: %s.", strings.ReplaceAll(string(candidate.VisualRole), "_", " ")),
		},
		EstimatedSavingsBytes: estimateResourceSavings(*candidate),
		RelatedResourceIDs:    []string{candidate.ID},
	}
}
