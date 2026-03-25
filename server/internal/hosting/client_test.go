package hosting

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestCheckReturnsGreenResult(t *testing.T) {
	client := NewClient("https://example.test/api", time.Second)
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"hosted_by":"Cloudflare","green":true}`)),
			Header:     make(http.Header),
		}, nil
	})

	result, err := client.Check(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !result.IsGreen {
		t.Fatalf("expected green result")
	}
	if result.Verdict != VerdictGreen {
		t.Fatalf("expected green verdict, got %s", result.Verdict)
	}
}

func TestCheckFallsBackToUnknownOnStatusError(t *testing.T) {
	client := NewClient("https://example.test/api", time.Second)
	client.httpClient.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadGateway,
			Body:       io.NopCloser(strings.NewReader(`{}`)),
			Header:     make(http.Header),
		}, nil
	})

	result, err := client.Check(context.Background(), "example.com")
	if err == nil {
		t.Fatal("expected error")
	}
	if result.Verdict != VerdictUnknown {
		t.Fatalf("expected unknown verdict, got %s", result.Verdict)
	}
}

