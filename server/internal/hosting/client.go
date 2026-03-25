package hosting

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Verdict string

const (
	VerdictGreen    Verdict = "green"
	VerdictNotGreen Verdict = "not_green"
	VerdictUnknown  Verdict = "unknown"
)

type Result struct {
	HostedBy string  `json:"hosted_by"`
	IsGreen  bool    `json:"is_green"`
	Verdict  Verdict `json:"verdict"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

type response struct {
	HostedBy string `json:"hosted_by"`
	Green    bool   `json:"green"`
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *Client) Check(ctx context.Context, hostname string) (Result, error) {
	if hostname == "" {
		return Result{Verdict: VerdictUnknown}, fmt.Errorf("hostname is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/"+hostname, nil)
	if err != nil {
		return Result{Verdict: VerdictUnknown}, err
	}
	req.Header.Set("accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Result{Verdict: VerdictUnknown}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Result{Verdict: VerdictUnknown}, fmt.Errorf("greencheck returned status %d", resp.StatusCode)
	}

	var payload response
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return Result{Verdict: VerdictUnknown}, err
	}

	result := Result{
		HostedBy: payload.HostedBy,
		IsGreen:  payload.Green,
		Verdict:  VerdictNotGreen,
	}
	if payload.Green {
		result.Verdict = VerdictGreen
	}
	return result, nil
}

