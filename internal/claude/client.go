package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/user/telegram-claude-bot/internal/events"
	"github.com/user/telegram-claude-bot/internal/store"
)

// Client wraps the Claude CLI subprocess.
type Client struct {
	config           *store.GlobalConfig
	rateLimiter      *RateLimiter
	activeQueries    sync.Map // telegramID -> *exec.Cmd
	queryInfo        sync.Map // telegramID -> *QueryActivity
	pendingQuestions sync.Map // telegramID -> *PendingQuestion
}

// NewClient creates a new Claude CLI client.
func NewClient(cfg *store.GlobalConfig) *Client {
	return &Client{
		config:      cfg,
		rateLimiter: NewRateLimiter(cfg.RateLimitPerMinute, cfg.MaxConcurrent),
	}
}

// SendToClaude sends a message to Claude CLI and streams the response.
func (c *Client) SendToClaude(ctx context.Context, message string, opts ClaudeOptions) (*ClaudeResult, error) {
	timeoutMs := opts.TimeoutMs
	if timeoutMs <= 0 {
		timeoutMs = c.config.TimeoutMs
	}
	if timeoutMs <= 0 {
		timeoutMs = 300000
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	args := buildArgs(message, opts)

	log.Printf("[claude] Running: claude %v", args)

	cmd := exec.CommandContext(ctx, "claude", args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start claude: %w", err)
	}

	c.activeQueries.Store(opts.TelegramID, cmd)
	// H1: Defer cleanup to prevent stuck-active state on panics or early returns
	defer func() {
		c.activeQueries.Delete(opts.TelegramID)
		c.queryInfo.Delete(opts.TelegramID)
	}()
	activity := &QueryActivity{
		TelegramID: opts.TelegramID,
		Model:      opts.Model,
		StartTime:  store.NowUTC(),
		Tools:      []ToolActivity{},
	}
	c.queryInfo.Store(opts.TelegramID, activity)

	events.Bus.Emit(events.EventSDKStart, map[string]any{
		"telegram_id": opts.TelegramID,
		"model":       opts.Model,
		"mode":        opts.Mode,
	})

	result := &ClaudeResult{}

	// Read stderr in background to avoid blocking
	var stderrBuf bytes.Buffer
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		stderrScanner := bufio.NewScanner(stderr)
		for stderrScanner.Scan() {
			stderrBuf.WriteString(stderrScanner.Text())
			stderrBuf.WriteString("\n")
		}
	}()

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 10*1024*1024), 10*1024*1024)

	var accumulatedText string

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		var ev CLIEvent
		if err := json.Unmarshal(line, &ev); err != nil {
			continue
		}

		switch ev.Type {
		case "assistant":
			// Parse message.content blocks
			if ev.Message != nil {
				for _, block := range ev.Message.Content {
					switch block.Type {
					case "text":
						if block.Text != "" {
							accumulatedText += block.Text
							if opts.OnPartialResponse != nil {
								opts.OnPartialResponse(accumulatedText)
							}
						}
					case "tool_use":
						inputStr := ""
						if block.Input != nil {
							b, _ := json.Marshal(block.Input)
							inputStr = string(b)
						}
						toolAct := ToolActivity{
							Name:   block.Name,
							Input:  inputStr,
							Status: "running",
							Time:   store.NowUTC(),
						}
						activity.AppendTool(toolAct)
						if opts.OnToolUse != nil {
							opts.OnToolUse(block.Name, inputStr)
						}
						events.Bus.Emit(events.EventSDKToolUse, map[string]any{
							"telegram_id": opts.TelegramID,
							"tool":        block.Name,
							"input":       inputStr,
						})
					case "thinking":
						if block.Text != "" {
							if opts.OnThinking != nil {
								opts.OnThinking(block.Text)
							}
							events.Bus.Emit(events.EventSDKThinking, map[string]any{
								"telegram_id": opts.TelegramID,
							})
						}
					}
				}
			}
			// Fallback: subtype-based parsing
			if ev.Message == nil {
				switch ev.Subtype {
				case "text":
					if ev.Content != "" {
						accumulatedText += ev.Content
						if opts.OnPartialResponse != nil {
							opts.OnPartialResponse(accumulatedText)
						}
					}
				case "tool_use":
					inputStr := ""
					if ev.Input != nil {
						b, _ := json.Marshal(ev.Input)
						inputStr = string(b)
					}
					toolAct := ToolActivity{
						Name:   ev.Name,
						Input:  inputStr,
						Status: "running",
						Time:   store.NowUTC(),
					}
					activity.AppendTool(toolAct)
					if opts.OnToolUse != nil {
						opts.OnToolUse(ev.Name, inputStr)
					}
					events.Bus.Emit(events.EventSDKToolUse, map[string]any{
						"telegram_id": opts.TelegramID,
						"tool":        ev.Name,
						"input":       inputStr,
					})
				case "thinking":
					if ev.Content != "" {
						if opts.OnThinking != nil {
							opts.OnThinking(ev.Content)
						}
						events.Bus.Emit(events.EventSDKThinking, map[string]any{
							"telegram_id": opts.TelegramID,
						})
					}
				}
			}

		case "tool_result":
			// M4: Use thread-safe method to mark tool done
			activity.MarkLastToolDone()
			events.Bus.Emit(events.EventSDKToolResult, map[string]any{
				"telegram_id": opts.TelegramID,
			})

		case "result":
			result.SessionID = ev.SessionID
			result.CostUSD = ev.TotalCostUSD
			if ev.Usage != nil {
				result.InputTokens = ev.Usage.InputTokens
				result.OutputTokens = ev.Usage.OutputTokens
			}
			if ev.Result != "" {
				result.Content = ev.Result
			} else {
				result.Content = accumulatedText
			}
		}

		if opts.OnSDKEvent != nil {
			opts.OnSDKEvent(ev.Type, ev.Data)
		}
	}

	<-stderrDone

	if err := cmd.Wait(); err != nil {
		errMsg := err.Error()
		if stderrBuf.Len() > 0 {
			errMsg = stderrBuf.String()
		}
		events.Bus.Emit(events.EventSDKError, map[string]any{
			"telegram_id": opts.TelegramID,
			"error":       errMsg,
		})
		log.Printf("[claude] Error: %s | Stderr: %s", err.Error(), stderrBuf.String())
		return nil, fmt.Errorf("claude exited: %s", errMsg)
	}

	if result.Content == "" {
		result.Content = accumulatedText
	}

	events.Bus.Emit(events.EventSDKComplete, map[string]any{
		"telegram_id":   opts.TelegramID,
		"cost_usd":      result.CostUSD,
		"input_tokens":  result.InputTokens,
		"output_tokens": result.OutputTokens,
		"session_id":    result.SessionID,
	})

	return result, nil
}

// buildArgs constructs CLI args for the claude command.
func buildArgs(message string, opts ClaudeOptions) []string {
	args := []string{"--output-format", "stream-json", "--verbose"}

	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}

	// Resume existing Claude CLI session if we have a real CLI session ID.
	// Bot stores the CLI session ID from previous responses.
	if opts.CLISessionID != "" {
		args = append(args, "--resume", opts.CLISessionID)
	}

	if opts.SystemPrompt != "" {
		// Claude CLI arg parser breaks on newlines in --system-prompt value
		sp := strings.ReplaceAll(opts.SystemPrompt, "\n", " ")
		args = append(args, "--system-prompt", sp)
	}

	// C7: Wire Effort, Thinking, MaxTurns, MaxBudget into CLI args
	if opts.Effort != "" {
		args = append(args, "--reasoning-effort", opts.Effort)
	}
	if opts.Thinking != "" {
		args = append(args, "--thinking", opts.Thinking)
	}
	if opts.MaxTurns != nil {
		args = append(args, "--max-turns", fmt.Sprintf("%d", *opts.MaxTurns))
	}

	// Tool allowlist based on mode
	switch opts.Mode {
	case "ask":
		args = append(args, "--tools", "")
	case "plan":
		args = append(args, "--allowedTools", "Read,Glob,Grep,Bash")
	default: // "full"
		// use default tools
	}

	// C3: Use configurable permission mode instead of hardcoded bypassPermissions
	permMode := store.GetPermissionMode()
	args = append(args, "--permission-mode", permMode)

	// -p (--print) is a boolean flag for non-interactive mode.
	// C8: Use -- separator before message to prevent flag injection
	args = append(args, "-p", "--", message)

	return args
}

// InterruptQuery kills the active query for the given telegramID.
func (c *Client) InterruptQuery(telegramID string) bool {
	v, ok := c.activeQueries.Load(telegramID)
	if !ok {
		return false
	}
	cmd := v.(*exec.Cmd)
	if cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	c.activeQueries.Delete(telegramID)
	c.queryInfo.Delete(telegramID)
	c.rateLimiter.MarkInactive(telegramID)
	return true
}

// HasActiveQuery returns true if the telegramID has an active query.
func (c *Client) HasActiveQuery(telegramID string) bool {
	_, ok := c.activeQueries.Load(telegramID)
	return ok
}

// GetActiveProcessCount returns the number of active queries.
func (c *Client) GetActiveProcessCount() int {
	count := 0
	c.activeQueries.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// GetActiveQueryInfo returns all active query activities.
func (c *Client) GetActiveQueryInfo() []*QueryActivity {
	var result []*QueryActivity
	c.queryInfo.Range(func(_, v any) bool {
		if qa, ok := v.(*QueryActivity); ok {
			result = append(result, qa)
		}
		return true
	})
	return result
}

// AnswerQuestion sends an answer to a pending Claude question.
func (c *Client) AnswerQuestion(telegramID string, answer string) {
	v, ok := c.pendingQuestions.Load(telegramID)
	if !ok {
		return
	}
	pq := v.(*PendingQuestion)
	select {
	case pq.AnswerCh <- answer:
	default:
	}
}

// GetPendingQuestion returns the pending question for a telegramID.
func (c *Client) GetPendingQuestion(telegramID string) *PendingQuestion {
	v, ok := c.pendingQuestions.Load(telegramID)
	if !ok {
		return nil
	}
	return v.(*PendingQuestion)
}

// GetRateLimiter returns the rate limiter.
func (c *Client) GetRateLimiter() *RateLimiter {
	return c.rateLimiter
}
