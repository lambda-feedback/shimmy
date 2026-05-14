package runtime

type MuEdChatRole string

const (
	MuEdChatRoleUser      MuEdChatRole = "USER"
	MuEdChatRoleAssistant MuEdChatRole = "ASSISTANT"
	MuEdChatRoleSystem    MuEdChatRole = "SYSTEM"
)

type MuEdChatMessage struct {
	Role    MuEdChatRole `json:"role"`
	Content string       `json:"content"`
}

type MuEdChatUserPreferences struct {
	Tone     string `json:"tone,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Language string `json:"language,omitempty"`
}

type MuEdChatContext struct {
	Course     map[string]any `json:"course,omitempty"`
	Task       map[string]any `json:"task,omitempty"`
	Submission map[string]any `json:"submission,omitempty"`
}

type MuEdChatLLMConfig struct {
	Model       string         `json:"model,omitempty"`
	Temperature *float64       `json:"temperature,omitempty"`
	MaxTokens   *int           `json:"maxTokens,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

type MuEdChatDataPolicy struct {
	RetainData  bool `json:"retainData"`
	AllowReview bool `json:"allowReview"`
}

type MuEdChatConfiguration struct {
	LLM        *MuEdChatLLMConfig  `json:"llm,omitempty"`
	DataPolicy *MuEdChatDataPolicy `json:"dataPolicy,omitempty"`
}

type MuEdChatRequest struct {
	Messages       []MuEdChatMessage        `json:"messages"`
	ConversationID string                   `json:"conversationId,omitempty"`
	User           *MuEdChatUserPreferences `json:"user,omitempty"`
	Context        *MuEdChatContext         `json:"context,omitempty"`
	Configuration  *MuEdChatConfiguration   `json:"configuration,omitempty"`
}

type MuEdChatResponseMetadata struct {
	Tokens map[string]any `json:"tokens,omitempty"`
	Model  string         `json:"model,omitempty"`
	Timing map[string]any `json:"timing,omitempty"`
}

type MuEdChatResponse struct {
	Output   MuEdChatMessage           `json:"output"`
	Metadata *MuEdChatResponseMetadata `json:"metadata,omitempty"`
}

type MuEdChatHealthStatus string

const (
	MuEdChatHealthStatusOK          MuEdChatHealthStatus = "OK"
	MuEdChatHealthStatusDegraded    MuEdChatHealthStatus = "DEGRADED"
	MuEdChatHealthStatusUnavailable MuEdChatHealthStatus = "UNAVAILABLE"
)

type MuEdChatCapabilities struct {
	Chat            bool `json:"chat"`
	UserPreferences bool `json:"userPreferences"`
	Streaming       bool `json:"streaming"`
	DataPolicy      bool `json:"dataPolicy"`
}

type MuEdChatHealthResponse struct {
	Status               MuEdChatHealthStatus `json:"status"`
	Capabilities         MuEdChatCapabilities `json:"capabilities"`
	SupportedLanguages   []string             `json:"supportedLanguages"`
	SupportedModels      []string             `json:"supportedModels"`
	SupportedAPIVersions []string             `json:"supportedAPIVersions"`
}
