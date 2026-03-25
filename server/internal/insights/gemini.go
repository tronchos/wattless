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

func (provider GeminiProvider) SummarizeReport(ctx context.Context, report ReportContext) (ScanInsights, error) {
	prompt := fmt.Sprintf(`Eres un arquitecto frontend especializado en rendimiento web y sostenibilidad.
Devuelve JSON estricto sin markdown con esta forma:
{
  "executive_summary": string,
  "pitch_line": string,
  "top_actions": [
    {
      "id": string,
      "title": string,
      "reason": string,
      "estimated_savings_bytes": number,
      "likely_lcp_impact": "high"|"medium"|"low",
      "related_resource_id": string
    }
  ]
}

Reglas:
- Escribe en español.
- No inventes métricas fuera del contexto dado.
- Máximo 3 acciones.
- Cada acción debe referenciar un related_resource_id existente.
- Prioriza recursos con mayor estimated_savings_bytes y relación con LCP o terceros.

Contexto:
%s`, mustJSON(report))

	var payload struct {
		ExecutiveSummary string      `json:"executive_summary"`
		PitchLine        string      `json:"pitch_line"`
		TopActions       []TopAction `json:"top_actions"`
	}

	if err := provider.generateJSON(ctx, prompt, &payload); err != nil {
		return ScanInsights{}, err
	}

	return ScanInsights{
		Provider:         provider.Name(),
		ExecutiveSummary: strings.TrimSpace(payload.ExecutiveSummary),
		PitchLine:        strings.TrimSpace(payload.PitchLine),
		TopActions:       payload.TopActions,
	}, nil
}

func (provider GeminiProvider) RefactorCode(ctx context.Context, request RefactorRequest) (RefactorResult, error) {
	prompt := fmt.Sprintf(`Actúa como revisor senior de React/Next.js.
Devuelve JSON estricto sin markdown con esta forma:
{
  "summary": string,
  "optimized_code": string,
  "changes": [string],
  "expected_impact": string
}

Reglas:
- Escribe en español.
- Si el framework es "next", prioriza next/image, Script con strategy apropiada y minimizar JS crítico.
- Mantén el código ejecutable.
- No uses bloques markdown.
- Conserva el objetivo original del snippet.
- Si falta contexto, produce una mejora razonable y explícita.

Solicitud:
%s`, mustJSON(request))

	var payload struct {
		Summary        string   `json:"summary"`
		OptimizedCode  string   `json:"optimized_code"`
		Changes        []string `json:"changes"`
		ExpectedImpact string   `json:"expected_impact"`
	}

	if err := provider.generateJSON(ctx, prompt, &payload); err != nil {
		return RefactorResult{}, err
	}

	return RefactorResult{
		Provider:       provider.Name(),
		Summary:        strings.TrimSpace(payload.Summary),
		OptimizedCode:  strings.TrimSpace(payload.OptimizedCode),
		Changes:        payload.Changes,
		ExpectedImpact: strings.TrimSpace(payload.ExpectedImpact),
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

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", provider.model, provider.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := provider.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
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
