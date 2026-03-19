package bot

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/user/telegram-claude-bot/internal/claude"
	"github.com/user/telegram-claude-bot/internal/events"
	"github.com/user/telegram-claude-bot/internal/format"
	"github.com/user/telegram-claude-bot/internal/store"
	tele "gopkg.in/telebot.v4"
)

// --- Basic Commands ---

func (b *Bot) handleStart(c tele.Context) error {
	user := b.ensureUser(c)
	if user == nil {
		return c.Send("Failed to create user.")
	}

	if !user.IsWhitelisted {
		events.Bus.Emit(events.EventUserJoined, map[string]any{"telegram_id": user.TelegramID, "username": user.Username})
		return c.Send(fmt.Sprintf("Welcome! Your ID: %s\nYou need to be whitelisted by an admin.", user.TelegramID))
	}

	settings := store.GetEffectiveSettings(user.TelegramID, b.config)
	return c.Send(
		fmt.Sprintf("<b>Welcome to Claude Bot!</b>\n\nModel: %s\nEffort: %s\nThinking: %s\n\nSend any message to start chatting with Claude.\nUse /help for commands.",
			settings.Model, settings.Effort, settings.Thinking),
		&tele.SendOptions{ParseMode: tele.ModeHTML},
	)
}

func (b *Bot) handleHelp(c tele.Context) error {
	user := b.ensureUser(c)
	tid := ""
	if user != nil {
		tid = user.TelegramID
	}

	helpText := "<b>Commands</b>\n\n" +
		"<b>Chat</b>\n" +
		"Just send a message to chat with Claude\n" +
		"/ask &lt;question&gt; - Quick Q&amp;A (no tools)\n" +
		"/plan &lt;task&gt; - Plan mode (read-only tools)\n" +
		"/stop - Stop active query\n" +
		"/clear - Clear conversation\n\n" +
		"<b>Settings</b>\n" +
		"/model [name] - View/change AI model\n" +
		"/effort [level] - Set effort (low/medium/high/max)\n" +
		"/thinking [mode] - Set thinking (on/off/adaptive)\n" +
		"/settings - View all settings\n\n" +
		"<b>Data</b>\n" +
		"/project &lt;path&gt; - Set working directory\n" +
		"/rule add|remove|toggle|list - Manage rules\n" +
		"/memory save|get|list|delete|clear - Manage memory\n" +
		"/sessions [switch &lt;id&gt;] - Manage sessions\n" +
		"/file &lt;path&gt; - Send a file\n\n" +
		"<b>Info</b>\n" +
		"/status - Show status\n" +
		"/cost - View usage costs"

	if b.isAdmin(tid) {
		helpText += "\n\n<b>Admin</b>\n" +
			"/admin whitelist|ban|remove|users|stats|rule\n" +
			"/mcp add|remove|toggle|list"
	}

	return c.Send(helpText, &tele.SendOptions{ParseMode: tele.ModeHTML})
}

func (b *Bot) handleClear(c tele.Context) error {
	tid := telegramID(c)
	_ = store.DeactivateSession(tid)
	return c.Send("Conversation cleared. New session will start on next message.")
}

func (b *Bot) handleStop(c tele.Context) error {
	tid := telegramID(c)
	if b.claude.HasActiveQuery(tid) {
		if b.claude.InterruptQuery(tid) {
			return c.Send("Query interrupted.")
		}
		return c.Send("Failed to interrupt query.")
	}
	return c.Send("No active query to stop.")
}

func (b *Bot) handleStatus(c tele.Context) error {
	tid := telegramID(c)
	user, _ := store.GetUser(tid)
	settings := store.GetEffectiveSettings(tid, b.config)
	session, _ := store.GetActiveSession(tid)
	totalCost := store.GetUserTotalCost(tid)
	todayCost := store.GetUserCostToday(tid)

	sessionInfo := "none"
	if session != nil {
		sessionInfo = session.Title
		if sessionInfo == "" {
			sessionInfo = session.SessionID[:min(12, len(session.SessionID))]
		}
	}

	role := "user"
	if user != nil {
		role = user.Role
	}

	return c.Send(
		fmt.Sprintf("<b>Status</b>\nRole: %s\nModel: %s\nEffort: %s\nThinking: %s\nWorking dir: %s\nSession: %s\nCost today: $%.4f\nCost total: $%.4f\nActive queries: %d",
			role, settings.Model, settings.Effort, settings.Thinking,
			userWorkingDir(user), sessionInfo, todayCost, totalCost,
			b.claude.GetActiveProcessCount()),
		&tele.SendOptions{ParseMode: tele.ModeHTML},
	)
}

func (b *Bot) handleCost(c tele.Context) error {
	tid := telegramID(c)
	totalCost := store.GetUserTotalCost(tid)
	todayCost := store.GetUserCostToday(tid)

	return c.Send(
		fmt.Sprintf("<b>Usage Costs</b>\nToday: $%.4f\nTotal: $%.4f", todayCost, totalCost),
		&tele.SendOptions{ParseMode: tele.ModeHTML},
	)
}

func (b *Bot) handleFile(c tele.Context) error {
	path := strings.TrimSpace(c.Message().Payload)
	if path == "" {
		return c.Send("Usage: /file <path>")
	}
	return b.sendFile(c, path)
}

// --- Settings Commands ---

func (b *Bot) handleModel(c tele.Context) error {
	tid := telegramID(c)
	arg := strings.TrimSpace(c.Message().Payload)

	if arg != "" {
		resolved := store.ResolveModelAlias(arg)
		if resolved == "" {
			labels := make([]string, len(store.AvailableModels))
			for i, m := range store.AvailableModels {
				labels[i] = m.Label
			}
			return c.Send("Unknown model. Available: " + strings.Join(labels, ", "))
		}
		if err := store.UpsertSettings(tid, &store.UserSettings{Model: resolved}); err != nil {
			return c.Send("Failed to update model.")
		}
		modelInfo := findModel(resolved)
		label := resolved
		if modelInfo != nil {
			label = modelInfo.Label
		}
		events.Bus.Emit(events.EventSettingChanged, map[string]any{"telegram_id": tid, "setting": "model", "value": resolved})
		return c.Send("Model set to: " + label)
	}

	settings := store.GetEffectiveSettings(tid, b.config)
	return c.Send("Select a model:", ModelPicker(settings.Model))
}

func (b *Bot) handleEffort(c tele.Context) error {
	tid := telegramID(c)
	arg := strings.TrimSpace(strings.ToLower(c.Message().Payload))

	if arg != "" {
		valid := false
		for _, e := range store.EffortLevels {
			if e == arg {
				valid = true
				break
			}
		}
		if !valid {
			settings := store.GetEffectiveSettings(tid, b.config)
			return c.Send(fmt.Sprintf("Current effort: %s\n\nSelect effort level:", settings.Effort), EffortPicker(settings.Effort))
		}
		if err := store.UpsertSettings(tid, &store.UserSettings{Effort: arg}); err != nil {
			return c.Send("Failed to update effort.")
		}
		events.Bus.Emit(events.EventSettingChanged, map[string]any{"telegram_id": tid, "setting": "effort", "value": arg})
		return c.Send("Effort set to: " + arg)
	}

	settings := store.GetEffectiveSettings(tid, b.config)
	return c.Send(fmt.Sprintf("Current effort: %s\n\nSelect effort level:", settings.Effort), EffortPicker(settings.Effort))
}

func (b *Bot) handleThinking(c tele.Context) error {
	tid := telegramID(c)
	arg := strings.TrimSpace(strings.ToLower(c.Message().Payload))

	if arg != "" {
		valid := false
		for _, t := range store.ThinkingModes {
			if t == arg {
				valid = true
				break
			}
		}
		if !valid {
			settings := store.GetEffectiveSettings(tid, b.config)
			return c.Send(fmt.Sprintf("Current thinking: %s\n\nSelect thinking mode:", settings.Thinking), ThinkingPicker(settings.Thinking))
		}
		if err := store.UpsertSettings(tid, &store.UserSettings{Thinking: arg}); err != nil {
			return c.Send("Failed to update thinking.")
		}
		label := arg
		switch arg {
		case "on":
			label = "Enabled"
		case "off":
			label = "Disabled"
		case "adaptive":
			label = "Adaptive"
		}
		events.Bus.Emit(events.EventSettingChanged, map[string]any{"telegram_id": tid, "setting": "thinking", "value": arg})
		return c.Send("Thinking set to: " + label)
	}

	settings := store.GetEffectiveSettings(tid, b.config)
	return c.Send(fmt.Sprintf("Current thinking: %s\n\nSelect thinking mode:", settings.Thinking), ThinkingPicker(settings.Thinking))
}

func (b *Bot) handleSettings(c tele.Context) error {
	tid := telegramID(c)
	settings := store.GetEffectiveSettings(tid, b.config)
	return c.Send(
		fmt.Sprintf("<b>Settings</b>\n%s\n\nTap a button to change:", store.FormatSettings(settings)),
		&tele.SendOptions{ParseMode: tele.ModeHTML},
		SettingsMenu(),
	)
}

// --- Project ---

func (b *Bot) handleProject(c tele.Context) error {
	tid := telegramID(c)
	dir := strings.TrimSpace(c.Message().Payload)

	if dir == "" {
		user, _ := store.GetUser(tid)
		wd := "not set"
		if user != nil && user.WorkingDirectory != "" {
			wd = user.WorkingDirectory
		}
		return c.Send(fmt.Sprintf("Current working directory: %s\n\nUsage: /project <path>", wd))
	}

	if !b.validateDir(dir) {
		return c.Send(fmt.Sprintf("Directory not allowed. Allowed: %s", strings.Join(b.getAllowedDirs(), ", ")))
	}

	if err := store.SetWorkingDir(tid, dir); err != nil {
		return c.Send("Failed to set working directory.")
	}
	_ = store.DeactivateSession(tid)

	// Auto-switch to existing session for this folder
	existingSession, _ := store.GetSessionForDir(tid, dir)
	if existingSession != nil {
		_ = store.SwitchSession(tid, existingSession.SessionID)
		title := existingSession.Title
		if title == "" {
			title = existingSession.SessionID[:min(12, len(existingSession.SessionID))]
		}
		return c.Send(fmt.Sprintf("Working directory: %s\nResumed session: %s", dir, title))
	}

	return c.Send(fmt.Sprintf("Working directory: %s\nNew session will start on next message.", dir))
}

// --- Rules ---

func (b *Bot) handleRule(c tele.Context) error {
	tid := telegramID(c)
	args := strings.TrimSpace(c.Message().Payload)

	firstSpace := strings.Index(args, " ")
	sub := args
	rest := ""
	if firstSpace > 0 {
		sub = args[:firstSpace]
		rest = strings.TrimSpace(args[firstSpace+1:])
	}

	switch sub {
	case "add":
		pipeIdx := strings.Index(rest, "|")
		if pipeIdx == -1 {
			return c.Send("Usage: /rule add <name> | <content>")
		}
		name := strings.TrimSpace(rest[:pipeIdx])
		content := strings.TrimSpace(rest[pipeIdx+1:])
		if name == "" || content == "" {
			return c.Send("Usage: /rule add <name> | <content>")
		}
		added, err := store.AddUserRule(tid, name, content)
		if err != nil {
			return c.Send("Failed to add rule.")
		}
		if !added {
			return c.Send(fmt.Sprintf("Rule \"%s\" already exists.", name))
		}
		events.Bus.Emit(events.EventRuleChanged, map[string]any{"telegram_id": tid, "action": "add", "name": name})
		return c.Send(fmt.Sprintf("Rule \"%s\" added.", name))

	case "remove":
		if rest == "" {
			return c.Send("Usage: /rule remove <name>")
		}
		removed, err := store.RemoveUserRule(tid, rest)
		if err != nil {
			return c.Send("Failed to remove rule.")
		}
		if !removed {
			return c.Send(fmt.Sprintf("Rule \"%s\" not found.", rest))
		}
		events.Bus.Emit(events.EventRuleChanged, map[string]any{"telegram_id": tid, "action": "remove", "name": rest})
		return c.Send(fmt.Sprintf("Rule \"%s\" removed.", rest))

	case "toggle":
		if rest == "" {
			return c.Send("Usage: /rule toggle <name>")
		}
		found, isActive, err := store.ToggleUserRule(tid, rest)
		if err != nil {
			return c.Send("Failed to toggle rule.")
		}
		if !found {
			return c.Send(fmt.Sprintf("Rule \"%s\" not found.", rest))
		}
		status := "disabled"
		if isActive {
			status = "enabled"
		}
		events.Bus.Emit(events.EventRuleChanged, map[string]any{"telegram_id": tid, "action": "toggle", "name": rest})
		return c.Send(fmt.Sprintf("Rule \"%s\" %s.", rest, status))

	case "list", "":
		rules, _ := store.ListUserRules(tid)
		if len(rules) == 0 {
			return c.Send("No personal rules. Use /rule add <name> | <content>")
		}
		return c.Send("Your rules:\n" + store.FormatRuleList(rules))

	default:
		return c.Send("Usage: /rule add|remove|toggle|list")
	}
}

// --- Memory ---

func (b *Bot) handleMemory(c tele.Context) error {
	tid := telegramID(c)
	args := strings.TrimSpace(c.Message().Payload)

	firstSpace := strings.Index(args, " ")
	sub := args
	rest := ""
	if firstSpace > 0 {
		sub = args[:firstSpace]
		rest = strings.TrimSpace(args[firstSpace+1:])
	}

	switch sub {
	case "save":
		parts := strings.SplitN(rest, " ", 2)
		if len(parts) < 2 {
			return c.Send("Usage: /memory save <key> <value>")
		}
		if err := store.SetMemory(tid, parts[0], parts[1], "general"); err != nil {
			return c.Send("Failed to save memory.")
		}
		events.Bus.Emit(events.EventMemoryUpdated, map[string]any{"telegram_id": tid, "key": parts[0]})
		return c.Send(fmt.Sprintf("Memory \"%s\" saved.", parts[0]))

	case "get":
		if rest == "" {
			return c.Send("Usage: /memory get <key>")
		}
		mem, err := store.GetMemory(tid, rest)
		if err != nil || mem == nil {
			return c.Send(fmt.Sprintf("Memory \"%s\" not found.", rest))
		}
		return c.Send(fmt.Sprintf("<b>%s</b>\n%s", format.EscapeHTML(mem.Key), format.EscapeHTML(mem.Value)),
			&tele.SendOptions{ParseMode: tele.ModeHTML})

	case "list":
		memories, _ := store.ListMemory(tid)
		if len(memories) == 0 {
			return c.Send("No memories. Use /memory save <key> <value>")
		}
		return c.Send("Your memories:\n" + store.FormatMemoryList(memories))

	case "delete":
		if rest == "" {
			return c.Send("Usage: /memory delete <key>")
		}
		deleted, err := store.DeleteMemory(tid, rest)
		if err != nil {
			return c.Send("Failed to delete memory.")
		}
		if !deleted {
			return c.Send(fmt.Sprintf("Memory \"%s\" not found.", rest))
		}
		events.Bus.Emit(events.EventMemoryUpdated, map[string]any{"telegram_id": tid, "key": rest, "action": "delete"})
		return c.Send(fmt.Sprintf("Memory \"%s\" deleted.", rest))

	case "clear":
		count, _ := store.ClearMemory(tid)
		return c.Send(fmt.Sprintf("Cleared %d memories.", count))

	case "":
		return c.Send("Usage: /memory save|get|list|delete|clear")

	default:
		return c.Send("Usage: /memory save|get|list|delete|clear")
	}
}

// --- Sessions ---

func (b *Bot) handleSessions(c tele.Context) error {
	tid := telegramID(c)
	args := strings.TrimSpace(c.Message().Payload)

	if strings.HasPrefix(args, "switch ") {
		sessionID := strings.TrimPrefix(args, "switch ")
		if err := store.SwitchSession(tid, sessionID); err != nil {
			return c.Send("Failed to switch session.")
		}
		events.Bus.Emit(events.EventSessionChanged, map[string]any{"telegram_id": tid, "session_id": sessionID})
		return c.Send(fmt.Sprintf("Switched to session: %s", sessionID))
	}

	sessions, _ := store.ListSessions(tid)
	if len(sessions) == 0 {
		return c.Send("No sessions. Start chatting to create one.")
	}

	return c.Send("Your sessions:", SessionList(sessions))
}

// --- Ask & Plan ---

func (b *Bot) handleAsk(c tele.Context) error {
	question := strings.TrimSpace(c.Message().Payload)
	if question == "" {
		return c.Send("Usage: /ask <question>")
	}
	return b.sendToClaude(c, question, "ask")
}

func (b *Bot) handlePlan(c tele.Context) error {
	task := strings.TrimSpace(c.Message().Payload)
	if task == "" {
		return c.Send("Usage: /plan <task>")
	}
	return b.sendToClaude(c, task, "plan")
}

// --- Admin ---

func (b *Bot) handleAdmin(c tele.Context) error {
	args := strings.TrimSpace(c.Message().Payload)
	firstSpace := strings.Index(args, " ")
	sub := args
	rest := ""
	if firstSpace > 0 {
		sub = args[:firstSpace]
		rest = strings.TrimSpace(args[firstSpace+1:])
	}

	switch sub {
	case "whitelist":
		if rest == "" {
			return c.Send("Usage: /admin whitelist <telegram_id>")
		}
		// M8: Validate telegram ID is numeric
		if _, err := strconv.ParseInt(rest, 10, 64); err != nil {
			return c.Send("Invalid telegram ID: must be numeric.")
		}
		if err := store.SetWhitelist(rest, true); err != nil {
			// User might not exist yet, create them
			user := &store.User{
				TelegramID:    rest,
				Role:          "user",
				IsWhitelisted: true,
				CreatedAt:     store.NowUTC(),
			}
			// M1: Check and report CreateUser error
			if createErr := store.CreateUser(user); createErr != nil {
				return c.Send(fmt.Sprintf("Failed to whitelist user: %v", createErr))
			}
		}
		return c.Send(fmt.Sprintf("User %s whitelisted.", rest))

	case "ban":
		if rest == "" {
			return c.Send("Usage: /admin ban <telegram_id>")
		}
		// M8: Validate telegram ID is numeric
		if _, err := strconv.ParseInt(rest, 10, 64); err != nil {
			return c.Send("Invalid telegram ID: must be numeric.")
		}
		_ = store.SetWhitelist(rest, false)
		return c.Send(fmt.Sprintf("User %s banned.", rest))

	case "remove":
		if rest == "" {
			return c.Send("Usage: /admin remove <telegram_id>")
		}
		// M8: Validate telegram ID is numeric
		if _, err := strconv.ParseInt(rest, 10, 64); err != nil {
			return c.Send("Invalid telegram ID: must be numeric.")
		}
		_ = store.DeleteUser(rest)
		return c.Send(fmt.Sprintf("User %s removed.", rest))

	case "users":
		users, _ := store.ListAllUsers()
		if len(users) == 0 {
			return c.Send("No users.")
		}
		var sb strings.Builder
		sb.WriteString("<b>Users</b>\n")
		for _, u := range users {
			status := "❌"
			if u.IsWhitelisted {
				status = "✅"
			}
			sb.WriteString(fmt.Sprintf("%s %s (@%s) [%s]\n", status, u.TelegramID, u.Username, u.Role))
		}
		return c.Send(sb.String(), &tele.SendOptions{ParseMode: tele.ModeHTML})

	case "stats":
		stats, _ := store.GetStats()
		costStats := store.GetAllCostStats()
		var sb strings.Builder
		sb.WriteString("<b>Stats</b>\n")
		for k, v := range stats {
			sb.WriteString(fmt.Sprintf("%s: %v\n", k, v))
		}
		sb.WriteString("\n<b>Costs</b>\n")
		for k, v := range costStats {
			sb.WriteString(fmt.Sprintf("%s: $%.4f\n", k, v))
		}
		sb.WriteString(fmt.Sprintf("\nActive queries: %d", b.claude.GetActiveProcessCount()))
		return c.Send(sb.String(), &tele.SendOptions{ParseMode: tele.ModeHTML})

	case "rule":
		return b.handleAdminRule(c, rest)

	case "":
		return c.Send("Usage: /admin whitelist|ban|remove|users|stats|rule")

	default:
		return c.Send("Usage: /admin whitelist|ban|remove|users|stats|rule")
	}
}

func (b *Bot) handleAdminRule(c tele.Context, args string) error {
	firstSpace := strings.Index(args, " ")
	sub := args
	rest := ""
	if firstSpace > 0 {
		sub = args[:firstSpace]
		rest = strings.TrimSpace(args[firstSpace+1:])
	}

	switch sub {
	case "add":
		pipeIdx := strings.Index(rest, "|")
		if pipeIdx == -1 {
			return c.Send("Usage: /admin rule add <name> | <content>")
		}
		name := strings.TrimSpace(rest[:pipeIdx])
		content := strings.TrimSpace(rest[pipeIdx+1:])
		added, _ := store.AddGlobalRule(name, content)
		if !added {
			return c.Send(fmt.Sprintf("Global rule \"%s\" already exists.", name))
		}
		return c.Send(fmt.Sprintf("Global rule \"%s\" added.", name))

	case "remove":
		if rest == "" {
			return c.Send("Usage: /admin rule remove <name>")
		}
		removed, _ := store.RemoveGlobalRule(rest)
		if !removed {
			return c.Send(fmt.Sprintf("Global rule \"%s\" not found.", rest))
		}
		return c.Send(fmt.Sprintf("Global rule \"%s\" removed.", rest))

	case "list":
		rules, _ := store.ListGlobalRules()
		if len(rules) == 0 {
			return c.Send("No global rules.")
		}
		return c.Send("Global rules:\n" + store.FormatRuleList(rules))

	default:
		return c.Send("Usage: /admin rule add|remove|list")
	}
}

// --- MCP ---

func (b *Bot) handleMcp(c tele.Context) error {
	args := strings.TrimSpace(c.Message().Payload)
	firstSpace := strings.Index(args, " ")
	sub := args
	rest := ""
	if firstSpace > 0 {
		sub = args[:firstSpace]
		rest = strings.TrimSpace(args[firstSpace+1:])
	}

	switch sub {
	case "add":
		// Parse: name type config_json
		parts := strings.SplitN(rest, " ", 3)
		if len(parts) < 3 {
			return c.Send("Usage: /mcp add <name> <type> <config_json>")
		}
		server := &store.McpServer{
			Name:      parts[0],
			Type:      parts[1],
			IsActive:  true,
			CreatedAt: store.NowUTC(),
		}
		// Simple parsing - just store command from config
		if err := store.AddMcpServer(server); err != nil {
			return c.Send("Failed to add MCP server.")
		}
		events.Bus.Emit(events.EventMCPChanged, map[string]any{"action": "add", "name": parts[0]})
		return c.Send(fmt.Sprintf("MCP server \"%s\" added.", parts[0]))

	case "remove":
		if rest == "" {
			return c.Send("Usage: /mcp remove <name>")
		}
		removed, _ := store.RemoveMcpServer(rest)
		if !removed {
			return c.Send(fmt.Sprintf("MCP server \"%s\" not found.", rest))
		}
		events.Bus.Emit(events.EventMCPChanged, map[string]any{"action": "remove", "name": rest})
		return c.Send(fmt.Sprintf("MCP server \"%s\" removed.", rest))

	case "toggle":
		if rest == "" {
			return c.Send("Usage: /mcp toggle <name>")
		}
		found, isActive, _ := store.ToggleMcpServer(rest)
		if !found {
			return c.Send(fmt.Sprintf("MCP server \"%s\" not found.", rest))
		}
		status := "disabled"
		if isActive {
			status = "enabled"
		}
		events.Bus.Emit(events.EventMCPChanged, map[string]any{"action": "toggle", "name": rest})
		return c.Send(fmt.Sprintf("MCP server \"%s\" %s.", rest, status))

	case "list", "":
		servers, _ := store.ListMcpServers()
		if len(servers) == 0 {
			return c.Send("No MCP servers configured.")
		}
		return c.Send("MCP Servers:\n" + store.FormatServerList(servers))

	default:
		return c.Send("Usage: /mcp add|remove|toggle|list")
	}
}

// --- Message Handlers ---

func (b *Bot) handleText(c tele.Context) error {
	return b.sendToClaude(c, c.Text(), "full")
}

func (b *Bot) handlePhoto(c tele.Context) error {
	photo := c.Message().Photo
	if photo == nil {
		return c.Send("No photo found.")
	}

	// Download the photo
	reader, err := b.tele.File(&photo.File)
	if err != nil {
		return c.Send("Failed to download photo.")
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return c.Send("Failed to read photo.")
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	mediaType := "image/jpeg"
	if strings.HasSuffix(photo.FilePath, ".png") {
		mediaType = "image/png"
	}

	caption := c.Message().Caption
	if caption == "" {
		caption = "What's in this image?"
	}

	return b.sendToClaudeWithImages(c, caption, []claude.ImageInput{{Base64: b64, MediaType: mediaType}}, "full")
}

func (b *Bot) handleDocument(c tele.Context) error {
	doc := c.Message().Document
	if doc == nil {
		return c.Send("No document found.")
	}

	if doc.FileSize > 20*1024*1024 {
		return c.Send("File too large (max 20MB).")
	}

	reader, err := b.tele.File(&doc.File)
	if err != nil {
		return c.Send("Failed to download document.")
	}
	defer reader.Close()

	// H9: Limit document read to prevent unbounded memory usage (matches FileSize check above)
	data, err := io.ReadAll(io.LimitReader(reader, 20*1024*1024))
	if err != nil {
		return c.Send("Failed to read document.")
	}

	// Check if it's an image
	contentType := http.DetectContentType(data)
	if strings.HasPrefix(contentType, "image/") {
		b64 := base64.StdEncoding.EncodeToString(data)
		caption := c.Message().Caption
		if caption == "" {
			caption = "What's in this image?"
		}
		return b.sendToClaudeWithImages(c, caption, []claude.ImageInput{{Base64: b64, MediaType: contentType}}, "full")
	}

	// Text file
	text := string(data)
	if len(text) > 50000 {
		text = text[:50000] + "\n... (truncated)"
	}

	message := fmt.Sprintf("File: %s\n\n```\n%s\n```", doc.FileName, text)
	if c.Message().Caption != "" {
		message = c.Message().Caption + "\n\n" + message
	}

	return b.sendToClaude(c, message, "full")
}

// --- Core send-to-claude logic ---

func (b *Bot) sendToClaude(c tele.Context, message string, mode string) error {
	return b.sendToClaudeWithImages(c, message, nil, mode)
}

func (b *Bot) sendToClaudeWithImages(c tele.Context, message string, images []claude.ImageInput, mode string) error {
	tid := telegramID(c)
	user, _ := store.GetUser(tid)

	// C1: Embed images as base64 data URLs in the message for Claude CLI
	if len(images) > 0 {
		var imgParts []string
		for i, img := range images {
			imgParts = append(imgParts, fmt.Sprintf("[Image %d: data:%s;base64,%s]", i+1, img.MediaType, img.Base64))
		}
		message = strings.Join(imgParts, "\n") + "\n\n" + message
	}

	// Rate limit check
	rlResult := b.rateLimiter.CheckRateLimit(tid)
	if !rlResult.Allowed {
		return c.Send(fmt.Sprintf("Rate limited: %s\nRetry in %d seconds.", rlResult.Reason, rlResult.RetryAfterSec))
	}

	b.rateLimiter.MarkActive(tid)
	defer b.rateLimiter.MarkInactive(tid)

	settings := store.GetEffectiveSettings(tid, b.config)

	// Get or create session
	session, _ := store.GetActiveSession(tid)
	if session == nil {
		// M6: Prefix temp IDs with "tmp_" for clarity
		sessionID := fmt.Sprintf("tmp_%d", time.Now().UnixNano())
		wd := ""
		if user != nil {
			wd = user.WorkingDirectory
		}
		meta := &store.SessionMeta{
			SessionID:  sessionID,
			TelegramID: tid,
			Title:      truncate(message, 50),
			WorkingDir: wd,
			IsActive:   true,
			CreatedAt:  store.NowUTC(),
			LastUsed:   store.NowUTC(),
		}
		_ = store.SaveSession(meta)
		session = meta
	}

	// Build system prompt
	systemPrompt, _ := store.BuildSystemPrompt(tid)

	events.Bus.Emit(events.EventMessageReceived, map[string]any{
		"telegram_id": tid,
		"message":     truncate(message, 100),
	})

	// Save user message to session
	sessionPath := store.GetSessionPath(tid, session.SessionID)
	_ = store.AppendSessionMessage(sessionPath, "user", message)

	// Send placeholder
	placeholder, err := b.tele.Send(c.Chat(), "Processing...")
	if err != nil {
		return c.Send("Failed to send message.")
	}

	// Prepare Claude options
	opts := claude.ClaudeOptions{
		TelegramID:   tid,
		WorkingDir:   userWorkingDir(user),
		SessionID:    session.SessionID,
		CLISessionID: session.CLISessionID,
		Images:       images,
		Mode:       mode,
		Model:      settings.Model,
		Effort:     settings.Effort,
		Thinking:   settings.Thinking,
		TimeoutMs:  b.config.TimeoutMs,
	}

	// Streaming: throttle message edits to every 3 seconds
	var editMu sync.Mutex
	lastEdit := time.Now()
	var lastText string

	opts.OnPartialResponse = func(text string) {
		editMu.Lock()
		defer editMu.Unlock()
		lastText = text
		if time.Since(lastEdit) > 3*time.Second {
			displayText := text
			if len(displayText) > 4000 {
				displayText = displayText[len(displayText)-4000:]
			}
			_, _ = b.tele.Edit(placeholder, displayText)
			lastEdit = time.Now()
		}
	}

	opts.OnToolUse = func(name string, input string) {
		editMu.Lock()
		defer editMu.Unlock()
		toolMsg := fmt.Sprintf("🔧 Using: %s", name)
		if lastText != "" {
			toolMsg = lastText + "\n\n" + toolMsg
		}
		if len(toolMsg) > 4000 {
			toolMsg = toolMsg[len(toolMsg)-4000:]
		}
		_, _ = b.tele.Edit(placeholder, toolMsg)
	}

	// Add system prompt via dedicated field
	opts.SystemPrompt = systemPrompt

	// Execute query — C9: use request context for cancellation propagation
	ctx := c.Get("ctx")
	if ctx == nil {
		ctx = context.Background()
	}
	result, err := b.claude.SendToClaude(ctx.(context.Context), message, opts)

	if err != nil {
		errMsg := "Error: " + err.Error()
		_, _ = b.tele.Edit(placeholder, errMsg)
		return nil
	}

	// Save Claude CLI session ID for future --resume
	if result.SessionID != "" {
		session.CLISessionID = result.SessionID
		_ = store.SaveSession(session)
	}

	// Save assistant response to session
	if result.Content != "" {
		_ = store.AppendSessionMessage(sessionPath, "assistant", result.Content)
	}

	// Track cost
	if result.CostUSD > 0 {
		_ = store.AddCostRecord(tid, &store.CostRecord{
			SessionID:    session.SessionID,
			CostUSD:      result.CostUSD,
			InputTokens:  result.InputTokens,
			OutputTokens: result.OutputTokens,
			Model:        settings.Model,
			CreatedAt:    store.NowUTC(),
		})
	}

	// Update session last used
	_ = store.UpdateSessionLastUsed(tid, session.SessionID)

	// Compact session if needed
	// L5: Log compaction errors instead of silently discarding
	go func() {
		if err := claude.CompactSessionIfNeeded(b.config, tid, session.SessionID); err != nil {
			log.Printf("[compact] Error compacting session %s for %s: %v", session.SessionID, tid, err)
		}
	}()

	// Send final formatted response
	finalText := result.Content
	if finalText == "" {
		finalText = "(No response)"
	}

	htmlText := format.MarkdownToHTML(finalText)

	events.Bus.Emit(events.EventMessageSent, map[string]any{
		"telegram_id": tid,
		"length":      len(finalText),
		"cost":        result.CostUSD,
	})

	// Try editing placeholder with HTML
	_, editErr := b.tele.Edit(placeholder, htmlText, &tele.SendOptions{ParseMode: tele.ModeHTML})
	if editErr != nil {
		// Fallback: try without HTML formatting
		_, editErr = b.tele.Edit(placeholder, finalText)
		if editErr != nil {
			// If edit fails (message too long), delete and send new
			_ = b.tele.Delete(placeholder)
			return b.sendLong(c, htmlText, string(tele.ModeHTML))
		}
	}

	return nil
}

// --- Helpers ---

func userWorkingDir(user *store.User) string {
	if user != nil && user.WorkingDirectory != "" {
		return user.WorkingDirectory
	}
	wd, _ := os.Getwd()
	return wd
}

// M2: truncate uses rune conversion to avoid breaking multi-byte characters.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// L1: Removed custom min() function — Go 1.21+ builtin is used instead.
