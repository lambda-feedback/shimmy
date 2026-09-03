package progress

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestIsDisallowedIP(t *testing.T) {
	disallowed := []string{
		"127.0.0.1",       // loopback
		"::1",             // loopback (v6)
		"169.254.169.254", // link-local: cloud metadata endpoint
		"fe80::1",         // link-local (v6)
		"10.0.0.1",        // private RFC1918
		"172.16.0.1",      // private RFC1918
		"192.168.1.1",     // private RFC1918
		"fc00::1",         // private RFC4193
		"0.0.0.0",         // unspecified
		"224.0.0.1",       // multicast
	}
	for _, s := range disallowed {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("failed to parse test IP %q", s)
		}
		if !isDisallowedIP(ip) {
			t.Errorf("expected %q to be disallowed", s)
		}
	}

	allowed := []string{
		"8.8.8.8",
		"1.1.1.1",
		"93.184.216.34",
	}
	for _, s := range allowed {
		ip := net.ParseIP(s)
		if ip == nil {
			t.Fatalf("failed to parse test IP %q", s)
		}
		if isDisallowedIP(ip) {
			t.Errorf("expected %q to be allowed", s)
		}
	}
}

func TestHostAllowed(t *testing.T) {
	allowed := []string{"api.example.com", "*.example.org"}

	cases := []struct {
		host string
		want bool
	}{
		{"api.example.com", true},
		{"API.EXAMPLE.COM", true},
		{"other.example.com", false},
		{"foo.example.org", true},
		{"a.b.example.org", true},
		{"example.org", false}, // bare domain not covered by wildcard
		{"evil.com", false},
	}

	for _, c := range cases {
		if got := hostAllowed(c.host, allowed); got != c.want {
			t.Errorf("hostAllowed(%q, %v) = %v, want %v", c.host, allowed, got, c.want)
		}
	}
}

func TestHTTPFactory_DefaultBlocksLoopbackDelivery(t *testing.T) {
	var received bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewHTTPFactory(HTTPFactoryParams{
		Config: Config{CallbackTimeout: 500 * time.Millisecond},
		Log:    zap.NewNop(),
	})

	r, err := f.NewReporter(srv.URL, "corr-1")
	if err != nil {
		t.Fatalf("expected NewReporter to succeed (block happens at delivery time), got %v", err)
	}

	r.Report(context.Background(), Event{Stage: StageEvaluating})

	if received {
		t.Errorf("expected delivery to a loopback address to be blocked by default")
	}
}

func TestHTTPFactory_AllowPrivateNetworks_PermitsLoopbackDelivery(t *testing.T) {
	var received bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	f := NewHTTPFactory(HTTPFactoryParams{
		Config: Config{CallbackTimeout: time.Second, AllowPrivateNetworks: true},
		Log:    zap.NewNop(),
	})

	r, err := f.NewReporter(srv.URL, "corr-1")
	if err != nil {
		t.Fatalf("expected NewReporter to succeed, got %v", err)
	}

	r.Report(context.Background(), Event{Stage: StageEvaluating})

	if !received {
		t.Errorf("expected delivery to succeed with AllowPrivateNetworks: true")
	}
}

func TestHTTPFactory_AllowedHosts_RejectsUnlistedHost(t *testing.T) {
	f := NewHTTPFactory(HTTPFactoryParams{
		Config: Config{
			CallbackTimeout: time.Second,
			AllowedHosts:    []string{"good.example.com"},
		},
		Log: zap.NewNop(),
	})

	if _, err := f.NewReporter("https://evil.example.com/hook", "corr-1"); err == nil {
		t.Errorf("expected an error for a host not in AllowedHosts")
	}

	r, err := f.NewReporter("https://good.example.com/hook", "corr-1")
	if err != nil {
		t.Fatalf("expected no error for an allowed host, got %v", err)
	}
	if r == nil {
		t.Fatalf("expected a non-nil reporter for an allowed host")
	}
}
