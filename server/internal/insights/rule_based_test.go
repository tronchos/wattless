package insights

import (
	"context"
	"strings"
	"testing"
)

func TestBuildExecutiveSummaryAvoidsBelowFoldClaimForMixedGallery(t *testing.T) {
	report := ReportContext{
		Performance: PerformanceContext{
			LCPMS: 580,
		},
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				RepeatedGalleryBytes: 900_000,
			},
			ResourceGroups: []ResourceGroupContext{
				{
					ID:           "group-cards",
					Kind:         "repeated_gallery",
					TotalBytes:   900_000,
					PositionBand: "mixed",
				},
			},
		},
	}

	summary := buildExecutiveSummary(report)
	if strings.Contains(strings.ToLower(summary), "por debajo del fold") {
		t.Fatalf("expected conservative summary, got %q", summary)
	}
}

func TestBuildPitchLineAvoidsBelowFoldClaimForMixedGallery(t *testing.T) {
	report := ReportContext{
		Performance: PerformanceContext{
			LCPMS: 580,
		},
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				RepeatedGalleryBytes: 900_000,
			},
			ResourceGroups: []ResourceGroupContext{
				{
					ID:           "group-cards",
					Kind:         "repeated_gallery",
					TotalBytes:   900_000,
					PositionBand: "mixed",
				},
			},
		},
	}

	pitch := buildPitchLine(report)
	if strings.Contains(strings.ToLower(pitch), "bajo el fold") {
		t.Fatalf("expected conservative pitch, got %q", pitch)
	}
}

func TestBuildRuleBasedAssetInsightAvoidsBelowFoldClaimForMixedGalleryAsset(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:           "card-1",
			Type:         "image",
			Bytes:        220_000,
			PositionBand: "mixed",
			VisualRole:   "repeated_card_media",
		},
		[]AnalysisFindingContext{
			{
				ID:                    "repeated_gallery_overdelivery",
				Category:              "media",
				Severity:              "medium",
				Confidence:            "high",
				Title:                 "Galería repetida sobredimensionada",
				Summary:               "La galería repetida suma demasiado peso para el valor que aporta.",
				EstimatedSavingsBytes: 180_000,
				RelatedResourceIDs:    []string{"card-1", "card-2", "card-3"},
			},
		},
		nil,
	)

	if strings.Contains(strings.ToLower(draft.ShortProblem), "below the fold") {
		t.Fatalf("expected conservative asset copy, got %q", draft.ShortProblem)
	}
	if draft.Scope != "group" {
		t.Fatalf("expected group scope, got %q", draft.Scope)
	}
}

func TestBuildRuleBasedAssetInsightDefaultsToAssetScopeWithoutFinding(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:           "lonely-script",
			Type:         "script",
			Bytes:        45_000,
			PositionBand: "unknown",
			VisualRole:   "unknown",
		},
		nil,
		nil,
	)

	if draft.Scope != "asset" {
		t.Fatalf("expected asset scope, got %q", draft.Scope)
	}
}

func TestRecommendedFixForFindingUsesAstroSnippetForGallery(t *testing.T) {
	fix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{
			FrameworkHint: "astro",
			Evidence:      []string{"Se detectaron nodos astro-island."},
		},
	}, AnalysisFindingContext{
		ID: "repeated_gallery_overdelivery",
	})

	if fix == nil {
		t.Fatal("expected fix suggestion")
	}
	if strings.Contains(fix.OptimizedCode, `next/image`) {
		t.Fatalf("expected astro-specific code, got %q", fix.OptimizedCode)
	}
	if !strings.Contains(fix.OptimizedCode, `astro:assets`) {
		t.Fatalf("expected astro snippet, got %q", fix.OptimizedCode)
	}
	if !strings.Contains(fix.OptimizedCode, `"eager"`) || !strings.Contains(fix.OptimizedCode, `"lazy"`) {
		t.Fatalf("expected mixed eager/lazy guidance, got %q", fix.OptimizedCode)
	}
	if strings.Contains(fix.OptimizedCode, `index < 3`) {
		t.Fatalf("expected snippet to avoid hardcoded first row size, got %q", fix.OptimizedCode)
	}
	if !strings.Contains(fix.OptimizedCode, `firstRowCount`) {
		t.Fatalf("expected snippet to rely on firstRowCount, got %q", fix.OptimizedCode)
	}
}

func TestRecommendedFixForFindingUsesNeutralNextSnippetForGallery(t *testing.T) {
	fix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{
			FrameworkHint: "nextjs",
		},
	}, AnalysisFindingContext{
		ID: "repeated_gallery_overdelivery",
	})

	if fix == nil {
		t.Fatal("expected gallery fix")
	}
	if strings.Contains(fix.OptimizedCode, "CourseGrid") || strings.Contains(fix.OptimizedCode, "courses") {
		t.Fatalf("expected neutral grid naming, got %q", fix.OptimizedCode)
	}
	if !strings.Contains(fix.OptimizedCode, "CardGrid") || !strings.Contains(fix.OptimizedCode, "items") {
		t.Fatalf("expected generic grid snippet, got %q", fix.OptimizedCode)
	}
}

func TestAssetTitleAvoidsOversizedLabelForTinyImages(t *testing.T) {
	title := assetTitle(ResourceContext{
		ID:                    "avatar",
		Type:                  "image",
		Bytes:                 18_000,
		EstimatedSavingsBytes: 12_000,
		NaturalWidth:          500,
		NaturalHeight:         500,
	}, nil)

	if title == "Imagen sobredimensionada" {
		t.Fatalf("expected tiny image to avoid oversized label, got %q", title)
	}
}

func TestBuildRuleBasedAssetInsightUsesLowImpactCopyForSmallOversizedImage(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:                    "avatar",
			Type:                  "image",
			Bytes:                 18_000,
			EstimatedSavingsBytes: 9_000,
			NaturalWidth:          500,
			NaturalHeight:         500,
			PositionBand:          "below_fold",
			VisualRole:            "below_fold_media",
		},
		nil,
		nil,
	)

	if draft.Title != "Imagen sobredimensionada, pero de bajo impacto" {
		t.Fatalf("expected softer title for small image, got %q", draft.Title)
	}
	if !strings.Contains(strings.ToLower(draft.ShortProblem), "impacto total es limitado") {
		t.Fatalf("expected softer short problem, got %q", draft.ShortProblem)
	}
	if !strings.Contains(strings.ToLower(draft.RecommendedAction), "variante más pequeña") {
		t.Fatalf("expected recommendation to mention smaller variant, got %q", draft.RecommendedAction)
	}
}

func TestBuildRuleBasedAssetInsightUsesLogoSpecificCopy(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:                    "logo",
			URL:                   "https://example.com/images/logo-light.png",
			Type:                  "image",
			MIMEType:              "image/png",
			Bytes:                 38_394,
			EstimatedSavingsBytes: 23_036,
			NaturalWidth:          1570,
			NaturalHeight:         319,
			VisualRole:            "above_fold_media",
		},
		nil,
		nil,
	)

	if draft.Title != "Logo raster sobredimensionado" {
		t.Fatalf("expected logo-specific title, got %q", draft.Title)
	}
	if !strings.Contains(strings.ToLower(draft.ShortProblem), "logo") {
		t.Fatalf("expected logo-specific short problem, got %q", draft.ShortProblem)
	}
	if !strings.Contains(strings.ToLower(draft.RecommendedAction), "svg") {
		t.Fatalf("expected logo recommendation to mention svg, got %q", draft.RecommendedAction)
	}
}

func TestBuildRuleBasedAssetInsightDoesNotTreatGenericIconAsLogo(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:                    "feature-icon",
			URL:                   "https://example.com/images/feature-icon.png",
			Type:                  "image",
			MIMEType:              "image/png",
			Bytes:                 38_394,
			EstimatedSavingsBytes: 23_036,
			NaturalWidth:          512,
			NaturalHeight:         512,
			VisualRole:            "above_fold_media",
		},
		nil,
		nil,
	)

	if draft.Title == "Logo raster sobredimensionado" {
		t.Fatalf("expected generic icon to avoid logo-specific title, got %q", draft.Title)
	}
	if strings.Contains(strings.ToLower(draft.RecommendedAction), "svg") {
		t.Fatalf("expected generic icon to avoid logo-specific svg guidance, got %q", draft.RecommendedAction)
	}
}

func TestSummarizeReportProvidesFixesBeyondPrimaryAction(t *testing.T) {
	result, err := NewRuleBasedProvider().SummarizeReport(context.TODO(), ReportContext{
		SiteProfile: SiteProfileContext{
			FrameworkHint: "astro",
		},
		Analysis: AnalysisContext{
			Findings: []AnalysisFindingContext{
				{
					ID:                    "repeated_gallery_overdelivery",
					Category:              "media",
					Severity:              "medium",
					Confidence:            "high",
					Title:                 "Comprime la galería repetida del catálogo",
					Summary:               "La galería repetida suma demasiado peso.",
					EstimatedSavingsBytes: 900_000,
					RelatedResourceIDs:    []string{"card-1"},
				},
				{
					ID:                    "third_party_analytics_overhead",
					Category:              "third_party",
					Severity:              "medium",
					Confidence:            "high",
					Title:                 "Recorta la sobrecarga de analítica",
					Summary:               "La analítica añade ruido de red.",
					EstimatedSavingsBytes: 80_000,
					RelatedResourceIDs:    []string{"analytics"},
				},
				{
					ID:                    "font_stack_overweight",
					Category:              "fonts",
					Severity:              "medium",
					Confidence:            "medium",
					Title:                 "Recorta el coste tipográfico",
					Summary:               "Las fuentes pesan demasiado.",
					EstimatedSavingsBytes: 60_000,
					RelatedResourceIDs:    []string{"font-1"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected summarize error: %v", err)
	}
	if len(result.Insights.TopActions) != 3 {
		t.Fatalf("expected 3 top actions, got %d", len(result.Insights.TopActions))
	}
	if result.Insights.TopActions[1].RecommendedFix == nil {
		t.Fatal("expected secondary action to include a recommended fix")
	}
	if result.Insights.TopActions[2].RecommendedFix == nil {
		t.Fatal("expected tertiary action to include a recommended fix")
	}
}

func TestBuildExecutiveSummaryPrioritizesDominantImageFinding(t *testing.T) {
	summary := buildExecutiveSummary(ReportContext{
		Analysis: AnalysisContext{
			Findings: []AnalysisFindingContext{
				{
					ID:         "dominant_image_overdelivery",
					Severity:   "high",
					Confidence: "high",
				},
			},
		},
	})

	if !strings.Contains(strings.ToLower(summary), "un solo asset visual") {
		t.Fatalf("expected dominant image summary, got %q", summary)
	}
}

func TestPrioritizeFindingsPrefersPaymentOverGalleryWhenSeverityTies(t *testing.T) {
	sorted := prioritizeFindings([]AnalysisFindingContext{
		{
			ID:                    "repeated_gallery_overdelivery",
			Severity:              "medium",
			Confidence:            "high",
			EstimatedSavingsBytes: 900_000,
		},
		{
			ID:                    "third_party_payment_overhead",
			Severity:              "medium",
			Confidence:            "high",
			EstimatedSavingsBytes: 220_000,
		},
	})

	if len(sorted) < 2 {
		t.Fatalf("expected at least 2 findings, got %#v", sorted)
	}
	if sorted[0].ID != "third_party_payment_overhead" {
		t.Fatalf("expected payment finding to outrank gallery on editorial priority, got %#v", sorted)
	}
}

func TestBuildExecutiveSummaryMentionsNearThresholdCPUWhenLeadingFinding(t *testing.T) {
	summary := buildExecutiveSummary(ReportContext{
		Analysis: AnalysisContext{
			Findings: []AnalysisFindingContext{
				{
					ID:         "main_thread_cpu_pressure",
					Severity:   "low",
					Confidence: "low",
				},
			},
		},
	})

	if !strings.Contains(strings.ToLower(summary), "cerca del umbral") {
		t.Fatalf("expected summary to mention near-threshold cpu, got %q", summary)
	}
}

func TestBuildRuleBasedAssetInsightDoesNotAttachUnanchoredCPUActionToScript(t *testing.T) {
	asset := ResourceContext{
		ID:   "visible-script",
		Type: "script",
		URL:  "https://example.com/app.js",
	}
	draft := BuildRuleBasedAssetInsight(
		asset,
		[]AnalysisFindingContext{
			{
				ID:                 "main_thread_cpu_pressure",
				Category:           "cpu",
				Severity:           "medium",
				Confidence:         "high",
				Title:              "Reduce la presión real sobre la hebra principal",
				Summary:            "Hay Long Tasks de arranque.",
				RelatedResourceIDs: []string{"other-script"},
			},
		},
		[]TopAction{
			{
				ID:                 "act-1",
				RelatedFindingID:   "main_thread_cpu_pressure",
				RelatedResourceIDs: nil,
				Reason:             "Difiere el JS costoso.",
				Confidence:         "high",
				LikelyLCPImpact:    "medium",
				RecommendedFix: &RecommendedFix{
					Summary:       "Carga el script tras interacción.",
					OptimizedCode: "setTimeout(loadHeavyModule, 0);",
				},
			},
		},
	)

	if draft.RelatedFindingID != "" {
		t.Fatalf("expected script without exact anchor to avoid inherited CPU finding, got %q", draft.RelatedFindingID)
	}
	if draft.RelatedActionID != "" {
		t.Fatalf("expected script without exact anchor to avoid inherited CPU action, got %q", draft.RelatedActionID)
	}
	if draft.RecommendedFix != nil {
		t.Fatal("expected script without exact anchor to avoid inherited CPU fix")
	}
}

func TestBuildRuleBasedAssetInsightPrefersAnalyticsFindingForAnalyticsScript(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:               "posthog",
			Type:             "script",
			URL:              "https://us-assets.i.posthog.com/static/posthog-recorder.js",
			IsThirdPartyTool: true,
			ThirdPartyKind:   "analytics",
		},
		[]AnalysisFindingContext{
			{
				ID:                 "main_thread_cpu_pressure",
				Category:           "cpu",
				Severity:           "medium",
				Confidence:         "high",
				Title:              "Reduce la presión real sobre la hebra principal",
				Summary:            "Hay Long Tasks de arranque.",
				RelatedResourceIDs: []string{"posthog"},
			},
			{
				ID:                 "third_party_analytics_overhead",
				Category:           "third_party",
				Severity:           "medium",
				Confidence:         "high",
				Title:              "Recorta la sobrecarga de analítica",
				Summary:            "La capa de analítica añade ruido de red.",
				RelatedResourceIDs: []string{"posthog"},
			},
		},
		[]TopAction{
			{
				ID:                 "act-cpu",
				RelatedFindingID:   "main_thread_cpu_pressure",
				RelatedResourceIDs: []string{"posthog"},
				Reason:             "Difiere el JS costoso.",
				Confidence:         "high",
				LikelyLCPImpact:    "medium",
			},
			{
				ID:                 "act-analytics",
				RelatedFindingID:   "third_party_analytics_overhead",
				RelatedResourceIDs: []string{"posthog"},
				Reason:             "Retrasa la analítica hasta interacción.",
				Confidence:         "high",
				LikelyLCPImpact:    "low",
			},
		},
	)

	if draft.RelatedFindingID != "third_party_analytics_overhead" {
		t.Fatalf("expected analytics finding to win for analytics script, got %q", draft.RelatedFindingID)
	}
	if draft.RelatedActionID != "act-analytics" {
		t.Fatalf("expected analytics action to match the chosen finding, got %q", draft.RelatedActionID)
	}
	if draft.Title != "Recorta la sobrecarga de analítica" {
		t.Fatalf("expected analytics title, got %q", draft.Title)
	}
}

func TestBuildRuleBasedAssetInsightPrefersDominantImageFindingOverGallery(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:         "blog-jpeg",
			Type:       "image",
			VisualRole: "repeated_card_media",
		},
		[]AnalysisFindingContext{
			{
				ID:                 "repeated_gallery_overdelivery",
				Category:           "media",
				Severity:           "medium",
				Confidence:         "high",
				Title:              "Reduce el peso de las miniaturas del blog",
				Summary:            "Las miniaturas del blog suman demasiado peso.",
				RelatedResourceIDs: []string{"blog-jpeg", "blog-avif"},
			},
			{
				ID:                 "dominant_image_overdelivery",
				Category:           "media",
				Severity:           "high",
				Confidence:         "high",
				Title:              "Corrige una imagen dominante sobredimensionada",
				Summary:            "Una sola imagen pesa demasiado.",
				RelatedResourceIDs: []string{"blog-jpeg"},
			},
		},
		[]TopAction{
			{
				ID:                 "act-gallery",
				RelatedFindingID:   "repeated_gallery_overdelivery",
				RelatedResourceIDs: []string{"blog-jpeg"},
			},
			{
				ID:                 "act-dominant",
				RelatedFindingID:   "dominant_image_overdelivery",
				RelatedResourceIDs: []string{"blog-jpeg"},
			},
		},
	)

	if draft.RelatedFindingID != "dominant_image_overdelivery" {
		t.Fatalf("expected dominant image finding to win over group finding, got %q", draft.RelatedFindingID)
	}
	if draft.RelatedActionID != "act-dominant" {
		t.Fatalf("expected action to follow dominant image finding, got %q", draft.RelatedActionID)
	}
}

func TestRecommendedFixForFindingProvidesSocialEmbedSnippet(t *testing.T) {
	fix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{
			FrameworkHint: "nextjs",
		},
	}, AnalysisFindingContext{
		ID: "third_party_social_overhead",
	})

	if fix == nil {
		t.Fatal("expected social finding fix")
	}
	if !strings.Contains(fix.OptimizedCode, "Cargar publicación") || !strings.Contains(fix.OptimizedCode, "social.example.com/embed.js") {
		t.Fatalf("expected social embed snippet, got %q", fix.OptimizedCode)
	}
}

func TestRecommendedFixForFindingAdaptsGalleryFixWhenLazyAlreadyPresent(t *testing.T) {
	fix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{
			FrameworkHint: "astro",
		},
	}, AnalysisFindingContext{
		ID:       "repeated_gallery_overdelivery",
		Evidence: []string{`Lazy loading ya presente en la mayoría: 14 de 16.`},
	})

	if fix == nil {
		t.Fatal("expected gallery fix")
	}
	if strings.Contains(strings.ToLower(strings.Join(fix.Changes, " ")), "lazy loading para el resto") {
		t.Fatalf("expected lazy-majority fix to stop prescribing lazy loading, got %#v", fix.Changes)
	}
	if strings.Contains(fix.OptimizedCode, `loading={index < firstRowCount ? "eager" : "lazy"}`) {
		t.Fatalf("expected lazy-majority snippet to focus on sizing/format, got %q", fix.OptimizedCode)
	}
}

func TestRecommendedFixForFindingProvidesPaymentAndVideoSnippets(t *testing.T) {
	paymentFix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{FrameworkHint: "nextjs"},
	}, AnalysisFindingContext{
		ID: "third_party_payment_overhead",
	})
	if paymentFix == nil {
		t.Fatal("expected payment fix")
	}
	if !strings.Contains(paymentFix.OptimizedCode, "Ver entradas") || !strings.Contains(paymentFix.OptimizedCode, "iframe") {
		t.Fatalf("expected payment snippet, got %q", paymentFix.OptimizedCode)
	}

	videoFix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{FrameworkHint: "nextjs"},
	}, AnalysisFindingContext{
		ID: "third_party_video_overhead",
	})
	if videoFix == nil {
		t.Fatal("expected video fix")
	}
	if !strings.Contains(videoFix.OptimizedCode, "Reproducir video") || !strings.Contains(videoFix.OptimizedCode, "youtube.com/embed") {
		t.Fatalf("expected video snippet, got %q", videoFix.OptimizedCode)
	}
}

func TestRecommendedFixForFindingUsesLazySnippetForBelowFoldResponsiveImage(t *testing.T) {
	fix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{FrameworkHint: "astro"},
	}, AnalysisFindingContext{
		ID: "responsive_image_overdelivery",
		Evidence: []string{
			"Posición visual: below fold.",
		},
	})

	if fix == nil {
		t.Fatal("expected responsive image fix")
	}
	if !strings.Contains(fix.OptimizedCode, `loading="lazy"`) {
		t.Fatalf("expected below-fold responsive snippet to stay lazy, got %q", fix.OptimizedCode)
	}
}

func TestRecommendedFixForFindingProvidesAdsAndLegacyFormatSnippets(t *testing.T) {
	adsFix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{FrameworkHint: "generic"},
	}, AnalysisFindingContext{
		ID: "third_party_ads_overhead",
	})
	if adsFix == nil || !strings.Contains(strings.ToLower(adsFix.OptimizedCode), "googletag") {
		t.Fatalf("expected ads fix snippet, got %#v", adsFix)
	}

	imageFix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{FrameworkHint: "astro"},
	}, AnalysisFindingContext{
		ID: "legacy_image_format_overhead",
	})
	if imageFix == nil || !strings.Contains(imageFix.OptimizedCode, "Picture") {
		t.Fatalf("expected legacy image fix, got %#v", imageFix)
	}

	fontFix := recommendedFixForFinding(ReportContext{}, AnalysisFindingContext{
		ID: "legacy_font_format_overhead",
	})
	if fontFix == nil || !strings.Contains(strings.ToLower(fontFix.OptimizedCode), "woff2") {
		t.Fatalf("expected legacy font fix, got %#v", fontFix)
	}
}

func TestRecommendedFixForFindingUsesIconFontSpecificGuidance(t *testing.T) {
	fix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{FrameworkHint: "nextjs"},
		TopResources: []ResourceContext{
			{
				ID:   "fa-solid",
				URL:  "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/webfonts/fa-solid-900.woff2",
				Type: "font",
			},
		},
	}, AnalysisFindingContext{
		ID:                 "font_stack_overweight",
		RelatedResourceIDs: []string{"fa-solid"},
	})

	if fix == nil {
		t.Fatal("expected icon-font fix")
	}
	if !strings.Contains(strings.ToLower(fix.Summary), "svgs individuales") {
		t.Fatalf("expected icon-font summary, got %q", fix.Summary)
	}
	if !strings.Contains(strings.ToLower(fix.OptimizedCode), "lucide-react") {
		t.Fatalf("expected icon-font snippet, got %q", fix.OptimizedCode)
	}
}

func TestRecommendedFixForFindingUsesIconFontGuidanceFromFindingCopyEvenWithoutTopResource(t *testing.T) {
	fix := recommendedFixForFinding(ReportContext{
		SiteProfile: SiteProfileContext{FrameworkHint: "nextjs"},
	}, AnalysisFindingContext{
		ID:      "font_stack_overweight",
		Title:   "Recorta el coste de la fuente de iconos",
		Summary: "Una icon font genérica pesa más de lo razonable.",
		Evidence: []string{
			"Se detectó una fuente de iconos pesada entre los recursos tipográficos.",
		},
	})

	if fix == nil {
		t.Fatal("expected icon-font fix without relying on top resources")
	}
	if !strings.Contains(strings.ToLower(fix.Summary), "svgs individuales") {
		t.Fatalf("expected icon-font summary, got %q", fix.Summary)
	}
}

func TestBuildRuleBasedAssetInsightUsesIconFontCopy(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:   "fa-solid",
			URL:  "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.4.0/webfonts/fa-solid-900.woff2",
			Type: "font",
		},
		[]AnalysisFindingContext{
			{
				ID:                 "font_stack_overweight",
				Category:           "fonts",
				Severity:           "medium",
				Confidence:         "medium",
				Title:              "Recorta el coste de la fuente de iconos",
				Summary:            "La icon font pesa más de lo razonable.",
				RelatedResourceIDs: []string{"fa-solid"},
			},
		},
		nil,
	)

	if draft.Title != "Recorta el coste de la fuente de iconos" {
		t.Fatalf("expected icon-font title, got %q", draft.Title)
	}
	if !strings.Contains(strings.ToLower(draft.ShortProblem), "iconos") {
		t.Fatalf("expected icon-font short problem, got %q", draft.ShortProblem)
	}
	if !strings.Contains(strings.ToLower(draft.RecommendedAction), "svg") {
		t.Fatalf("expected icon-font recommendation, got %q", draft.RecommendedAction)
	}
}

func TestBuildExecutiveSummaryExplainsTextualFirstRender(t *testing.T) {
	summary := buildExecutiveSummary(ReportContext{
		Summary: SummaryContext{
			ThirdPartyBytes: 60_000,
		},
		TotalBytesTransferred: 200_000,
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				AboveFoldVisualBytes: 0,
				RenderCriticalBytes:  180_000,
			},
			Findings: []AnalysisFindingContext{
				{
					ID:         "repeated_gallery_overdelivery",
					Severity:   "medium",
					Confidence: "high",
				},
				{
					ID:         "render_lcp_dom_node",
					Severity:   "medium",
					Confidence: "medium",
				},
			},
			ResourceGroups: []ResourceGroupContext{
				{
					ID:           "group-speakers",
					Kind:         "repeated_gallery",
					Label:        "Fotos de speakers",
					TotalBytes:   600_000,
					PositionBand: "below_fold",
				},
			},
		},
		Performance: PerformanceContext{
			LCPMS: 900,
		},
	})

	if !strings.Contains(strings.ToLower(summary), "texto, fuentes y css") {
		t.Fatalf("expected textual-first-render framing, got %q", summary)
	}
}

func TestBuildExecutiveSummaryAndPitchAlignForAdsLead(t *testing.T) {
	report := ReportContext{
		Performance: PerformanceContext{
			RenderMetricsComplete: true,
			LCPMS:                 1680,
		},
		Analysis: AnalysisContext{
			Findings: []AnalysisFindingContext{
				{
					ID:         "third_party_ads_overhead",
					Severity:   "medium",
					Confidence: "high",
				},
			},
		},
	}

	summary := strings.ToLower(buildExecutiveSummary(report))
	pitch := strings.ToLower(buildPitchLine(report))
	if !strings.Contains(summary, "publicitario") {
		t.Fatalf("expected ads summary, got %q", summary)
	}
	if !strings.Contains(pitch, "publicitario") {
		t.Fatalf("expected ads pitch, got %q", pitch)
	}
}

func TestBuildExecutiveSummaryTreatsIncompleteRenderMetricsCautiously(t *testing.T) {
	summary := strings.ToLower(buildExecutiveSummary(ReportContext{
		Performance: PerformanceContext{
			RenderMetricsComplete: false,
		},
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				RenderCriticalBytes: 180_000,
			},
		},
	}))

	if !strings.Contains(summary, "no captur") && !strings.Contains(summary, "métricas de render") {
		t.Fatalf("expected incomplete render caution, got %q", summary)
	}
}

func TestBuildRuleBasedAssetInsightDoesNotCrossMatchRepeatedGalleryAssets(t *testing.T) {
	draft := BuildRuleBasedAssetInsight(
		ResourceContext{
			ID:         "sponsor-logo",
			Type:       "image",
			URL:        "https://example.com/sponsors/acme.webp",
			VisualRole: "repeated_card_media",
		},
		[]AnalysisFindingContext{
			{
				ID:                 "repeated_gallery_overdelivery",
				Category:           "media",
				Severity:           "medium",
				Confidence:         "high",
				Title:              "Reduce el peso de fotos de speakers",
				Summary:            "Las fotos de speakers suman demasiado peso.",
				RelatedResourceIDs: []string{"speaker-1", "speaker-2"},
			},
		},
		nil,
	)

	if draft.RelatedFindingID != "" {
		t.Fatalf("expected cross-gallery asset to avoid inherited finding, got %q", draft.RelatedFindingID)
	}
}

func TestBuildExecutiveSummaryKeepsTextualLeadWhenRenderLCPSitsOnDOMNode(t *testing.T) {
	summary := buildExecutiveSummary(ReportContext{
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				AboveFoldVisualBytes: 0,
				RenderCriticalBytes:  220_000,
			},
			Findings: []AnalysisFindingContext{
				{
					ID:         "render_lcp_dom_node",
					Severity:   "medium",
					Confidence: "medium",
				},
			},
		},
	})

	if !strings.Contains(strings.ToLower(summary), "texto, fuentes y css") {
		t.Fatalf("expected textual lead for dom-node LCP summary, got %q", summary)
	}
}

func TestBuildPitchLineKeepsTextualLeadWhenRenderLCPSitsOnDOMNode(t *testing.T) {
	pitch := buildPitchLine(ReportContext{
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				AboveFoldVisualBytes: 0,
				RenderCriticalBytes:  220_000,
			},
			Findings: []AnalysisFindingContext{
				{
					ID:         "render_lcp_dom_node",
					Severity:   "medium",
					Confidence: "medium",
				},
			},
		},
	})

	if !strings.Contains(strings.ToLower(pitch), "texto, fuentes y css") {
		t.Fatalf("expected textual lead for dom-node LCP pitch, got %q", pitch)
	}
}

func TestBuildExecutiveSummaryPrioritizesDominantThirdPartyEditorially(t *testing.T) {
	summary := buildExecutiveSummary(ReportContext{
		TotalBytesTransferred: 1_000_000,
		Summary: SummaryContext{
			ThirdPartyBytes: 650_000,
		},
		Analysis: AnalysisContext{
			Summary: AnalysisSummaryContext{
				RenderCriticalBytes: 120_000,
			},
			Findings: []AnalysisFindingContext{
				{
					ID:         "repeated_gallery_overdelivery",
					Severity:   "medium",
					Confidence: "high",
				},
			},
			ResourceGroups: []ResourceGroupContext{
				{
					ID:           "group-gallery",
					Kind:         "repeated_gallery",
					Label:        "Fotos de speakers",
					TotalBytes:   500_000,
					PositionBand: "below_fold",
				},
				{
					ID:           "group-payment",
					Kind:         "third_party_cluster",
					Label:        "Cluster de pagos",
					TotalBytes:   220_000,
					PositionBand: "unknown",
				},
			},
		},
		Performance: PerformanceContext{
			LCPMS: 900,
		},
	})

	if !strings.Contains(strings.ToLower(summary), "terceros") && !strings.Contains(strings.ToLower(summary), "cluster de pagos") {
		t.Fatalf("expected third-party editorial lead, got %q", summary)
	}
}
