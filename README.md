# Telegram Claude Bot

A Telegram bot that brings Claude AI assistant capabilities to whitelisted users via the Claude CLI. Built in Go with streaming responses, per-user settings, session management, vision support, and an admin dashboard.

Bot Telegram cung cấp khả năng trợ lý Claude AI cho người dùng được cấp quyền thông qua Claude CLI. Xây dựng bằng Go với phản hồi streaming, cài đặt riêng từng người dùng, quản lý phiên, hỗ trợ hình ảnh, và bảng điều khiển quản trị.

---

**[English](#english)** | **[Tiếng Việt](#tiếng-việt)**

---

<a id="english"></a>

# English

## Table of Contents

- [Features](#features)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Usage](#usage)
  - [Bot Commands](#bot-commands)
  - [Settings](#settings)
  - [Rules System](#rules-system)
  - [Memory System](#memory-system)
  - [Session Management](#session-management)
  - [Image & File Support](#image--file-support)
  - [MCP Servers](#mcp-servers-admin)
  - [Admin Commands](#admin-commands)
- [Query Modes](#query-modes)
- [Rate Limiting](#rate-limiting)
- [Dashboard & API](#dashboard--api)
- [Architecture](#architecture)
- [Data Storage](#data-storage)
- [Deployment](#deployment)

---

## Features

- **Claude AI Integration** — Full Claude CLI subprocess with streaming JSON responses
- **Per-User Settings** — Each user configures their own model, effort level, and thinking mode
- **Rule System** — Global admin rules + personal user rules injected into the system prompt
- **Persistent Memory** — Per-user key-value memory that persists across sessions
- **Session Management** — Markdown-based sessions with YAML frontmatter, session switching, and auto-compaction
- **Vision Support** — Send photos and image documents for Claude to analyze
- **File Handling** — Upload text files for analysis, download files from the server
- **MCP Server Support** — Configure Model Context Protocol servers (stdio, SSE, HTTP)
- **Rate Limiting** — Per-user rate limiting with concurrent query protection
- **Cost Tracking** — Track token usage and costs per user
- **Admin Dashboard** — HTTP server with REST API and WebSocket for real-time event monitoring
- **Inline Keyboards** — Interactive settings pickers within Telegram
- **Whitelist System** — Only authorized users can interact with the bot
- **Graceful Shutdown** — Clean process termination on SIGINT/SIGTERM

---

## Prerequisites

| Requirement | Details |
|-------------|---------|
| **Go** | 1.25.4 or later |
| **Claude CLI** | Installed and available in `PATH` ([Install guide](https://docs.anthropic.com/en/docs/claude-code)) |
| **Telegram Bot Token** | Create a bot via [@BotFather](https://t.me/BotFather) |
| **Admin Telegram ID** | Your numeric Telegram user ID |

---

## Installation

```bash
# Clone the repository
git clone https://github.com/user/telegram-claude-bot.git
cd telegram-claude-bot

# Copy and edit configuration
cp .env.example .env
# Edit .env with your values (see Configuration section)

# Install dependencies
go mod tidy

# Run in development
go run ./cmd/bot/

# Or build and run
go build -o dist/telegram-claude-bot ./cmd/bot/
./dist/telegram-claude-bot
```

---

## Configuration

Configuration is loaded from environment variables (`.env` file) and can be overridden via `data/config.json` at runtime.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `TELEGRAM_BOT_TOKEN` | *(required)* | Bot token from @BotFather |
| `ADMIN_TELEGRAM_IDS` | *(required)* | Comma-separated admin Telegram user IDs |
| `ALLOWED_WORKING_DIRS` | *(empty = any)* | Comma-separated allowed base directories for `/project` |
| `CLAUDE_DEFAULT_MODEL` | `claude-sonnet-4-6` | Default Claude model |
| `CLAUDE_DEFAULT_EFFORT` | `high` | Default reasoning effort: `low`, `medium`, `high`, `max` |
| `CLAUDE_DEFAULT_THINKING` | `adaptive` | Default thinking mode: `on`, `off`, `adaptive` |
| `CLAUDE_CLI_TIMEOUT_MS` | `360000` | Claude CLI timeout in ms (default: 6 minutes) |
| `CLAUDE_CONTEXT_MESSAGES` | `10` | Number of context messages sent to Claude |
| `MAX_CONCURRENT_CLI_PROCESSES` | `3` | Max simultaneous Claude processes system-wide |
| `RATE_LIMIT_REQUESTS_PER_MINUTE` | `10` | Per-user request rate limit |
| `WEB_PORT` | `3000` | Dashboard HTTP server port |
| `ADMIN_API_KEY` | *(empty)* | API key for admin dashboard endpoints |
| `COMPACT_THRESHOLD` | `20` | Message count before session compaction triggers |
| `COMPACT_KEEP_RECENT` | `6` | Recent messages to keep during compaction |
| `COMPACT_ENABLED` | `true` | Enable/disable session auto-compaction |

### Runtime Config Override

Create `data/config.json` to override environment values at runtime:

```json
{
  "claude_default_model": "claude-opus-4-6",
  "rate_limit_requests_per_minute": "20"
}
```

Admin can also update config via the dashboard API (`POST /api/config`).

---

## Usage

### Bot Commands

#### General

| Command | Description |
|---------|-------------|
| `/start` | Welcome message with current settings |
| `/help` | Show all available commands |
| `/clear` | Clear conversation and start a new session |
| `/stop` | Interrupt an active Claude query |
| `/status` | Show current status (role, model, session, costs) |
| `/cost` | View token usage costs (today and total) |

#### Chat Modes

| Command | Description |
|---------|-------------|
| *(any text)* | Chat with Claude in full mode (all tools enabled) |
| `/ask <question>` | Quick Q&A without any tools |
| `/plan <task>` | Plan mode with read-only tools (Read, Glob, Grep, Bash) |

#### Working Directory

| Command | Description |
|---------|-------------|
| `/project <path>` | Set working directory for Claude. Auto-switches to existing session for that folder if one exists |
| `/file <path>` | Send a file from the server to Telegram |

### Settings

Change settings via commands or inline keyboard buttons:

| Command | Options | Description |
|---------|---------|-------------|
| `/model [name]` | `sonnet`, `opus`, `haiku` | View or change AI model |
| `/effort [level]` | `low`, `medium`, `high`, `max` | Set reasoning effort level |
| `/thinking [mode]` | `on`, `off`, `adaptive` | Set extended thinking mode |
| `/settings` | — | View all settings with interactive inline buttons |

**Available Models:**

| Alias | Full Model ID |
|-------|---------------|
| `sonnet` | `claude-sonnet-4-6` (Sonnet 4.6) |
| `opus` | `claude-opus-4-6` (Opus 4.6) |
| `haiku` | `claude-haiku-4-5` (Haiku 4.5) |

### Rules System

Rules are custom instructions injected into Claude's system prompt. There are two types:

- **Global Rules** (admin-only) — Apply to all users
- **Personal Rules** — Apply only to the user who created them

| Command | Description |
|---------|-------------|
| `/rule add <name> \| <content>` | Add a personal rule |
| `/rule remove <name>` | Remove a personal rule |
| `/rule toggle <name>` | Enable/disable a personal rule |
| `/rule list` | List all personal rules |

**Example:**
```
/rule add coding | Always use TypeScript with strict mode enabled
/rule add style | Reply in Vietnamese
```

### Memory System

Persistent key-value memory injected into the system prompt for every conversation:

| Command | Description |
|---------|-------------|
| `/memory save <key> <value>` | Save a memory entry |
| `/memory get <key>` | Retrieve a memory by key |
| `/memory list` | List all saved memories |
| `/memory delete <key>` | Delete a specific memory |
| `/memory clear` | Clear all memories |

**Example:**
```
/memory save name My name is John
/memory save lang I prefer Vietnamese responses
```

### Session Management

Each conversation is stored as a Markdown session. Sessions persist across bot restarts.

| Command | Description |
|---------|-------------|
| `/sessions` | List all sessions with their titles |
| `/sessions switch <id>` | Switch to a specific session |
| `/clear` | Start a new session |

**Session Compaction:**
When a session reaches the compaction threshold (default: 20 messages), older messages are automatically summarized using Claude Haiku and the summary is stored in the session frontmatter. The most recent messages (default: 6) are kept intact.

### Image & File Support

**Photos:**
- Send any photo to the bot for vision analysis
- Supported formats: JPEG, PNG, GIF, WebP
- Photos are base64-encoded and sent to Claude

**Documents:**
- Upload text files (code, logs, etc.) for analysis — truncated at 50,000 characters
- Upload image files (detected by content type) for vision
- Maximum file size: 20MB

**File Download:**
- Use `/file <path>` to download files from the server to Telegram

### MCP Servers (Admin)

Configure [Model Context Protocol](https://modelcontextprotocol.io/) servers for extended Claude capabilities:

| Command | Description |
|---------|-------------|
| `/mcp add <name> <type> <config>` | Add an MCP server |
| `/mcp remove <name>` | Remove an MCP server |
| `/mcp toggle <name>` | Enable/disable an MCP server |
| `/mcp list` | List all MCP servers |

**Supported types:** `stdio`, `sse`, `http`

**Example (stdio):**
```
/mcp add filesystem stdio {"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","/home/user"]}
```

### Admin Commands

Only users listed in `ADMIN_TELEGRAM_IDS` can use these commands:

| Command | Description |
|---------|-------------|
| `/admin whitelist <id>` | Whitelist a user by Telegram ID |
| `/admin ban <id>` | Ban a user |
| `/admin remove <id>` | Remove a user completely |
| `/admin users` | List all users with roles and whitelist status |
| `/admin stats` | System statistics and cost summary |
| `/admin rule add <name> \| <content>` | Add a global rule |
| `/admin rule remove <name>` | Remove a global rule |
| `/admin rule list` | List all global rules |

---

## Query Modes

| Mode | Trigger | Tools Available |
|------|---------|-----------------|
| **Full** | Send any text message | All tools enabled |
| **Ask** | `/ask <question>` | No tools (pure Q&A) |
| **Plan** | `/plan <task>` | Read-only: Read, Glob, Grep, Bash |

---

## Rate Limiting

- **Per-user limit:** Configurable requests per minute (default: 10)
- **Concurrency:** Only 1 active query per user at a time
- **System-wide:** Maximum concurrent CLI processes (default: 3)
- **Sliding window:** 1-minute sliding window with automatic cleanup every 5 minutes
- **Feedback:** Users receive "rate limit exceeded" with retry-after time or "query already in progress" messages

---

## Dashboard & API

The bot runs an HTTP dashboard server on the configured port (default: 3000).

### Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` | No | Health check with uptime |
| `GET` | `/ws` | No | WebSocket for real-time events |
| `GET` | `/api/users` | API Key | List all users |
| `GET` | `/api/stats` | API Key | System stats and cost summary |
| `GET` | `/api/config` | API Key | Get current configuration |
| `POST` | `/api/config` | API Key | Update configuration values |
| `GET` | `/api/logs?date=YYYY-MM-DD` | API Key | Get activity logs by date |

### Authentication

Set `ADMIN_API_KEY` in your environment. Authenticate via:
- Header: `X-API-Key: your-key`
- Query parameter: `?api_key=your-key`

### WebSocket Events

Connect to `/ws` to receive real-time events:

| Event | Description |
|-------|-------------|
| `message_received` | User sent a message |
| `message_sent` | Claude response delivered |
| `sdk_start` / `sdk_complete` / `sdk_error` | Query lifecycle |
| `sdk_tool_use` / `sdk_tool_result` | Tool execution |
| `sdk_thinking` | Extended thinking output |
| `user_joined` | New user joined |
| `setting_changed` | User changed settings |
| `session_changed` / `session_compacted` | Session events |
| `rule_changed` / `memory_updated` | Data changes |
| `mcp_changed` / `config_changed` | Configuration changes |

---

## Architecture

Single-process, multi-goroutine design with 3 main subsystems:

```
                    ┌──────────────────┐
                    │   Telegram API   │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │   Bot Handlers   │  ← Commands, messages, callbacks
                    │  (internal/bot)  │
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼───────┐ ┌───▼────┐ ┌───────▼────────┐
     │  Claude Client  │ │ Store  │ │   Dashboard    │
     │(internal/claude)│ │(JSON/MD)│ │(internal/dash) │
     └────────┬───────┘ └────────┘ └───────┬────────┘
              │                            │
     ┌────────▼───────┐           ┌────────▼───────┐
     │   Claude CLI   │           │  HTTP + WS     │
     │  (subprocess)  │           │  (chi router)  │
     └────────────────┘           └────────────────┘
```

### Package Overview

| Package | Role |
|---------|------|
| `cmd/bot/` | Entry point, lifecycle management |
| `internal/bot/` | Telegram bot setup, handlers, callbacks, middleware |
| `internal/claude/` | Claude CLI wrapper, rate limiting, session compaction |
| `internal/store/` | JSON/Markdown CRUD for all data entities |
| `internal/dashboard/` | HTTP server, REST API, WebSocket hub |
| `internal/events/` | EventBus publish/subscribe system |
| `internal/format/` | Markdown-to-HTML converter, message splitting |

---

## Data Storage

All data is stored as JSON files and Markdown documents in the `data/` directory (no database required):

```
data/
├── config.json                             # Runtime config overrides
├── users/{telegram_id}.json                # User profiles
├── settings/{telegram_id}.json             # Per-user settings
├── rules/
│   ├── global/{name}.json                  # Global rules (admin)
│   └── users/{telegram_id}/{name}.json     # Personal rules
├── memory/{telegram_id}/{key}.json         # Per-user memories
├── sessions/{telegram_id}/{session_id}.md  # Session files (Markdown + YAML)
├── costs/{telegram_id}.json                # Cost tracking arrays
├── mcp/{name}.json                         # MCP server configs
└── logs/{date}.json                        # Daily activity logs
```

---

## Deployment

### Simple Binary Deployment

```bash
# Build
go build -o telegram-claude-bot ./cmd/bot/

# Configure
cp .env.example .env
nano .env  # Edit with real values

# Run
./telegram-claude-bot
```

### Systemd Service (Linux)

Create `/etc/systemd/system/telegram-claude-bot.service`:

```ini
[Unit]
Description=Telegram Claude Bot
After=network.target

[Service]
Type=simple
User=botuser
WorkingDirectory=/opt/telegram-claude-bot
ExecStart=/opt/telegram-claude-bot/telegram-claude-bot
EnvironmentFile=/opt/telegram-claude-bot/.env
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable telegram-claude-bot
sudo systemctl start telegram-claude-bot
sudo journalctl -u telegram-claude-bot -f  # View logs
```

### Important Notes

- The `data/` directory must be writable by the bot process
- Claude CLI must be installed and authenticated on the host
- The bot uses long-polling (no webhook setup needed)
- Set `ALLOWED_WORKING_DIRS` in production to restrict file system access

---

---

<a id="tiếng-việt"></a>

# Tiếng Việt

## Mục Lục

- [Tính Năng](#tính-năng)
- [Yêu Cầu Hệ Thống](#yêu-cầu-hệ-thống)
- [Cài Đặt](#cài-đặt)
- [Cấu Hình](#cấu-hình)
- [Hướng Dẫn Sử Dụng](#hướng-dẫn-sử-dụng)
  - [Các Lệnh Bot](#các-lệnh-bot)
  - [Cài Đặt Cá Nhân](#cài-đặt-cá-nhân)
  - [Hệ Thống Quy Tắc](#hệ-thống-quy-tắc-rules)
  - [Hệ Thống Bộ Nhớ](#hệ-thống-bộ-nhớ-memory)
  - [Quản Lý Phiên](#quản-lý-phiên-sessions)
  - [Hỗ Trợ Hình Ảnh & File](#hỗ-trợ-hình-ảnh--file)
  - [Máy Chủ MCP](#máy-chủ-mcp-admin-1)
  - [Lệnh Quản Trị](#lệnh-quản-trị)
- [Chế Độ Truy Vấn](#chế-độ-truy-vấn)
- [Giới Hạn Tốc Độ](#giới-hạn-tốc-độ)
- [Bảng Điều Khiển & API](#bảng-điều-khiển--api)
- [Kiến Trúc](#kiến-trúc)
- [Lưu Trữ Dữ Liệu](#lưu-trữ-dữ-liệu)
- [Triển Khai](#triển-khai)

---

## Tính Năng

- **Tích hợp Claude AI** — Gọi Claude CLI qua subprocess với phản hồi streaming JSON
- **Cài đặt riêng từng người dùng** — Mỗi người dùng tự chọn model, mức độ suy luận, chế độ tư duy
- **Hệ thống quy tắc** — Quy tắc toàn cục (admin) + quy tắc cá nhân, được chèn vào system prompt
- **Bộ nhớ lưu trữ** — Bộ nhớ key-value cho từng người dùng, tồn tại xuyên phiên
- **Quản lý phiên** — Phiên dạng Markdown với YAML frontmatter, chuyển đổi phiên, tự động nén phiên
- **Hỗ trợ hình ảnh** — Gửi ảnh/tài liệu hình ảnh để Claude phân tích
- **Xử lý file** — Tải lên file văn bản để phân tích, tải xuống file từ server
- **Hỗ trợ MCP Server** — Cấu hình máy chủ Model Context Protocol (stdio, SSE, HTTP)
- **Giới hạn tốc độ** — Giới hạn theo người dùng với bảo vệ truy vấn đồng thời
- **Theo dõi chi phí** — Theo dõi token và chi phí theo từng người dùng
- **Bảng điều khiển** — HTTP server với REST API và WebSocket theo dõi sự kiện thời gian thực
- **Bàn phím inline** — Chọn cài đặt tương tác ngay trong Telegram
- **Hệ thống whitelist** — Chỉ người dùng được cấp quyền mới tương tác được với bot
- **Tắt máy an toàn** — Kết thúc tiến trình sạch sẽ khi nhận SIGINT/SIGTERM

---

## Yêu Cầu Hệ Thống

| Yêu Cầu | Chi Tiết |
|----------|----------|
| **Go** | Phiên bản 1.25.4 trở lên |
| **Claude CLI** | Đã cài đặt và có trong `PATH` ([Hướng dẫn cài đặt](https://docs.anthropic.com/en/docs/claude-code)) |
| **Telegram Bot Token** | Tạo bot qua [@BotFather](https://t.me/BotFather) |
| **Telegram ID của Admin** | ID số của tài khoản Telegram admin |

---

## Cài Đặt

```bash
# Clone repository
git clone https://github.com/user/telegram-claude-bot.git
cd telegram-claude-bot

# Sao chép và chỉnh sửa cấu hình
cp .env.example .env
# Chỉnh sửa .env với các giá trị của bạn (xem mục Cấu Hình)

# Cài đặt dependencies
go mod tidy

# Chạy chế độ phát triển
go run ./cmd/bot/

# Hoặc build và chạy
go build -o dist/telegram-claude-bot ./cmd/bot/
./dist/telegram-claude-bot
```

---

## Cấu Hình

Cấu hình được tải từ biến môi trường (file `.env`) và có thể ghi đè qua `data/config.json` trong lúc chạy.

### Biến Môi Trường

| Biến | Mặc Định | Mô Tả |
|------|----------|-------|
| `TELEGRAM_BOT_TOKEN` | *(bắt buộc)* | Token bot từ @BotFather |
| `ADMIN_TELEGRAM_IDS` | *(bắt buộc)* | Danh sách ID Telegram admin, cách nhau bởi dấu phẩy |
| `ALLOWED_WORKING_DIRS` | *(trống = tất cả)* | Thư mục làm việc cho phép, cách nhau bởi dấu phẩy |
| `CLAUDE_DEFAULT_MODEL` | `claude-sonnet-4-6` | Model Claude mặc định |
| `CLAUDE_DEFAULT_EFFORT` | `high` | Mức suy luận mặc định: `low`, `medium`, `high`, `max` |
| `CLAUDE_DEFAULT_THINKING` | `adaptive` | Chế độ tư duy mặc định: `on`, `off`, `adaptive` |
| `CLAUDE_CLI_TIMEOUT_MS` | `360000` | Timeout CLI (ms, mặc định: 6 phút) |
| `CLAUDE_CONTEXT_MESSAGES` | `10` | Số lượng tin nhắn ngữ cảnh gửi cho Claude |
| `MAX_CONCURRENT_CLI_PROCESSES` | `3` | Số tiến trình Claude đồng thời tối đa |
| `RATE_LIMIT_REQUESTS_PER_MINUTE` | `10` | Giới hạn yêu cầu mỗi phút cho mỗi người dùng |
| `WEB_PORT` | `3000` | Cổng HTTP cho bảng điều khiển |
| `ADMIN_API_KEY` | *(trống)* | API key cho endpoint quản trị |
| `COMPACT_THRESHOLD` | `20` | Số tin nhắn trước khi nén phiên |
| `COMPACT_KEEP_RECENT` | `6` | Số tin nhắn gần đây giữ lại khi nén |
| `COMPACT_ENABLED` | `true` | Bật/tắt tự động nén phiên |

### Ghi Đè Cấu Hình Runtime

Tạo file `data/config.json` để ghi đè các giá trị môi trường khi đang chạy:

```json
{
  "claude_default_model": "claude-opus-4-6",
  "rate_limit_requests_per_minute": "20"
}
```

Admin cũng có thể cập nhật cấu hình qua API (`POST /api/config`).

---

## Hướng Dẫn Sử Dụng

### Các Lệnh Bot

#### Chung

| Lệnh | Mô Tả |
|------|-------|
| `/start` | Tin chào mừng với cài đặt hiện tại |
| `/help` | Hiển thị tất cả lệnh khả dụng |
| `/clear` | Xóa hội thoại và bắt đầu phiên mới |
| `/stop` | Dừng truy vấn Claude đang chạy |
| `/status` | Hiện trạng thái hiện tại (vai trò, model, phiên, chi phí) |
| `/cost` | Xem chi phí sử dụng token (hôm nay và tổng) |

#### Chế Độ Chat

| Lệnh | Mô Tả |
|------|-------|
| *(gửi văn bản bất kỳ)* | Chat với Claude chế độ đầy đủ (tất cả công cụ) |
| `/ask <câu hỏi>` | Hỏi đáp nhanh, không dùng công cụ |
| `/plan <nhiệm vụ>` | Chế độ lập kế hoạch với công cụ chỉ đọc (Read, Glob, Grep, Bash) |

#### Thư Mục Làm Việc

| Lệnh | Mô Tả |
|------|-------|
| `/project <đường dẫn>` | Đặt thư mục làm việc cho Claude. Tự động chuyển sang phiên đã có của thư mục đó nếu tồn tại |
| `/file <đường dẫn>` | Gửi file từ server về Telegram |

### Cài Đặt Cá Nhân

Thay đổi cài đặt qua lệnh hoặc nút inline:

| Lệnh | Lựa Chọn | Mô Tả |
|------|----------|-------|
| `/model [tên]` | `sonnet`, `opus`, `haiku` | Xem hoặc đổi model AI |
| `/effort [mức]` | `low`, `medium`, `high`, `max` | Đặt mức độ suy luận |
| `/thinking [chế_độ]` | `on`, `off`, `adaptive` | Đặt chế độ tư duy mở rộng |
| `/settings` | — | Xem tất cả cài đặt với nút bấm tương tác |

**Các Model Khả Dụng:**

| Tên Tắt | Model ID Đầy Đủ |
|---------|-----------------|
| `sonnet` | `claude-sonnet-4-6` (Sonnet 4.6) |
| `opus` | `claude-opus-4-6` (Opus 4.6) |
| `haiku` | `claude-haiku-4-5` (Haiku 4.5) |

### Hệ Thống Quy Tắc (Rules)

Quy tắc là các chỉ dẫn tùy chỉnh được chèn vào system prompt của Claude. Có hai loại:

- **Quy tắc toàn cục** (chỉ admin) — Áp dụng cho tất cả người dùng
- **Quy tắc cá nhân** — Chỉ áp dụng cho người dùng tạo ra chúng

| Lệnh | Mô Tả |
|------|-------|
| `/rule add <tên> \| <nội dung>` | Thêm quy tắc cá nhân |
| `/rule remove <tên>` | Xóa quy tắc cá nhân |
| `/rule toggle <tên>` | Bật/tắt quy tắc cá nhân |
| `/rule list` | Liệt kê tất cả quy tắc cá nhân |

**Ví dụ:**
```
/rule add coding | Luôn sử dụng TypeScript với strict mode
/rule add ngon_ngu | Trả lời bằng tiếng Việt
```

### Hệ Thống Bộ Nhớ (Memory)

Bộ nhớ key-value lưu trữ lâu dài, được chèn vào system prompt mỗi cuộc hội thoại:

| Lệnh | Mô Tả |
|------|-------|
| `/memory save <key> <giá trị>` | Lưu một mục bộ nhớ |
| `/memory get <key>` | Lấy bộ nhớ theo key |
| `/memory list` | Liệt kê tất cả bộ nhớ đã lưu |
| `/memory delete <key>` | Xóa một bộ nhớ cụ thể |
| `/memory clear` | Xóa tất cả bộ nhớ |

**Ví dụ:**
```
/memory save ten Tôi là Minh
/memory save ngon_ngu Tôi thích trả lời bằng tiếng Việt
```

### Quản Lý Phiên (Sessions)

Mỗi cuộc hội thoại được lưu như một phiên Markdown. Phiên tồn tại qua các lần khởi động lại bot.

| Lệnh | Mô Tả |
|------|-------|
| `/sessions` | Liệt kê tất cả phiên với tiêu đề |
| `/sessions switch <id>` | Chuyển sang phiên cụ thể |
| `/clear` | Bắt đầu phiên mới |

**Nén Phiên Tự Động (Compaction):**
Khi phiên đạt ngưỡng nén (mặc định: 20 tin nhắn), các tin nhắn cũ tự động được tóm tắt bởi Claude Haiku và bản tóm tắt được lưu trong frontmatter phiên. Các tin nhắn gần nhất (mặc định: 6) được giữ nguyên.

### Hỗ Trợ Hình Ảnh & File

**Ảnh chụp:**
- Gửi bất kỳ ảnh nào cho bot để phân tích hình ảnh
- Định dạng hỗ trợ: JPEG, PNG, GIF, WebP
- Ảnh được mã hóa base64 và gửi tới Claude

**Tài liệu:**
- Tải lên file văn bản (code, log, v.v.) để phân tích — giới hạn 50.000 ký tự
- Tải lên file hình ảnh (nhận diện theo content type) để phân tích hình ảnh
- Kích thước file tối đa: 20MB

**Tải file:**
- Sử dụng `/file <đường dẫn>` để tải file từ server về Telegram

### Máy Chủ MCP (Admin)

Cấu hình máy chủ [Model Context Protocol](https://modelcontextprotocol.io/) để mở rộng khả năng của Claude:

| Lệnh | Mô Tả |
|------|-------|
| `/mcp add <tên> <loại> <config>` | Thêm máy chủ MCP |
| `/mcp remove <tên>` | Xóa máy chủ MCP |
| `/mcp toggle <tên>` | Bật/tắt máy chủ MCP |
| `/mcp list` | Liệt kê tất cả máy chủ MCP |

**Loại hỗ trợ:** `stdio`, `sse`, `http`

**Ví dụ (stdio):**
```
/mcp add filesystem stdio {"command":"npx","args":["-y","@modelcontextprotocol/server-filesystem","/home/user"]}
```

### Lệnh Quản Trị

Chỉ người dùng trong danh sách `ADMIN_TELEGRAM_IDS` mới dùng được các lệnh này:

| Lệnh | Mô Tả |
|------|-------|
| `/admin whitelist <id>` | Cấp quyền cho người dùng theo Telegram ID |
| `/admin ban <id>` | Chặn người dùng |
| `/admin remove <id>` | Xóa người dùng hoàn toàn |
| `/admin users` | Liệt kê tất cả người dùng với vai trò và trạng thái |
| `/admin stats` | Thống kê hệ thống và tổng hợp chi phí |
| `/admin rule add <tên> \| <nội dung>` | Thêm quy tắc toàn cục |
| `/admin rule remove <tên>` | Xóa quy tắc toàn cục |
| `/admin rule list` | Liệt kê tất cả quy tắc toàn cục |

---

## Chế Độ Truy Vấn

| Chế Độ | Kích Hoạt | Công Cụ Khả Dụng |
|--------|-----------|-------------------|
| **Đầy Đủ** | Gửi văn bản bất kỳ | Tất cả công cụ |
| **Hỏi Đáp** | `/ask <câu hỏi>` | Không có công cụ (hỏi đáp thuần túy) |
| **Lập Kế Hoạch** | `/plan <nhiệm vụ>` | Chỉ đọc: Read, Glob, Grep, Bash |

---

## Giới Hạn Tốc Độ

- **Giới hạn theo người dùng:** Số yêu cầu tối đa mỗi phút (mặc định: 10)
- **Đồng thời:** Chỉ 1 truy vấn đang hoạt động mỗi người dùng tại một thời điểm
- **Toàn hệ thống:** Số tiến trình CLI đồng thời tối đa (mặc định: 3)
- **Cửa sổ trượt:** Cửa sổ 1 phút với tự động dọn dẹp mỗi 5 phút
- **Phản hồi:** Người dùng nhận thông báo "vượt giới hạn" với thời gian chờ hoặc "truy vấn đang xử lý"

---

## Bảng Điều Khiển & API

Bot chạy HTTP server trên cổng đã cấu hình (mặc định: 3000).

### Các Endpoint

| Phương Thức | Đường Dẫn | Xác Thực | Mô Tả |
|-------------|-----------|----------|-------|
| `GET` | `/health` | Không | Kiểm tra sức khỏe với uptime |
| `GET` | `/ws` | Không | WebSocket cho sự kiện thời gian thực |
| `GET` | `/api/users` | API Key | Liệt kê tất cả người dùng |
| `GET` | `/api/stats` | API Key | Thống kê hệ thống và tổng chi phí |
| `GET` | `/api/config` | API Key | Lấy cấu hình hiện tại |
| `POST` | `/api/config` | API Key | Cập nhật cấu hình |
| `GET` | `/api/logs?date=YYYY-MM-DD` | API Key | Lấy nhật ký hoạt động theo ngày |

### Xác Thực

Đặt `ADMIN_API_KEY` trong môi trường. Xác thực qua:
- Header: `X-API-Key: your-key`
- Tham số query: `?api_key=your-key`

### Sự Kiện WebSocket

Kết nối tới `/ws` để nhận sự kiện thời gian thực:

| Sự Kiện | Mô Tả |
|---------|-------|
| `message_received` | Người dùng gửi tin nhắn |
| `message_sent` | Phản hồi Claude đã gửi |
| `sdk_start` / `sdk_complete` / `sdk_error` | Vòng đời truy vấn |
| `sdk_tool_use` / `sdk_tool_result` | Thực thi công cụ |
| `sdk_thinking` | Đầu ra tư duy mở rộng |
| `user_joined` | Người dùng mới tham gia |
| `setting_changed` | Người dùng thay đổi cài đặt |
| `session_changed` / `session_compacted` | Sự kiện phiên |
| `rule_changed` / `memory_updated` | Thay đổi dữ liệu |
| `mcp_changed` / `config_changed` | Thay đổi cấu hình |

---

## Kiến Trúc

Thiết kế đơn tiến trình, đa goroutine với 3 hệ thống chính:

```
                    ┌──────────────────┐
                    │   Telegram API   │
                    └────────┬─────────┘
                             │
                    ┌────────▼─────────┐
                    │   Bot Handlers   │  ← Lệnh, tin nhắn, callback
                    │  (internal/bot)  │
                    └────────┬─────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
     ┌────────▼───────┐ ┌───▼────┐ ┌───────▼────────┐
     │  Claude Client  │ │ Store  │ │   Dashboard    │
     │(internal/claude)│ │(JSON/MD)│ │(internal/dash) │
     └────────┬───────┘ └────────┘ └───────┬────────┘
              │                            │
     ┌────────▼───────┐           ┌────────▼───────┐
     │   Claude CLI   │           │  HTTP + WS     │
     │  (subprocess)  │           │  (chi router)  │
     └────────────────┘           └────────────────┘
```

### Tổng Quan Package

| Package | Vai Trò |
|---------|---------|
| `cmd/bot/` | Điểm vào, quản lý vòng đời |
| `internal/bot/` | Thiết lập bot Telegram, handler, callback, middleware |
| `internal/claude/` | Wrapper CLI Claude, giới hạn tốc độ, nén phiên |
| `internal/store/` | CRUD JSON/Markdown cho tất cả thực thể dữ liệu |
| `internal/dashboard/` | HTTP server, REST API, WebSocket hub |
| `internal/events/` | Hệ thống EventBus pub/sub |
| `internal/format/` | Chuyển đổi Markdown sang HTML, chia tin nhắn |

---

## Lưu Trữ Dữ Liệu

Tất cả dữ liệu được lưu trữ dưới dạng file JSON và tài liệu Markdown trong thư mục `data/` (không cần cơ sở dữ liệu):

```
data/
├── config.json                             # Ghi đè cấu hình runtime
├── users/{telegram_id}.json                # Hồ sơ người dùng
├── settings/{telegram_id}.json             # Cài đặt riêng người dùng
├── rules/
│   ├── global/{tên}.json                   # Quy tắc toàn cục (admin)
│   └── users/{telegram_id}/{tên}.json      # Quy tắc cá nhân
├── memory/{telegram_id}/{key}.json         # Bộ nhớ theo người dùng
├── sessions/{telegram_id}/{session_id}.md  # File phiên (Markdown + YAML)
├── costs/{telegram_id}.json                # Mảng theo dõi chi phí
├── mcp/{tên}.json                          # Cấu hình MCP server
└── logs/{ngày}.json                        # Nhật ký hoạt động hàng ngày
```

---

## Triển Khai

### Triển Khai Binary Đơn Giản

```bash
# Build
go build -o telegram-claude-bot ./cmd/bot/

# Cấu hình
cp .env.example .env
nano .env  # Chỉnh sửa với các giá trị thực

# Chạy
./telegram-claude-bot
```

### Dịch Vụ Systemd (Linux)

Tạo file `/etc/systemd/system/telegram-claude-bot.service`:

```ini
[Unit]
Description=Telegram Claude Bot
After=network.target

[Service]
Type=simple
User=botuser
WorkingDirectory=/opt/telegram-claude-bot
ExecStart=/opt/telegram-claude-bot/telegram-claude-bot
EnvironmentFile=/opt/telegram-claude-bot/.env
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable telegram-claude-bot
sudo systemctl start telegram-claude-bot
sudo journalctl -u telegram-claude-bot -f  # Xem nhật ký
```

### Lưu Ý Quan Trọng

- Thư mục `data/` phải có quyền ghi cho tiến trình bot
- Claude CLI phải được cài đặt và xác thực trên máy chủ
- Bot sử dụng long-polling (không cần thiết lập webhook)
- Đảm bảo đặt `ALLOWED_WORKING_DIRS` trong môi trường production để giới hạn truy cập hệ thống file

---

## License

MIT

---

*Built with Go, powered by Claude AI.*
