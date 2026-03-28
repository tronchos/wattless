package insights

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestGeminiGenerateJSONParsesCodeFencedPayload(t *testing.T) {
	responseBody, err := json.Marshal(map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"parts": []map[string]string{
						{"text": "```json\n{\"value\":\"ok\"}\n```"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected response body to marshal: %v", err)
	}

	provider := GeminiProvider{
		apiKey: "test-key",
		model:  "gemini-test",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(responseBody))),
				}, nil
			}),
		},
	}

	var payload struct {
		Value string `json:"value"`
	}
	if err := provider.generateJSON(context.Background(), "prompt", &payload); err != nil {
		t.Fatalf("expected code-fenced JSON to parse, got %v", err)
	}
	if payload.Value != "ok" {
		t.Fatalf("expected parsed payload value, got %q", payload.Value)
	}
}

func TestGeminiGenerateJSONRejectsOversizedResponses(t *testing.T) {
	innerJSON := `{"value":"` + strings.Repeat("a", 2<<20) + `"}`
	responseBody, err := json.Marshal(map[string]any{
		"candidates": []map[string]any{
			{
				"content": map[string]any{
					"parts": []map[string]string{
						{"text": innerJSON},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("expected response body to marshal: %v", err)
	}

	provider := GeminiProvider{
		apiKey: "test-key",
		model:  "gemini-test",
		httpClient: &http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader(string(responseBody))),
				}, nil
			}),
		},
	}

	var payload struct {
		Value string `json:"value"`
	}
	if err := provider.generateJSON(context.Background(), "prompt", &payload); err == nil {
		t.Fatal("expected oversized response to fail after hitting the read limit")
	}
}
