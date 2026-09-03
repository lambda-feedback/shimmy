package runtime

// MuEdAdapter translates between one specific µEd API version's wire format and
// the legacy worker protocol. Exactly one implementation is registered per
// supported version; the HTTP handlers resolve the client's X-Api-Version to an
// adapter and drive it, staying version-agnostic themselves.
//
// Decode (not just transform) sits behind this interface on purpose: a future
// version with a different request shape owns its own json.Unmarshal target and
// its own preview-detection rule without the handlers changing.
type MuEdAdapter interface {
	// Version is the µEd API version this adapter implements, e.g. "0.1.0".
	Version() string

	// DecodeEvaluate parses a POST /evaluate request body into the legacy
	// worker request map and the command to run — CommandEvaluate or
	// CommandPreview. The adapter owns preview detection.
	DecodeEvaluate(body []byte) (legacy map[string]any, command Command, err error)

	// EncodeEvaluateFeedback converts a legacy worker result into the µEd
	// feedback array for the command DecodeEvaluate returned.
	EncodeEvaluateFeedback(command Command, result map[string]any) ([]map[string]any, error)

	// EncodeHealth converts a legacy health result into the µEd health response.
	// streamingEnabled is shimmy's own SSE progress-streaming capability for
	// this deployment; the adapter folds it into the advertised capabilities.
	EncodeHealth(legacyResult map[string]any, streamingEnabled bool) map[string]any

	// DecodeChat parses a POST /chat request body into the worker request map.
	DecodeChat(body []byte) (map[string]any, error)

	// EncodeChat converts a worker chat result into the µEd chat response.
	EncodeChat(result map[string]any) (map[string]any, error)

	// EncodeChatHealth converts a worker chat health result into the µEd chat
	// health response. streamingEnabled is shimmy's own SSE progress-streaming
	// capability for this deployment, overlaid on the worker's reported
	// capabilities.
	EncodeChatHealth(result map[string]any, streamingEnabled bool) map[string]any

	// SupportsStreaming reports whether this µEd version's contract defines the
	// opt-in SSE progress-streaming response surface (the text/event-stream
	// media type on /evaluate and /chat). Handlers only stream when this is
	// true, so a client negotiating a version without that surface always gets
	// the buffered JSON body.
	SupportsStreaming() bool
}

// MuEdRegistry holds the µEd version adapters known to the process, in
// registration order (oldest first).
type MuEdRegistry struct {
	order     []string
	byVersion map[string]MuEdAdapter
}

// NewMuEdRegistry returns an empty registry.
func NewMuEdRegistry() *MuEdRegistry {
	return &MuEdRegistry{byVersion: map[string]MuEdAdapter{}}
}

// Register adds an adapter. Registering a version again replaces the earlier
// adapter but keeps its position in the order.
func (r *MuEdRegistry) Register(a MuEdAdapter) {
	v := a.Version()
	if _, seen := r.byVersion[v]; !seen {
		r.order = append(r.order, v)
	}
	r.byVersion[v] = a
}

// Versions returns the supported versions in registration order.
func (r *MuEdRegistry) Versions() []string {
	out := make([]string, len(r.order))
	copy(out, r.order)
	return out
}

// Supports reports whether version is registered.
func (r *MuEdRegistry) Supports(version string) bool {
	_, ok := r.byVersion[version]
	return ok
}

// Latest is the most recently registered version, or "" when the registry is
// empty. Used only as the value stamped on a 406 response for an unsupported
// version.
func (r *MuEdRegistry) Latest() string {
	if len(r.order) == 0 {
		return ""
	}
	return r.order[len(r.order)-1]
}

// Default is the version used when a client sends no X-Api-Version header: the
// first registered version. Pinned deliberately — it does not track Latest, so
// registering a newer version never silently moves header-less clients onto new
// semantics. Bump it in its own change.
func (r *MuEdRegistry) Default() string {
	if len(r.order) == 0 {
		return ""
	}
	return r.order[0]
}

// Resolve maps a requested version to a concrete supported one: the default
// version when requested is empty, the request itself when supported, otherwise
// the latest supported version.
func (r *MuEdRegistry) Resolve(requested string) string {
	if requested == "" {
		return r.Default()
	}
	if r.Supports(requested) {
		return requested
	}
	return r.Latest()
}

// Adapter returns the adapter for an already-resolved version, or nil.
func (r *MuEdRegistry) Adapter(version string) MuEdAdapter {
	return r.byVersion[version]
}

// defaultMuEdRegistry is the process-wide registry. Version adapter files
// register into it from their init(); see mued_v0_1_0.go.
var defaultMuEdRegistry = NewMuEdRegistry()

// DefaultMuEdRegistry returns the process-wide µEd version registry.
func DefaultMuEdRegistry() *MuEdRegistry { return defaultMuEdRegistry }
