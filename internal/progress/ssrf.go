package progress

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// isDisallowedIP reports whether ip must never be a target for an outbound
// progress callback: loopback, link-local (this also covers cloud metadata
// endpoints such as AWS's 169.254.169.254), private (RFC1918/RFC4193),
// unspecified, and multicast addresses.
func isDisallowedIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		ip.IsPrivate()
}

// hostAllowed reports whether host matches one of the allowed patterns.
// A pattern is either an exact hostname (e.g. "api.example.com") or a
// "*.example.com" wildcard matching any subdomain of example.com (but not
// example.com itself, which must be listed separately if intended).
func hostAllowed(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, pattern := range allowed {
		pattern = strings.ToLower(pattern)
		if pattern == host {
			return true
		}
		if suffix, ok := strings.CutPrefix(pattern, "*."); ok && strings.HasSuffix(host, "."+suffix) {
			return true
		}
	}
	return false
}

// newSSRFGuardedTransport returns an http.Transport that resolves DNS
// itself and refuses to dial any IP address isDisallowedIP flags, rather
// than trusting the request's literal hostname string. Checking the
// hostname alone would miss the common bypass of pointing an
// innocent-looking domain at a private or link-local address.
func newSSRFGuardedTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()

	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}

		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, err
		}

		for _, ip := range ips {
			if isDisallowedIP(ip) {
				continue
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}

		return nil, fmt.Errorf("host %q resolves only to disallowed private/loopback/link-local addresses", host)
	}

	return transport
}
