package progress

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func newTestFactory() *HTTPFactory {
	f := NewHTTPFactory(HTTPFactoryParams{
		Config: Config{CallbackTimeout: time.Second},
		Log:    zap.NewNop(),
	})
	return f.(*HTTPFactory)
}

func TestHTTPFactory_NewReporter_EmptyURL_ReturnsNilReporterNoError(t *testing.T) {
	f := newTestFactory()

	r, err := f.NewReporter("", "corr-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if r != nil {
		t.Fatalf("expected nil reporter for empty callback url, got %v", r)
	}
}

func TestHTTPFactory_NewReporter_ValidURL_ReturnsReporter(t *testing.T) {
	f := newTestFactory()

	r, err := f.NewReporter("https://example.com/callback", "corr-1")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if r == nil {
		t.Fatalf("expected non-nil reporter for valid url")
	}
}

func TestHTTPFactory_NewReporter_InvalidURL_ReturnsError(t *testing.T) {
	f := newTestFactory()

	cases := []string{
		"not-a-url",
		"ftp://example.com/callback",
		"://broken",
	}

	for _, c := range cases {
		if _, err := f.NewReporter(c, "corr-1"); err == nil {
			t.Errorf("expected error for callback url %q, got nil", c)
		}
	}
}

func TestNewHTTPFactory_DefaultsTimeoutWhenUnset(t *testing.T) {
	f := NewHTTPFactory(HTTPFactoryParams{
		Config: Config{},
		Log:    zap.NewNop(),
	}).(*HTTPFactory)

	if f.timeout != defaultCallbackTimeout {
		t.Errorf("expected default timeout %v, got %v", defaultCallbackTimeout, f.timeout)
	}
}
