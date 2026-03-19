package bot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/user/telegram-claude-bot/internal/format"
	tele "gopkg.in/telebot.v4"
)

// sendLong splits a long message and sends chunks.
func (b *Bot) sendLong(c tele.Context, text string, parseMode string) error {
	chunks := format.SplitMessage(text, 4096)
	for i, chunk := range chunks {
		opts := &tele.SendOptions{}
		if parseMode != "" {
			opts.ParseMode = tele.ParseMode(parseMode)
		}
		if i == 0 {
			if _, err := b.tele.Send(c.Chat(), chunk, opts); err != nil {
				// Fallback without parse mode if HTML fails
				if parseMode != "" {
					if _, err2 := b.tele.Send(c.Chat(), chunk); err2 != nil {
						return err2
					}
				} else {
					return err
				}
			}
		} else {
			if _, err := b.tele.Send(c.Chat(), chunk, opts); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateDir checks if a directory is within allowed directories.
// L2: Uses separator-aware prefix check to prevent /allowed_extra bypass.
func (b *Bot) validateDir(dir string) bool {
	allowed := b.config.AllowedWorkingDirs
	if len(allowed) == 0 {
		return true
	}
	cleanDir := filepath.Clean(dir)
	for _, a := range allowed {
		cleanAllowed := filepath.Clean(a)
		// Exact match or proper subdirectory (with separator)
		if cleanDir == cleanAllowed || strings.HasPrefix(cleanDir, cleanAllowed+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// getAllowedDirs returns the list of allowed working directories.
func (b *Bot) getAllowedDirs() []string {
	return b.config.AllowedWorkingDirs
}

// isAdmin checks if a telegram ID is an admin.
func (b *Bot) isAdmin(tid string) bool {
	for _, id := range b.config.AdminTelegramIDs {
		if strings.TrimSpace(id) == tid {
			return true
		}
	}
	return false
}

// sendFile sends a file from the server to the user.
// C2: Validates path against AllowedWorkingDirs to prevent arbitrary file read.
func (b *Bot) sendFile(c tele.Context, path string) error {
	// Clean path to resolve traversal sequences like ../
	path = filepath.Clean(path)

	// Validate the path is within allowed directories
	if !b.validateDir(path) {
		return c.Send(fmt.Sprintf("Access denied. File must be within allowed directories: %s", strings.Join(b.getAllowedDirs(), ", ")))
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return c.Send("File not found: " + path)
	}

	doc := &tele.Document{
		File:     tele.FromDisk(path),
		FileName: filepath.Base(path),
	}
	return c.Send(doc)
}
