package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port                        string
	ClientOrigin                string
	RequestTimeout              time.Duration
	NavigationTimeout           time.Duration
	NetworkIdleWait             time.Duration
	ViewportWidth               int
	ViewportHeight              int
	FullPageMaxHeight           int
	FullPageSingleShotThreshold int
	FullPageTileHeight          int
	FullPageCaptureQuality      int
	FullPagePrimeMaxDuration    time.Duration
	BrowserBin                  string
	GreencheckBaseURL           string
	AIProvider                  string
	GeminiAPIKey                string
	GeminiModel                 string
	LLMTimeout                  time.Duration
}

func Load() Config {
	return Config{
		Port:                        envOrDefault("PORT", "8080"),
		ClientOrigin:                envOrDefault("CLIENT_ORIGIN", "http://localhost:3000"),
		RequestTimeout:              durationOrDefault("REQUEST_TIMEOUT", 20*time.Second),
		NavigationTimeout:           durationOrDefault("NAVIGATION_TIMEOUT", 15*time.Second),
		NetworkIdleWait:             durationOrDefault("NETWORK_IDLE_WAIT", 1500*time.Millisecond),
		ViewportWidth:               intOrDefault("VIEWPORT_WIDTH", 1440),
		ViewportHeight:              intOrDefault("VIEWPORT_HEIGHT", 900),
		FullPageMaxHeight:           intOrDefault("FULL_PAGE_MAX_HEIGHT", 16000),
		FullPageSingleShotThreshold: intOrDefault("FULL_PAGE_SINGLESHOT_THRESHOLD", 8000),
		FullPageTileHeight:          intOrDefault("FULL_PAGE_TILE_HEIGHT", 2400),
		FullPageCaptureQuality:      intOrDefault("FULL_PAGE_CAPTURE_QUALITY", 80),
		FullPagePrimeMaxDuration:    durationOrDefault("FULL_PAGE_PRIME_MAX_DURATION", 2500*time.Millisecond),
		BrowserBin:                  os.Getenv("BROWSER_BIN"),
		GreencheckBaseURL:           envOrDefault("GREENCHECK_BASE_URL", "https://api.thegreenwebfoundation.org/api/v3/greencheck"),
		AIProvider:                  envOrDefault("AI_PROVIDER", "rule_based"),
		GeminiAPIKey:                os.Getenv("GEMINI_API_KEY"),
		GeminiModel:                 envOrDefault("GEMINI_MODEL", "gemini-2.0-flash"),
		LLMTimeout:                  durationOrDefault("LLM_TIMEOUT", 12*time.Second),
	}
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intOrDefault(key string, fallback int) int {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback
	}

	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
