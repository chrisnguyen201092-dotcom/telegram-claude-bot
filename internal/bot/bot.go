package bot

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/user/telegram-claude-bot/internal/claude"
	"github.com/user/telegram-claude-bot/internal/store"
	tele "gopkg.in/telebot.v4"
)

// Bot holds the telebot instance and dependencies.
type Bot struct {
	tele        *tele.Bot
	claude      *claude.Client
	config      *store.GlobalConfig
	rateLimiter *claude.RateLimiter
}

// New creates and configures a new Bot instance.
func New(cfg *store.GlobalConfig) (*Bot, error) {
	if cfg.TelegramBotToken == "" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is required")
	}

	pref := tele.Settings{
		Token: cfg.TelegramBotToken,
		Poller: &tele.LongPoller{
			Timeout: 10 * time.Second,
		},
	}

	teleBot, err := tele.NewBot(pref)
	if err != nil {
		return nil, fmt.Errorf("create telebot: %w", err)
	}

	claudeClient := claude.NewClient(cfg)

	b := &Bot{
		tele:        teleBot,
		claude:      claudeClient,
		config:      cfg,
		rateLimiter: claudeClient.GetRateLimiter(),
	}

	b.registerHandlers()

	// Set bot commands menu
	commands := []tele.Command{
		{Text: "start", Description: "Start the bot"},
		{Text: "help", Description: "Show help"},
		{Text: "model", Description: "View/change AI model"},
		{Text: "effort", Description: "Set reasoning effort"},
		{Text: "thinking", Description: "Set thinking mode"},
		{Text: "settings", Description: "View all settings"},
		{Text: "ask", Description: "Quick Q&A (no tools)"},
		{Text: "plan", Description: "Plan mode (read-only tools)"},
		{Text: "clear", Description: "Clear conversation"},
		{Text: "stop", Description: "Stop active query"},
		{Text: "status", Description: "Show status"},
		{Text: "cost", Description: "View usage costs"},
		{Text: "project", Description: "Set working directory"},
		{Text: "rule", Description: "Manage rules"},
		{Text: "memory", Description: "Manage memory"},
		{Text: "sessions", Description: "Manage sessions"},
	}
	if err := teleBot.SetCommands(commands); err != nil {
		log.Printf("Failed to set commands: %v", err)
	}

	return b, nil
}

// Start begins polling for updates.
func (b *Bot) Start() {
	log.Printf("Bot started. Admins: %v", b.config.AdminTelegramIDs)
	b.tele.Start()
}

// Stop halts the bot's polling loop.
func (b *Bot) Stop() {
	b.tele.Stop()
}

func (b *Bot) registerHandlers() {
	// Whitelisted group
	wl := b.tele.Group()
	wl.Use(b.WhitelistOnly())

	// Admin group
	admin := b.tele.Group()
	admin.Use(b.AdminOnly())

	// Public commands
	b.tele.Handle("/start", b.handleStart)
	b.tele.Handle("/help", b.handleHelp)

	// Whitelisted commands
	wl.Handle("/clear", b.handleClear)
	wl.Handle("/stop", b.handleStop)
	wl.Handle("/status", b.handleStatus)
	wl.Handle("/cost", b.handleCost)
	wl.Handle("/model", b.handleModel)
	wl.Handle("/effort", b.handleEffort)
	wl.Handle("/thinking", b.handleThinking)
	wl.Handle("/settings", b.handleSettings)
	wl.Handle("/project", b.handleProject)
	wl.Handle("/rule", b.handleRule)
	wl.Handle("/memory", b.handleMemory)
	wl.Handle("/sessions", b.handleSessions)
	wl.Handle("/ask", b.handleAsk)
	wl.Handle("/plan", b.handlePlan)
	wl.Handle("/file", b.handleFile)

	// Admin commands
	admin.Handle("/admin", b.handleAdmin)
	admin.Handle("/mcp", b.handleMcp)

	// Message handlers (on whitelisted group)
	wl.Handle(tele.OnText, b.handleText)
	wl.Handle(tele.OnPhoto, b.handlePhoto)
	wl.Handle(tele.OnDocument, b.handleDocument)

	// Callback queries (global)
	b.tele.Handle(tele.OnCallback, b.handleCallback)
}

// telegramID returns the string telegram ID from context.
func telegramID(c tele.Context) string {
	return strconv.FormatInt(c.Sender().ID, 10)
}
