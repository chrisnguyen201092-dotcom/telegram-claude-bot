package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/user/telegram-claude-bot/internal/bot"
	"github.com/user/telegram-claude-bot/internal/store"
)

func main() {
	// Load .env file (optional, env vars take precedence)
	_ = godotenv.Load()

	// Initialize data directories
	if err := store.InitDataDirs(); err != nil {
		log.Fatalf("Failed to init data dirs: %v", err)
	}

	// Load config
	cfg := store.LoadGlobalConfig()
	if cfg.TelegramBotToken == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN is required (set in .env or environment)")
	}

	// Create and start bot
	b, err := bot.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		b.Stop()
	}()

	log.Println("Bot starting...")
	b.Start()
}
