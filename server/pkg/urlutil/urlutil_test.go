package urlutil

import (
	"context"
	"errors"
	"testing"
)

func TestNormalizeAddsScheme(t *testing.T) {
	normalized, hostname, err := Normalize("example.com/landing")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if normalized != "https://example.com/landing" {
		t.Fatalf("unexpected normalized URL: %s", normalized)
	}
	if hostname != "example.com" {
		t.Fatalf("unexpected hostname: %s", hostname)
	}
}

func TestNormalizeKeepsExistingSchemeCaseInsensitive(t *testing.T) {
	normalized, hostname, err := Normalize("HTTP://example.com/landing")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if normalized != "http://example.com/landing" {
		t.Fatalf("unexpected normalized URL: %s", normalized)
	}
	if hostname != "example.com" {
		t.Fatalf("unexpected hostname: %s", hostname)
	}
}

func TestNormalizeRejectsUnsupportedScheme(t *testing.T) {
	if _, _, err := Normalize("ftp://example.com/file.txt"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeRejectsInvalidURL(t *testing.T) {
	if _, _, err := Normalize("://"); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidatePublicTargetRejectsLoopbackIP(t *testing.T) {
	err := ValidatePublicTarget(context.Background(), "127.0.0.1")
	if !errors.Is(err, ErrBlockedTarget) {
		t.Fatalf("expected ErrBlockedTarget, got %v", err)
	}
}

func TestValidatePublicTargetRejectsLocalhost(t *testing.T) {
	err := ValidatePublicTarget(context.Background(), "localhost")
	if !errors.Is(err, ErrBlockedTarget) {
		t.Fatalf("expected ErrBlockedTarget, got %v", err)
	}
}

func TestValidatePublicTargetAllowsPublicIP(t *testing.T) {
	err := ValidatePublicTarget(context.Background(), "1.1.1.1")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}
