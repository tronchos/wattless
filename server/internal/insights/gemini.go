package insights

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GeminiProvider struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewGeminiProvider(apiKey, model string, timeout time.Duration) GeminiProvider {
	return GeminiProvider{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (GeminiProvider) Name() string {
	return "gemini"
}

func (provider GeminiProvider) SuggestResource(resource ResourceContext) string {
	return NewRuleBasedProvider().SuggestResource(resource)
}

func (provider GeminiProvider) SummarizeReport(ctx context.Context, report ReportContext) (ProviderResult, error) {
	prompt := fmt.Sprintf(`Eres un arquitecto frontend especializado en rendimiento web y sostenibilidad.
Devuelve JSON estricto sin markdown con esta forma:
{
  "executive_summary": string,
  "pitch_line": string,
  "top_actions": [
    {
      "id": string,
      "related_finding_id": string,
      "title": string,
      "reason": string,
      "confidence": "high"|"medium"|"low",
      "evidence": [string],
      "estimated_savings_bytes": number,
      "likely_lcp_impact": "high"|"medium"|"low",
      "related_resource_ids": [string],
      "recommended_fix": {
        "summary": "Explicación breve de la corrección",
        "optimized_code": "Código de ejemplo adaptado al framework_hint del sitio",
        "changes": ["Cambio 1", "Cambio 2"],
        "expected_impact": "Impacto esperado"
      }
    }
  ],
  "asset_insights": [
    {
      "resource_id": string,
      "title": string,
      "short_problem": string,
      "why_it_matters": string,
      "recommended_action": string,
      "confidence": "high"|"medium"|"low",
      "likely_lcp_impact": "high"|"medium"|"low",
      "related_finding_id": string,
      "related_action_id": string,
      "scope": "asset"|"group"|"global",
      "evidence": [string],
      "recommended_fix": {
        "summary": "Explicación breve de la corrección",
        "optimized_code": "Código de ejemplo adaptado al framework_hint del sitio",
        "changes": ["Cambio 1", "Cambio 2"],
        "expected_impact": "Impacto esperado"
      }
    }
  ]
}

Reglas:
- Escribe en español.
- No inventes métricas fuera del contexto dado.
- Máximo 3 acciones.
- Cada acción debe referenciar un related_finding_id existente y al menos un related_resource_ids existente.
- Los top_resources del contexto son también los focus_assets válidos para asset_insights.
- asset_insights solo puede usar resource_id presentes en top_resources.
- Máximo 1 asset_insight por resource_id.
- Máximo 3 evidencias por asset.
- short_problem, why_it_matters y recommended_action deben ser frases cortas.
- Si el fix es demasiado genérico o no es seguro, omite recommended_fix en el asset.
- Prioriza findings, no bytes crudos aislados.
- No llames hero image a un recurso salvo que su visual_role sea hero_media o lcp_candidate.
- No digas below the fold salvo que la evidencia de posición lo soporte.
- Distingue claramente entre carga inicial y below-the-fold.
- Si el LCP observado corresponde a un nodo del DOM sin asset asociado, habla de CSS, tipografía o CPU antes que de imágenes.
- Usa el campo confidence para no sobreafirmar.
- No interpretes script_resource_duration_ms como bloqueo real; usa long_tasks_total_ms para hablar de presión de CPU.
- El campo 'recommended_fix' debe incluirse obligatoriamente en al menos la primera top action (el cuello de botella crítico).
- Usa report.site_profile.framework_hint para elegir el estilo del snippet:
  - nextjs => React/Next
  - astro => Astro o HTML compatible con Astro
  - generic/unknown => HTML o JS vanilla
- En 'optimized_code', produce un snippet nativo de código ilustrando la solución sin bloques markdown y asumiendo que el asset problemático se usará ahí. No uses componentes de Next si framework_hint no es nextjs.

Contexto:
%s`, mustJSON(report))

	var payload struct {
		ExecutiveSummary string              `json:"executive_summary"`
		PitchLine        string              `json:"pitch_line"`
		TopActions       []TopAction         `json:"top_actions"`
		AssetInsights    []AssetInsightDraft `json:"asset_insights"`
	}

	if err := provider.generateJSON(ctx, prompt, &payload); err != nil {
		return ProviderResult{}, err
	}

	return ProviderResult{
		Insights: ScanInsights{
			Provider:         provider.Name(),
			ExecutiveSummary: strings.TrimSpace(payload.ExecutiveSummary),
			PitchLine:        strings.TrimSpace(payload.PitchLine),
			TopActions:       payload.TopActions,
		},
		AssetInsights: payload.AssetInsights,
	}, nil
}

func (provider GeminiProvider) generateJSON(ctx context.Context, prompt string, target any) error {
	if provider.apiKey == "" {
		return fmt.Errorf("missing Gemini API key")
	}

	requestBody := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":      0.2,
			"responseMimeType": "application/json",
		},
	}

	rawBody, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", provider.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", provider.apiKey)

	resp, err := provider.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("gemini request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var response struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return err
	}
	if len(response.Candidates) == 0 || len(response.Candidates[0].Content.Parts) == 0 {
		return fmt.Errorf("gemini returned no candidates")
	}

	text := extractJSONPayload(response.Candidates[0].Content.Parts[0].Text)
	if err := json.Unmarshal([]byte(text), target); err != nil {
		return fmt.Errorf("invalid gemini JSON payload: %w", err)
	}
	return nil
}

func extractJSONPayload(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "```json")
	value = strings.TrimPrefix(value, "```")
	value = strings.TrimSuffix(value, "```")
	value = strings.TrimSpace(value)

	start := strings.Index(value, "{")
	end := strings.LastIndex(value, "}")
	if start >= 0 && end > start {
		return value[start : end+1]
	}
	return value
}

func mustJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
