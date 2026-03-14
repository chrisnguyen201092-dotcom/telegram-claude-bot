# CLAUDE.md

This file provides guidance to Claude Code when working with code in this repository.

## Project Overview

Telegram bot providing Claude AI assistant capabilities to whitelisted users. Built in **Go**, uses Claude CLI subprocess for AI integration. Features per-user model/effort/thinking settings, rule system, memory persistence, session management (Markdown-based), image vision, MCP server config.

## Development Commands

```bash
# Run in development
go run ./cmd/bot/

# Build binary
go build -o dist/telegram-claude-bot ./cmd/bot/

# Run built binary
./dist/telegram-claude-bot

# Run tests
go test ./...

# Tidy dependencies
go mod tidy
```

## Architecture

**Single-process, multi-goroutine design.** One Go binary runs:

1. **Telegram Bot** (`internal/bot/`) — telebot v4 in long-polling mode. 17+ commands, inline keyboard callbacks, photo/document uploads.
2. **Claude Integration** (`internal/claude/`) — Wraps `claude` CLI subprocess with streaming JSON output. Per-user settings, rate limiting, session compaction.

**Data Storage:** JSON files + Markdown (no database).
- `data/users/{telegram_id}.json` — user profiles
- `data/settings/{telegram_id}.json` — per-user settings
- `data/rules/global/{name}.json`, `data/rules/users/{tid}/{name}.json` — rules
- `data/memory/{tid}/{key}.json` — user memories
- `data/sessions/{tid}/{session_id}.md` — sessions as Markdown with YAML frontmatter
- `data/costs/{tid}.json` — cost tracking arrays
- `data/mcp/{name}.json` — MCP server configs
- `data/logs/{date}.json` — daily activity logs
- `data/config.json` — global config overrides

**Data flow:** Bot handler → `claude.SendToClaude()` → CLI subprocess with streaming → callbacks update Telegram message → events broadcast via EventBus
## Key Technical Details

- **Runtime is Go.** Uses standard library + telebot v4 + chi + coder/websocket.
- **Claude CLI** called via `os/exec` with `--output-format stream-json`. Supports session resume, tool allowlists, timeout.
- **No database.** All data stored as JSON files with per-path mutex locking for concurrent safety.
- **Sessions** stored as Markdown files with YAML frontmatter. Messages appended via `os.O_APPEND`.
- **Session compaction** uses Claude haiku to summarize old messages, writes summary to frontmatter.
- **Config** loaded from env vars (`.env`), overridable via `data/config.json`.

## Bot Commands

| Command | Description |
|---------|-------------|
| `/model [name]` | View/change AI model (sonnet/opus/haiku) |
| `/effort [level]` | Set reasoning effort (low/medium/high/max) |
| `/thinking [mode]` | Set thinking mode (on/off/adaptive) |
| `/settings` | View all settings with inline keyboard |
| `/rule add/list/remove/toggle` | Manage personal rules |
| `/memory save/get/list/delete/clear` | Manage per-user memory |
| `/sessions [switch <id>]` | List/switch sessions |
| `/ask <question>` | Quick Q&A without tools |
| `/plan <task>` | Plan mode (read-only tools) |
| `/stop` | Interrupt active query |
| `/cost` | View usage costs |
| `/mcp add/remove/toggle/list` | MCP server management (admin) |
| `/admin rule add/remove/list` | Global rule management (admin) |

## File Map

| Package | Role |
|---------|------|
| `cmd/bot/main.go` | Entry point, lifecycle |
| `internal/bot/` | Bot setup, handlers, callbacks, keyboards, middleware |
| `internal/claude/` | CLI wrapper, types, rate limiting, session compaction |
| `internal/store/` | JSON/Markdown CRUD for all entities |
| `internal/events/` | EventBus pub/sub |
| `internal/format/` | Markdown→HTML converter, message splitting |
| `data/` | Runtime data files (gitignored) |
