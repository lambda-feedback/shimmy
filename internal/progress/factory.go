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
	client  *http.Client
	timeout time.Duration
	log     *zap.Logger
}

var _ Factory = (*HTTPFactory)(nil)

// NewHTTPFactory builds a Factory that delivers progress events as
// outbound HTTP POST requests.
//
// The URL supplied to NewReporter is trusted as-is: today the only caller
// of shimmy's /evaluate endpoint is client-backend, already authenticated
// via the shared Auth.Key. If shimmy ever accepts callback URLs from less
// trusted callers, this is the place to add a host allowlist to close off
// the resulting SSRF surface.
func NewHTTPFactory(params HTTPFactoryParams) Factory {
	timeout := params.Config.CallbackTimeout
	if timeout <= 0 {
		timeout = defaultCallbackTimeout
	}

	return &HTTPFactory{
		client:  &http.Client{},
		timeout: timeout,
		log:     params.Log,
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

	return newHTTPReporter(f.client, callbackURL, correlationID, f.timeout, f.log.Named("progress")), nil
}
