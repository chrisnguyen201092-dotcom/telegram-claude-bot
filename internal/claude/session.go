package claude

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/user/telegram-claude-bot/internal/events"
	"github.com/user/telegram-claude-bot/internal/store"
)

// CompactSessionIfNeeded checks if a session needs compaction and performs it.
func CompactSessionIfNeeded(cfg *store.GlobalConfig, telegramID, sessionID string) error {
	if !cfg.CompactEnabled {
		return nil
	}

	count, err := store.GetSessionMessageCount(telegramID, sessionID)
	if err != nil || count <= cfg.CompactThreshold {
		return err
	}

	messages, err := store.GetSessionMessages(telegramID, sessionID)
	if err != nil {
		return err
	}

	keepRecent := cfg.CompactKeepRecent
	if keepRecent <= 0 {
		keepRecent = 6
	}
	if len(messages) <= keepRecent {
		return nil
	}

	// Split into old (to summarize) and recent (to keep)
	oldMessages := messages[:len(messages)-keepRecent]
	recentMessages := messages[len(messages)-keepRecent:]

	// Build conversation text for summarization
	var convBuf strings.Builder
	for _, msg := range oldMessages {
		convBuf.WriteString(fmt.Sprintf("[%s] %s:\n%s\n\n", msg.Timestamp, msg.Role, msg.Content))
	}

	// Call Claude haiku to summarize
	summary, err := summarizeWithClaude(convBuf.String())
	if err != nil {
		return fmt.Errorf("compaction summarize failed: %w", err)
	}

	// Read current session meta
	sessionPath := store.GetSessionPath(telegramID, sessionID)
	meta, _, err := store.ParseSessionMD(sessionPath)
	if err != nil {
		return err
	}

	// Update meta with summary
	if meta.Summary != "" {
		meta.Summary = meta.Summary + "\n\n" + summary
	} else {
		meta.Summary = summary
	}
	meta.MessagesCompacted += len(oldMessages)

	// Rewrite session file with only recent messages
	if err := store.WriteSessionMD(sessionPath, meta, recentMessages); err != nil {
		return err
	}

	events.Bus.Emit(events.EventSessionCompacted, map[string]any{
		"telegramId":        telegramID,
		"sessionId":         sessionID,
		"messagesCompacted": len(oldMessages),
	})

	return nil
}

func summarizeWithClaude(conversation string) (string, error) {
	prompt := fmt.Sprintf("Summarize this conversation concisely, capturing key decisions, actions taken, and important context. Keep it under 500 words.\n\n%s", conversation)

	cmd := exec.Command("claude", "-p", prompt, "--model", "claude-haiku-4-5", "--output-format", "text", "--no-user-input")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var result strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)
	for scanner.Scan() {
		result.WriteString(scanner.Text())
		result.WriteString("\n")
	}

	if err := cmd.Wait(); err != nil {
		return "", err
	}

	return strings.TrimSpace(result.String()), nil
}
