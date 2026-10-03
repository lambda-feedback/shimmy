package server

import (
	"bytes"
	"net/http"
	"strings"
)

// responseSniffer wraps the real http.ResponseWriter and decides, on the
// handler's first write, whether the response is a Server-Sent Events
// stream (Content-Type: text/event-stream) or a normal buffered response:
//
//   - streaming: the status and headers are committed to the real writer
//     immediately and every subsequent Write is forwarded straight
//     through; Flush delegates to the real writer so frames reach the
//     client incrementally. The OpenAPI response filter is skipped — it
//     has no model for a frame sequence and buffering would defeat the
//     stream.
//   - buffered: the body is accumulated in memory so the middleware can
//     run ValidateResponse against it before anything is sent.
//
// The choice is driven by what the handler actually did, not by a
// pre-flight guess from the request, so the middleware and the handler
// can never disagree about whether a response is streamed.
type responseSniffer struct {
	real    http.ResponseWriter
	status  int
	decided bool
	stream  bool
	buf     bytes.Buffer
}

func newResponseSniffer(real http.ResponseWriter) *responseSniffer {
	return &responseSniffer{real: real, status: http.StatusOK}
}

func (s *responseSniffer) Header() http.Header { return s.real.Header() }

func (s *responseSniffer) WriteHeader(code int) {
	if s.decided {
		return
	}
	s.status = code
	s.decide()
}

func (s *responseSniffer) Write(p []byte) (int, error) {
	if !s.decided {
		s.decide()
	}
	if s.stream {
		return s.real.Write(p)
	}
	return s.buf.Write(p)
}

// Flush forwards to the real writer only once the response has been
// identified as a stream; for a buffered response it is a no-op — the
// body is still being collected for validation.
func (s *responseSniffer) Flush() {
	if !s.stream {
		return
	}
	if f, ok := s.real.(http.Flusher); ok {
		f.Flush()
	}
}

func (s *responseSniffer) decide() {
	s.decided = true
	s.stream = strings.Contains(
		strings.ToLower(s.real.Header().Get("Content-Type")),
		"text/event-stream",
	)
	if s.stream {
		s.real.WriteHeader(s.status)
	}
}

// streamed reports whether the handler wrote a Server-Sent Events
// response that has already been committed to the client, so the
// middleware has nothing left to validate or forward.
func (s *responseSniffer) streamed() bool { return s.decided && s.stream }
