package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type GlobalConfig struct {
	TelegramBotToken   string
	AdminTelegramIDs   []string
	AllowedWorkingDirs []string
	WebPort            int
	DefaultModel       string
	DefaultEffort      string
	DefaultThinking    string
	MaxConcurrent      int
	TimeoutMs          int
	ContextMessages    int
	RateLimitPerMinute int
	AdminAPIKey        string
	CompactThreshold   int
	CompactKeepRecent  int
	CompactEnabled     bool
}

func configPath() string {
	return filepath.Join(DataDir, "config.json")
}

func getEnvInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func getEnvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getEnvList(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	var result []string
	for _, s := range strings.Split(v, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

func LoadGlobalConfig() *GlobalConfig {
	cfg := &GlobalConfig{
		TelegramBotToken:   os.Getenv("TELEGRAM_BOT_TOKEN"),
		AdminTelegramIDs:   getEnvList("ADMIN_TELEGRAM_IDS"),
		AllowedWorkingDirs: getEnvList("ALLOWED_WORKING_DIRS"),
		WebPort:            getEnvInt("WEB_PORT", 3000),
		DefaultModel:       os.Getenv("CLAUDE_DEFAULT_MODEL"),
		DefaultEffort:      os.Getenv("CLAUDE_DEFAULT_EFFORT"),
		DefaultThinking:    os.Getenv("CLAUDE_DEFAULT_THINKING"),
		MaxConcurrent:      getEnvInt("MAX_CONCURRENT", 5),
		TimeoutMs:          getEnvInt("TIMEOUT_MS", 300000),
		ContextMessages:    getEnvInt("CONTEXT_MESSAGES", 20),
		RateLimitPerMinute: getEnvInt("RATE_LIMIT_PER_MINUTE", 10),
		AdminAPIKey:        os.Getenv("ADMIN_API_KEY"),
		CompactThreshold:   getEnvInt("COMPACT_THRESHOLD", 50),
		CompactKeepRecent:  getEnvInt("COMPACT_KEEP_RECENT", 10),
		CompactEnabled:     getEnvBool("COMPACT_ENABLED", true),
	}

	// Apply defaults if empty
	if cfg.DefaultModel == "" {
		cfg.DefaultModel = "claude-sonnet-4-6"
	}
	if cfg.DefaultEffort == "" {
		cfg.DefaultEffort = "medium"
	}
	if cfg.DefaultThinking == "" {
		cfg.DefaultThinking = "adaptive"
	}

	// Override with data/config.json values
	fileConf, err := GetAllConfig()
	if err == nil {
		if v, ok := fileConf["TELEGRAM_BOT_TOKEN"]; ok && v != "" {
			cfg.TelegramBotToken = v
		}
		if v, ok := fileConf["ADMIN_TELEGRAM_IDS"]; ok && v != "" {
			cfg.AdminTelegramIDs = splitTrimmed(v)
		}
		if v, ok := fileConf["ALLOWED_WORKING_DIRS"]; ok && v != "" {
			cfg.AllowedWorkingDirs = splitTrimmed(v)
		}
		if v, ok := fileConf["WEB_PORT"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cfg.WebPort = n
			}
		}
		if v, ok := fileConf["CLAUDE_DEFAULT_MODEL"]; ok && v != "" {
			cfg.DefaultModel = v
		}
		if v, ok := fileConf["CLAUDE_DEFAULT_EFFORT"]; ok && v != "" {
			cfg.DefaultEffort = v
		}
		if v, ok := fileConf["CLAUDE_DEFAULT_THINKING"]; ok && v != "" {
			cfg.DefaultThinking = v
		}
		if v, ok := fileConf["MAX_CONCURRENT"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cfg.MaxConcurrent = n
			}
		}
		if v, ok := fileConf["TIMEOUT_MS"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cfg.TimeoutMs = n
			}
		}
		if v, ok := fileConf["CONTEXT_MESSAGES"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cfg.ContextMessages = n
			}
		}
		if v, ok := fileConf["RATE_LIMIT_PER_MINUTE"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cfg.RateLimitPerMinute = n
			}
		}
		if v, ok := fileConf["ADMIN_API_KEY"]; ok && v != "" {
			cfg.AdminAPIKey = v
		}
		if v, ok := fileConf["COMPACT_THRESHOLD"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cfg.CompactThreshold = n
			}
		}
		if v, ok := fileConf["COMPACT_KEEP_RECENT"]; ok && v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				cfg.CompactKeepRecent = n
			}
		}
		if v, ok := fileConf["COMPACT_ENABLED"]; ok && v != "" {
			if b, err := strconv.ParseBool(v); err == nil {
				cfg.CompactEnabled = b
			}
		}
	}

	return cfg
}

func splitTrimmed(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func GetConfig(key string) string {
	m, err := GetAllConfig()
	if err != nil {
		return ""
	}
	return m[key]
}

func SetConfig(key, value string) error {
	path := configPath()
	unlock := lockFile(path)
	defer unlock()

	m, err := GetAllConfig()
	if err != nil {
		m = make(map[string]string)
	}
	m[key] = value
	return WriteJSON(path, m)
}

func GetAllConfig() (map[string]string, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}
