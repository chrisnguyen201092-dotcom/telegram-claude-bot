package bot

import (
	"github.com/user/telegram-claude-bot/internal/store"
	tele "gopkg.in/telebot.v4"
)

// WhitelistOnly returns middleware that only allows whitelisted users.
func (b *Bot) WhitelistOnly() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			user := b.ensureUser(c)
			if user == nil || !user.IsWhitelisted {
				return c.Send("Not authorized.")
			}
			return next(c)
		}
	}
}

// AdminOnly returns middleware that only allows admin users.
func (b *Bot) AdminOnly() tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			user := b.ensureUser(c)
			if user == nil || !b.isAdmin(user.TelegramID) {
				return c.Send("Admin access required.")
			}
			return next(c)
		}
	}
}

// ensureUser gets or creates a user record.
func (b *Bot) ensureUser(c tele.Context) *store.User {
	tid := telegramID(c)
	user, err := store.GetUser(tid)
	if err == nil && user != nil {
		return user
	}

	// Create new user
	username := ""
	displayName := ""
	if c.Sender() != nil {
		username = c.Sender().Username
		displayName = c.Sender().FirstName
		if c.Sender().LastName != "" {
			displayName += " " + c.Sender().LastName
		}
	}

	isAdmin := b.isAdmin(tid)
	user = &store.User{
		TelegramID:  tid,
		Username:    username,
		DisplayName: displayName,
		Role:        "user",
		IsWhitelisted: isAdmin,
		CreatedAt:   store.NowUTC(),
	}
	if isAdmin {
		user.Role = "admin"
	}

	if err := store.CreateUser(user); err != nil {
		return nil
	}
	return user
}
