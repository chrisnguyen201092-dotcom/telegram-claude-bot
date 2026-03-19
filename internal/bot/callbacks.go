package bot

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/user/telegram-claude-bot/internal/events"
	"github.com/user/telegram-claude-bot/internal/store"
	tele "gopkg.in/telebot.v4"
)

func (b *Bot) handleCallback(c tele.Context) error {
	data := c.Data()
	_ = c.Respond()

	// telebot v4 prepends "\f{unique}|" to callback data — strip it
	if idx := strings.Index(data, "|"); idx >= 0 && len(data) > 0 && data[0] == '\f' {
		data = data[idx+1:]
	}

	switch {
	case strings.HasPrefix(data, "model:"):
		return b.callbackModel(c, strings.TrimPrefix(data, "model:"))
	case strings.HasPrefix(data, "effort:"):
		return b.callbackEffort(c, strings.TrimPrefix(data, "effort:"))
	case strings.HasPrefix(data, "thinking:"):
		return b.callbackThinking(c, strings.TrimPrefix(data, "thinking:"))
	case strings.HasPrefix(data, "settings:"):
		return b.callbackSettings(c, strings.TrimPrefix(data, "settings:"))
	case strings.HasPrefix(data, "session:"):
		return b.callbackSession(c, strings.TrimPrefix(data, "session:"))
	case strings.HasPrefix(data, "claudeq:"):
		return b.callbackQuestion(c, strings.TrimPrefix(data, "claudeq:"))
	case strings.HasPrefix(data, "confirm:"):
		return b.callbackConfirm(c, strings.TrimPrefix(data, "confirm:"))
	}

	return nil
}

func (b *Bot) callbackModel(c tele.Context, modelID string) error {
	tid := telegramID(c)
	modelInfo := findModel(modelID)
	if modelInfo == nil {
		return c.Send("Unknown model.")
	}
	if err := store.UpsertSettings(tid, &store.UserSettings{Model: modelID}); err != nil {
		return c.Send("Failed to update model.")
	}
	events.Bus.Emit(events.EventSettingChanged, map[string]any{"telegram_id": tid, "setting": "model", "value": modelID})
	return c.Edit(fmt.Sprintf("Model set to: %s", modelInfo.Label))
}

func (b *Bot) callbackEffort(c tele.Context, level string) error {
	tid := telegramID(c)
	// M9: Validate effort level against allowed values
	valid := false
	for _, e := range store.EffortLevels {
		if e == level {
			valid = true
			break
		}
	}
	if !valid {
		return c.Send("Invalid effort level.")
	}
	if err := store.UpsertSettings(tid, &store.UserSettings{Effort: level}); err != nil {
		return c.Send("Failed to update effort.")
	}
	events.Bus.Emit(events.EventSettingChanged, map[string]any{"telegram_id": tid, "setting": "effort", "value": level})
	return c.Edit(fmt.Sprintf("Effort set to: %s", level))
}

func (b *Bot) callbackThinking(c tele.Context, mode string) error {
	tid := telegramID(c)
	// M9: Validate thinking mode against allowed values
	valid := false
	for _, t := range store.ThinkingModes {
		if t == mode {
			valid = true
			break
		}
	}
	if !valid {
		return c.Send("Invalid thinking mode.")
	}
	if err := store.UpsertSettings(tid, &store.UserSettings{Thinking: mode}); err != nil {
		return c.Send("Failed to update thinking.")
	}
	label := mode
	switch mode {
	case "on":
		label = "Enabled"
	case "off":
		label = "Disabled"
	case "adaptive":
		label = "Adaptive"
	}
	events.Bus.Emit(events.EventSettingChanged, map[string]any{"telegram_id": tid, "setting": "thinking", "value": mode})
	return c.Edit(fmt.Sprintf("Thinking set to: %s", label))
}

func (b *Bot) callbackSettings(c tele.Context, item string) error {
	tid := telegramID(c)
	settings := store.GetEffectiveSettings(tid, b.config)

	switch item {
	case "model":
		return c.Edit("Select a model:", ModelPicker(settings.Model))
	case "effort":
		return c.Edit("Select effort level:", EffortPicker(settings.Effort))
	case "thinking":
		return c.Edit("Select thinking mode:", ThinkingPicker(settings.Thinking))
	case "rules":
		rules, _ := store.ListUserRules(tid)
		if len(rules) == 0 {
			return c.Edit("No personal rules. Use /rule add <name> | <content>")
		}
		return c.Edit("Your rules:\n" + store.FormatRuleList(rules))
	case "memory":
		memories, _ := store.ListMemory(tid)
		if len(memories) == 0 {
			return c.Edit("No memories. Use /memory save <key> <value>")
		}
		return c.Edit("Your memories:\n" + store.FormatMemoryList(memories))
	case "sessions":
		sessions, _ := store.ListSessions(tid)
		if len(sessions) == 0 {
			return c.Edit("No sessions.")
		}
		return c.Edit("Your sessions:", SessionList(sessions))
	}
	return nil
}

func (b *Bot) callbackSession(c tele.Context, sessionID string) error {
	tid := telegramID(c)
	if err := store.SwitchSession(tid, sessionID); err != nil {
		return c.Send("Failed to switch session.")
	}
	events.Bus.Emit(events.EventSessionChanged, map[string]any{"telegram_id": tid, "session_id": sessionID})
	return c.Edit(fmt.Sprintf("Switched to session: %s", sessionID))
}

func (b *Bot) callbackQuestion(c tele.Context, data string) error {
	// Format: telegramID:optionIndex
	parts := strings.SplitN(data, ":", 2)
	if len(parts) != 2 {
		return nil
	}
	tid := parts[0]
	idx, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil
	}

	pq := b.claude.GetPendingQuestion(tid)
	if pq == nil {
		return c.Edit("Question already answered or expired.")
	}

	if idx >= 0 && idx < len(pq.Options) {
		answer := pq.Options[idx]
		b.claude.AnswerQuestion(tid, answer)
		events.Bus.Emit(events.EventSDKAskAnswered, map[string]any{"telegram_id": tid, "answer": answer})
		return c.Edit(fmt.Sprintf("Answered: %s", answer))
	}
	return nil
}

func (b *Bot) callbackConfirm(c tele.Context, data string) error {
	// Format: action:yes or action:no
	if strings.HasSuffix(data, ":yes") {
		return c.Edit("Confirmed.")
	}
	return c.Edit("Cancelled.")
}

func findModel(id string) *store.ModelInfo {
	for _, m := range store.AvailableModels {
		if m.ID == id {
			return &m
		}
	}
	return nil
}
