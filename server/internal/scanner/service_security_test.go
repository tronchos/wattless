package scanner

import (
	"net"
	"testing"
)

func TestPreferredResolvedIPPrioritizesIPv4(t *testing.T) {
	value, err := preferredResolvedIP([]net.IP{
		net.ParseIP("2606:4700:4700::1111"),
		net.ParseIP("1.1.1.1"),
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if value != "1.1.1.1" {
		t.Fatalf("expected IPv4 address, got %q", value)
	}
}

func TestChromiumResolverAddressWrapsIPv6(t *testing.T) {
	if got := chromiumResolverAddress("2606:4700:4700::1111"); got != "[2606:4700:4700::1111]" {
		t.Fatalf("unexpected IPv6 Chromium resolver value: %q", got)
	}
	if got := chromiumResolverAddress("1.1.1.1"); got != "1.1.1.1" {
		t.Fatalf("unexpected IPv4 Chromium resolver value: %q", got)
	}
}
