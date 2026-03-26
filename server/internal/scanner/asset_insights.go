package scanner

import (
	"strings"

	"github.com/tronchos/wattless/server/internal/insights"
)

func attachAssetInsights(
	vampires []ResourceSummary,
	analysis Analysis,
	actions []insights.TopAction,
	drafts []insights.AssetInsightDraft,
) []ResourceSummary {
	if len(vampires) == 0 {
		return vampires
	}

	output := append([]ResourceSummary(nil), vampires...)
	findings := makeInsightAnalysis(analysis).Findings
	draftsByResourceID := sanitizeAssetInsightDrafts(drafts, analysis.Findings, actions, vampires)

	for index := range output {
		fallback := insights.BuildRuleBasedAssetInsight(makeInsightResource(output[index]), findings, actions)
		draft, ok := draftsByResourceID[output[index].ID]
		output[index].AssetInsight = buildAssetInsight(fallback, ok, draft)
	}

	return output
}

func sanitizeAssetInsightDrafts(
	drafts []insights.AssetInsightDraft,
	findings []AnalysisFinding,
	actions []insights.TopAction,
	vampires []ResourceSummary,
) map[string]insights.AssetInsightDraft {
	allowedResources := make(map[string]struct{}, len(vampires))
	for _, vampire := range vampires {
		allowedResources[vampire.ID] = struct{}{}
	}

	allowedFindings := make(map[string]struct{}, len(findings))
	for _, finding := range findings {
		allowedFindings[finding.ID] = struct{}{}
	}

	allowedActions := make(map[string]struct{}, len(actions))
	for _, action := range actions {
		allowedActions[action.ID] = struct{}{}
	}

	sanitized := make(map[string]insights.AssetInsightDraft, len(drafts))
	for _, draft := range drafts {
		resourceID := strings.TrimSpace(draft.ResourceID)
		if resourceID == "" {
			continue
		}
		if _, ok := allowedResources[resourceID]; !ok {
			continue
		}
		if _, seen := sanitized[resourceID]; seen {
			continue
		}

		draft.ResourceID = resourceID
		draft.Title = strings.TrimSpace(draft.Title)
		draft.ShortProblem = strings.TrimSpace(draft.ShortProblem)
		draft.WhyItMatters = strings.TrimSpace(draft.WhyItMatters)
		draft.RecommendedAction = strings.TrimSpace(draft.RecommendedAction)
		draft.Confidence = normalizeConfidence(draft.Confidence)
		draft.LikelyLCPImpact = normalizeImpact(draft.LikelyLCPImpact)
		draft.Scope = normalizeScope(draft.Scope)
		if strings.TrimSpace(draft.Source) == "" {
			draft.Source = "gemini"
		} else {
			draft.Source = normalizeSource(draft.Source)
		}
		draft.Evidence = trimNonEmptyEvidence(draft.Evidence, 3)

		if draft.RelatedFindingID != "" {
			if _, ok := allowedFindings[draft.RelatedFindingID]; !ok {
				draft.RelatedFindingID = ""
			}
		}
		if draft.RelatedActionID != "" {
			if _, ok := allowedActions[draft.RelatedActionID]; !ok {
				draft.RelatedActionID = ""
			}
		}
		if !hasMaterialFix(draft.RecommendedFix) {
			draft.RecommendedFix = nil
		}
		if draft.Title == "" && draft.ShortProblem == "" && draft.WhyItMatters == "" && draft.RecommendedAction == "" {
			continue
		}

		sanitized[resourceID] = draft
	}

	return sanitized
}

func buildAssetInsight(
	fallback insights.AssetInsightDraft,
	hasDraft bool,
	draft insights.AssetInsightDraft,
) AssetInsight {
	if !hasDraft || draft.Source == "rule_based" {
		return assetInsightFromDraft(fallback)
	}

	usedFallback := false
	insight := AssetInsight{
		Title:             pickAssetField(draft.Title, fallback.Title, &usedFallback),
		ShortProblem:      pickAssetField(draft.ShortProblem, fallback.ShortProblem, &usedFallback),
		WhyItMatters:      pickAssetField(draft.WhyItMatters, fallback.WhyItMatters, &usedFallback),
		RecommendedAction: pickAssetField(draft.RecommendedAction, fallback.RecommendedAction, &usedFallback),
		Confidence:        pickAssetField(draft.Confidence, fallback.Confidence, &usedFallback),
		LikelyLCPImpact:   pickAssetField(draft.LikelyLCPImpact, fallback.LikelyLCPImpact, &usedFallback),
		RelatedFindingID:  pickAssetField(draft.RelatedFindingID, fallback.RelatedFindingID, &usedFallback),
		RelatedActionID:   pickAssetField(draft.RelatedActionID, fallback.RelatedActionID, &usedFallback),
		Scope:             pickAssetField(draft.Scope, fallback.Scope, &usedFallback),
		Evidence:          pickAssetEvidence(draft.Evidence, fallback.Evidence, &usedFallback),
		RecommendedFix:    pickAssetFix(draft.RecommendedFix, fallback.RecommendedFix, &usedFallback),
	}
	if usedFallback {
		insight.Source = "hybrid"
	} else {
		insight.Source = normalizeSource(draft.Source)
	}
	return insight
}

func assetInsightFromDraft(draft insights.AssetInsightDraft) AssetInsight {
	return AssetInsight{
		Source:            normalizeSource(draft.Source),
		Scope:             normalizeScope(draft.Scope),
		Title:             strings.TrimSpace(draft.Title),
		ShortProblem:      strings.TrimSpace(draft.ShortProblem),
		WhyItMatters:      strings.TrimSpace(draft.WhyItMatters),
		RecommendedAction: strings.TrimSpace(draft.RecommendedAction),
		Confidence:        normalizeConfidence(draft.Confidence),
		LikelyLCPImpact:   normalizeImpact(draft.LikelyLCPImpact),
		RelatedFindingID:  strings.TrimSpace(draft.RelatedFindingID),
		RelatedActionID:   strings.TrimSpace(draft.RelatedActionID),
		Evidence:          trimNonEmptyEvidence(draft.Evidence, 3),
		RecommendedFix:    draft.RecommendedFix,
	}
}

func makeInsightResource(resource ResourceSummary) insights.ResourceContext {
	return insights.ResourceContext{
		ID:                    resource.ID,
		URL:                   resource.URL,
		Type:                  resource.Type,
		MIMEType:              resource.MIMEType,
		Bytes:                 resource.Bytes,
		StatusCode:            resource.StatusCode,
		Failed:                resource.Failed,
		FailureReason:         resource.FailureReason,
		TransferShare:         resource.TransferShare,
		EstimatedSavingsBytes: resource.EstimatedSavingsBytes,
		PositionBand:          resource.PositionBand,
		VisualRole:            resource.VisualRole,
		DOMTag:                resource.DOMTag,
		LoadingAttr:           resource.LoadingAttr,
		FetchPriority:         resource.FetchPriority,
		ResponsiveImage:       resource.ResponsiveImage,
		IsThirdPartyTool:      resource.IsThirdPartyTool,
		ThirdPartyKind:        resource.ThirdPartyKind,
	}
}

func pickAssetField(primary, fallback string, usedFallback *bool) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		*usedFallback = true
	}
	return fallback
}

func pickAssetEvidence(primary, fallback []string, usedFallback *bool) []string {
	primary = trimNonEmptyEvidence(primary, 3)
	if len(primary) > 0 {
		return primary
	}
	*usedFallback = true
	return trimNonEmptyEvidence(fallback, 3)
}

func pickAssetFix(primary, fallback *FixSuggestion, usedFallback *bool) *FixSuggestion {
	if hasMaterialFix(primary) {
		return primary
	}
	if hasMaterialFix(fallback) {
		*usedFallback = true
		return fallback
	}
	return nil
}

func trimNonEmptyEvidence(values []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	output := make([]string, 0, limit)
	seen := make(map[string]struct{}, limit)
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		output = append(output, value)
		if len(output) == limit {
			break
		}
	}
	return output
}

func normalizeConfidence(value string) string {
	switch value {
	case "high", "medium", "low":
		return value
	default:
		return "low"
	}
}

func normalizeImpact(value string) string {
	switch value {
	case "high", "medium", "low":
		return value
	default:
		return "low"
	}
}

func normalizeScope(value string) string {
	switch value {
	case "asset", "group", "global":
		return value
	default:
		return "asset"
	}
}

func normalizeSource(value string) string {
	switch value {
	case "gemini", "rule_based", "hybrid":
		return value
	default:
		return "rule_based"
	}
}

func hasMaterialFix(fix *FixSuggestion) bool {
	if fix == nil {
		return false
	}
	return strings.TrimSpace(fix.Summary) != "" ||
		strings.TrimSpace(fix.OptimizedCode) != "" ||
		strings.TrimSpace(fix.ExpectedImpact) != "" ||
		len(fix.Changes) > 0
}
