package urlutil

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var ErrInvalidURL = errors.New("invalid url")
var ErrBlockedTarget = errors.New("scan target must be public")

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

func ValidatePublicTarget(ctx context.Context, hostname string) error {
	value := strings.TrimSpace(strings.TrimSuffix(hostname, "."))
	if value == "" {
		return ErrInvalidURL
	}

	if isExplicitlyBlockedHostname(value) {
		return ErrBlockedTarget
	}

	if ip := net.ParseIP(value); ip != nil {
		if isPrivateOrLocalIP(ip) {
			return ErrBlockedTarget
		}
		return nil
	}

	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, value)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}
	if len(addresses) == 0 {
		return ErrInvalidURL
	}

	for _, address := range addresses {
		if isPrivateOrLocalIP(address.IP) {
			return ErrBlockedTarget
		}
	}

	return nil
}

func isExplicitlyBlockedHostname(hostname string) bool {
	host := strings.ToLower(hostname)
	return host == "localhost" ||
		strings.HasSuffix(host, ".localhost") ||
		strings.HasSuffix(host, ".local") ||
		strings.HasSuffix(host, ".internal")
}

func isPrivateOrLocalIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified()
}
