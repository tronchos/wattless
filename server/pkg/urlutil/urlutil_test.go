package urlutil

import "testing"

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
