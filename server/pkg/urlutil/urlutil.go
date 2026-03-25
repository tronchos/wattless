package urlutil

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var ErrInvalidURL = errors.New("invalid url")

func Normalize(raw string) (string, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", "", ErrInvalidURL
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	if parsed.Scheme == "" {
		value = "https://" + value
	}

	parsed, err = url.Parse(value)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", "", ErrInvalidURL
	}
	if parsed.Hostname() == "" {
		return "", "", ErrInvalidURL
	}

	parsed.Fragment = ""
	parsed.User = nil

	return parsed.String(), parsed.Hostname(), nil
}
