package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/user/telegram-claude-bot/internal/bot"
	"github.com/user/telegram-claude-bot/internal/dashboard"
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
	ctx, cancel := context.WithCancel(context.Background())
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("Shutting down...")
		cancel()
		b.Stop()
	}()

	// H8: Start rate limiter cleanup goroutine
	b.StartCleanup(ctx)

	// L6: Prune old log files (keep 30 days)
	_ = store.PruneLogs(30)

	// H7: Start dashboard server when configured
	if cfg.WebPort > 0 && cfg.AdminAPIKey != "" {
		addr := fmt.Sprintf(":%d", cfg.WebPort)
		go func() {
			log.Printf("Dashboard starting on %s", addr)
			if err := dashboard.StartServer(ctx, addr, cfg); err != nil {
				log.Printf("Dashboard server error: %v", err)
			}
		}()
	}

	log.Println("Bot starting...")
	b.Start()
}
