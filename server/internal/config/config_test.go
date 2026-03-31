package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvOrFilePrefersFileValue(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "gemini_api_key")
	if err := os.WriteFile(secretPath, []byte(" from-file \n"), 0o600); err != nil {
		t.Fatalf("expected secret file to be written: %v", err)
	}

	t.Setenv("GEMINI_API_KEY", "from-env")
	t.Setenv("GEMINI_API_KEY_FILE", secretPath)

	if got := envOrFile("GEMINI_API_KEY"); got != "from-file" {
		t.Fatalf("expected file value to win, got %q", got)
	}
}

func TestLoadUsesGeminiKeyFromFile(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "gemini_api_key")
	if err := os.WriteFile(secretPath, []byte("super-secret\n"), 0o600); err != nil {
		t.Fatalf("expected secret file to be written: %v", err)
	}

	t.Setenv("AI_PROVIDER", "gemini")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GEMINI_API_KEY_FILE", secretPath)

	cfg := Load()
	if cfg.GeminiAPIKey != "super-secret" {
		t.Fatalf("expected Gemini API key from file, got %q", cfg.GeminiAPIKey)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected config validation to accept _FILE secret, got %v", err)
	}
}

func TestEnvOrFileFallsBackToEnvWhenFileIsEmpty(t *testing.T) {
	secretPath := filepath.Join(t.TempDir(), "empty_secret")
	if err := os.WriteFile(secretPath, []byte("\n"), 0o600); err != nil {
		t.Fatalf("expected empty secret file to be written: %v", err)
	}

	t.Setenv("GEMINI_API_KEY", "from-env")
	t.Setenv("GEMINI_API_KEY_FILE", secretPath)

	if got := envOrFile("GEMINI_API_KEY"); got != "from-env" {
		t.Fatalf("expected env fallback when file is empty, got %q", got)
	}
}

func TestLoadClientOriginDefaultsToLocalhost(t *testing.T) {
	t.Setenv("CLIENT_ORIGIN", "")

	cfg := Load()
	if cfg.ClientOrigin != "http://localhost:5173" {
		t.Fatalf("expected default localhost:5173, got %q", cfg.ClientOrigin)
	}
}

func TestLoadClientOriginUsesEnvValue(t *testing.T) {
	t.Setenv("CLIENT_ORIGIN", "https://wattless.example")

	cfg := Load()
	if cfg.ClientOrigin != "https://wattless.example" {
		t.Fatalf("expected CLIENT_ORIGIN value, got %q", cfg.ClientOrigin)
	}
}
