package bot

import (
	"fmt"
	"strings"

	"github.com/user/telegram-claude-bot/internal/store"
	tele "gopkg.in/telebot.v4"
)

// ModelPicker creates an inline keyboard for model selection.
func ModelPicker(currentModel string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for _, m := range store.AvailableModels {
		label := m.Label
		if m.ID == currentModel {
			label = "✓ " + label
		}
		rows = append(rows, menu.Row(menu.Data(label, "model_"+m.ID, "model:"+m.ID)))
	}
	menu.Inline(rows...)
	return menu
}

// EffortPicker creates an inline keyboard for effort level selection.
func EffortPicker(currentEffort string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	var btns []tele.Btn
	for _, e := range store.EffortLevels {
		// M3: Replace deprecated strings.Title with manual capitalization
		label := strings.ToUpper(e[:1]) + e[1:]
		if e == currentEffort {
			label = "✓ " + label
		}
		btns = append(btns, menu.Data(label, "effort_"+e, "effort:"+e))
	}
	menu.Inline(menu.Row(btns...))
	return menu
}

// ThinkingPicker creates an inline keyboard for thinking mode selection.
func ThinkingPicker(currentThinking string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	modes := []struct {
		label string
		value string
	}{
		{"Enabled", "on"},
		{"Disabled", "off"},
		{"Adaptive", "adaptive"},
	}
	var btns []tele.Btn
	for _, m := range modes {
		label := m.label
		if m.value == currentThinking {
			label = "✓ " + label
		}
		btns = append(btns, menu.Data(label, "think_"+m.value, "thinking:"+m.value))
	}
	menu.Inline(menu.Row(btns...))
	return menu
}

// SettingsMenu creates the main settings inline keyboard.
func SettingsMenu() *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(
		menu.Row(
			menu.Data("Model", "s_model", "settings:model"),
			menu.Data("Effort", "s_effort", "settings:effort"),
			menu.Data("Thinking", "s_thinking", "settings:thinking"),
		),
		menu.Row(
			menu.Data("Rules", "s_rules", "settings:rules"),
			menu.Data("Memory", "s_memory", "settings:memory"),
			menu.Data("Sessions", "s_sessions", "settings:sessions"),
		),
	)
	return menu
}

// ConfirmDialog creates a yes/no confirmation keyboard.
func ConfirmDialog(action string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	menu.Inline(menu.Row(
		menu.Data("Yes", "cf_yes", "confirm:"+action+":yes"),
		menu.Data("No", "cf_no", "confirm:"+action+":no"),
	))
	return menu
}

// SessionList creates an inline keyboard listing sessions.
func SessionList(sessions []*store.SessionMeta) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	limit := 5
	if len(sessions) < limit {
		limit = len(sessions)
	}
	for i := 0; i < limit; i++ {
		s := sessions[i]
		label := s.Title
		if label == "" {
			label = s.SessionID
			if len(label) > 12 {
				label = label[:12] + "..."
			}
		}
		if s.IsActive {
			label = "✓ " + label
		}
		// Telegram callback data limit: 64 bytes total.
		// telebot format: "\f{unique}|{data}" — keep both short.
		sid := s.SessionID
		if len(sid) > 36 {
			sid = sid[:36]
		}
		rows = append(rows, menu.Row(menu.Data(label, "ss", "session:"+sid)))
	}
	menu.Inline(rows...)
	return menu
}

// QuestionPicker creates an inline keyboard for Claude's AskUserQuestion.
func QuestionPicker(tid string, options []string) *tele.ReplyMarkup {
	menu := &tele.ReplyMarkup{}
	var rows []tele.Row
	for i, opt := range options {
		data := fmt.Sprintf("claudeq:%s:%d", tid, i)
		rows = append(rows, menu.Row(menu.Data(opt, fmt.Sprintf("cq_%d", i), data)))
	}
	menu.Inline(rows...)
	return menu
}
