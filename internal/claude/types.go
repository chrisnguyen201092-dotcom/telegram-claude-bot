package claude

import "sync"

// ImageInput for vision
type ImageInput struct {
	Base64    string `json:"base64"`
	MediaType string `json:"media_type"` // "image/jpeg", "image/png", "image/gif", "image/webp"
}

// ClaudeOptions for sendToClaude
type ClaudeOptions struct {
	TelegramID     string
	WorkingDir     string
	SessionID      string
	CLISessionID   string // Claude CLI session ID from previous response, for --resume
	SystemPrompt   string
	Images         []ImageInput
	Mode           string // "full", "plan", "ask"
	Model          string
	Effort         string
	Thinking       string
	MaxTurns       *int
	MaxBudget      *float64
	TimeoutMs      int

	// Streaming callbacks
	OnPartialResponse func(text string)
	OnToolUse         func(name string, input string)
	OnThinking        func(text string)
	OnQuestion        func(question string, options []string)
	OnPlan            func(text string)
	OnImage           func(base64 string, mediaType string)
	OnFile            func(path string, content string)
	OnSDKEvent        func(eventType string, data map[string]any)
}

// ClaudeResult returned after query completes
type ClaudeResult struct {
	Content      string
	SessionID    string
	CostUSD      float64
	InputTokens  int
	OutputTokens int
	Error        string
	Plan         string
}

// ToolActivity tracks a tool use during query
type ToolActivity struct {
	Name   string `json:"name"`
	Input  string `json:"input"`
	Status string `json:"status"` // "running", "done", "error"
	Time   string `json:"time"`
}

// M4: QueryActivity tracks an active query with mutex for concurrent Tools access
type QueryActivity struct {
	mu         sync.Mutex     `json:"-"`
	TelegramID string         `json:"telegram_id"`
	Model      string         `json:"model"`
	StartTime  string         `json:"start_time"`
	Tools      []ToolActivity `json:"tools"`
}

// AppendTool safely appends a tool activity under the mutex.
func (qa *QueryActivity) AppendTool(tool ToolActivity) {
	qa.mu.Lock()
	defer qa.mu.Unlock()
	qa.Tools = append(qa.Tools, tool)
}

// MarkLastToolDone marks the last running tool as done.
func (qa *QueryActivity) MarkLastToolDone() {
	qa.mu.Lock()
	defer qa.mu.Unlock()
	for i := len(qa.Tools) - 1; i >= 0; i-- {
		if qa.Tools[i].Status == "running" {
			qa.Tools[i].Status = "done"
			break
		}
	}
}

// CLIEvent represents a parsed event from Claude CLI stream-json output
type CLIEvent struct {
	Type    string         `json:"type"`    // "system", "assistant", "tool_result", "result"
	Subtype string         `json:"subtype"` // "init", "text", "tool_use", "thinking", etc.
	Content string         `json:"content,omitempty"`
	Name    string         `json:"name,omitempty"`  // tool name
	Input   any            `json:"input,omitempty"` // tool input
	Data    map[string]any `json:"data,omitempty"`  // extra data

	// Result fields
	SessionID    string  `json:"session_id,omitempty"`
	Result       string  `json:"result,omitempty"`
	TotalCostUSD float64 `json:"total_cost_usd,omitempty"`
	StopReason   string  `json:"stop_reason,omitempty"`

	// Usage (nested in result event)
	Usage *CLIUsage `json:"usage,omitempty"`

	// Assistant message (nested)
	Message *CLIMessage `json:"message,omitempty"`
}

// CLIUsage represents token usage from result event
type CLIUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// CLIMessage represents assistant message in stream event
type CLIMessage struct {
	Content []CLIContentBlock `json:"content,omitempty"`
}

// CLIContentBlock represents a content block in assistant message
type CLIContentBlock struct {
	Type  string `json:"type"` // "text", "tool_use", "thinking"
	Text  string `json:"text,omitempty"`
	Name  string `json:"name,omitempty"`
	ID    string `json:"id,omitempty"`
	Input any    `json:"input,omitempty"`
}

// PendingQuestion for Claude's AskUserQuestion
type PendingQuestion struct {
	AnswerCh chan string
	Options  []string
	Question string
}
