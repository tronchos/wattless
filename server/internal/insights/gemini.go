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
        "optimized_code": "Código React/Next de ejemplo reemplazando el asset infractor",
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
- Prioriza findings, no bytes crudos aislados.
- No llames hero image a un recurso salvo que su visual_role sea hero_media o lcp_candidate.
- Distingue claramente entre carga inicial y below-the-fold.
- Usa el campo confidence para no sobreafirmar.
- No interpretes script_resource_duration_ms como bloqueo real; usa long_tasks_total_ms para hablar de presión de CPU.
- El campo 'recommended_fix' debe incluirse obligatoriamente en al menos la primera top action (el cuello de botella crítico).
- En 'optimized_code', produce un snippet nativo de código (preferible React/NextJS) ilustrando la solución sin bloques markdown y asumiendo que el asset problemático se usará ahí (ej: si falla img.png, escribe <Image src="img.png"... />). Mantenlo limpio y profesional.

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
