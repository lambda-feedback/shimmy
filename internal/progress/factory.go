package progress

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

// defaultCallbackTimeout is used when Config.CallbackTimeout is unset.
const defaultCallbackTimeout = time.Second

// Config is the configuration for outbound progress-callback delivery.
type Config struct {
	// CallbackTimeout bounds a single progress callback POST. If unset
	// (or <= 0), defaultCallbackTimeout is used.
	CallbackTimeout time.Duration `conf:"callback_timeout"`

	// AllowedHosts, if non-empty, restricts callback URLs to these hosts.
	// Entries may be an exact hostname (e.g. "api.example.com") or a
	// "*.example.com" wildcard matching any subdomain. Empty means no
	// host restriction — callback delivery is still subject to the
	// private-network protection below.
	AllowedHosts []string `conf:"allowed_hosts"`

	// AllowPrivateNetworks disables the default SSRF protection that
	// refuses to dial loopback, link-local (including cloud metadata
	// endpoints such as 169.254.169.254), and private (RFC1918/RFC4193)
	// IP addresses, however the URL's hostname resolves. Only enable
	// this if shimmy's callback targets are known to live on a private
	// network you trust (e.g. a same-VPC service).
	AllowPrivateNetworks bool `conf:"allow_private_networks"`

	// Sidecar bounds worker-authored progress events delivered via the
	// EVAL_PROGRESS_URL side-channel (see sidecar.go), before they're
	// relayed through the same outbound delivery path as shim-authored
	// events.
	Sidecar SidecarConfig `conf:"sidecar"`

	// Stream configures in-band SSE delivery of progress for /evaluate
	// requests that send "Accept: text/event-stream" (see sse_reporter.go).
	// Only effective in standalone/serve mode; Lambda cannot stream.
	Stream StreamConfig `conf:"stream"`
}

// StreamConfig configures in-band Server-Sent Events progress delivery.
type StreamConfig struct {
	// Enabled turns SSE streaming on. When false, the "Accept:
	// text/event-stream" request header is ignored and /evaluate serves
	// its normal buffered JSON response.
	Enabled bool `conf:"enabled"`

	// HeartbeatSeconds is the spacing between SSE heartbeat comments sent
	// while an evaluation runs, so an idle connection isn't dropped by an
	// intermediary. 0 disables heartbeats.
	HeartbeatSeconds int `conf:"heartbeat_seconds"`
}

// Factory builds a per-request Reporter from caller-supplied callback
// coordinates.
type Factory interface {
	// NewReporter returns a Reporter that delivers events to callbackURL,
	// tagging each with correlationID. If callbackURL is empty, it returns
	// (nil, nil) — the signal that progress reporting is disabled for this
	// request. An error is returned only when callbackURL is non-empty but
	// invalid.
	NewReporter(callbackURL, correlationID string) (Reporter, error)
}

type HTTPFactoryParams struct {
	fx.In

	Config Config
	Log    *zap.Logger
}

type HTTPFactory struct {
	client       *http.Client
	timeout      time.Duration
	log          *zap.Logger
	allowedHosts []string
}

var _ Factory = (*HTTPFactory)(nil)

// NewHTTPFactory builds a Factory that delivers progress events as
// outbound HTTP POST requests.
//
// Since the callback URL is caller-supplied, delivery is guarded against
// SSRF by default: the underlying transport refuses to dial loopback,
// link-local, or private IP addresses (see Config.AllowPrivateNetworks),
// and Config.AllowedHosts can further restrict which hostnames are
// accepted at all.
func NewHTTPFactory(params HTTPFactoryParams) Factory {
	timeout := params.Config.CallbackTimeout
	if timeout <= 0 {
		timeout = defaultCallbackTimeout
	}

	client := &http.Client{}
	if !params.Config.AllowPrivateNetworks {
		client.Transport = newSSRFGuardedTransport()
	}

	return &HTTPFactory{
		client:       client,
		timeout:      timeout,
		log:          params.Log,
		allowedHosts: params.Config.AllowedHosts,
	}
}

func (f *HTTPFactory) NewReporter(callbackURL, correlationID string) (Reporter, error) {
	if callbackURL == "" {
		return nil, nil
	}

	u, err := url.ParseRequestURI(callbackURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("invalid progress callback url: %q", callbackURL)
	}

	if len(f.allowedHosts) > 0 && !hostAllowed(u.Hostname(), f.allowedHosts) {
		return nil, fmt.Errorf("progress callback host %q is not in the allowed hosts list", u.Hostname())
	}

	return newHTTPReporter(f.client, callbackURL, correlationID, f.timeout, f.log.Named("progress")), nil
}
