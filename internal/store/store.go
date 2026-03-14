package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// DataDir is the base directory for all data files.
var DataDir = "data"

// fileLocks provides per-path mutex locking for concurrent safety.
var fileLocks sync.Map

func lockFile(path string) func() {
	mu, _ := fileLocks.LoadOrStore(path, &sync.Mutex{})
	m := mu.(*sync.Mutex)
	m.Lock()
	return func() { m.Unlock() }
}

// EnsureDir creates a directory (and parents) if it doesn't exist.
func EnsureDir(dir string) error {
	return os.MkdirAll(dir, 0755)
}

// ReadJSON reads a JSON file into a typed struct.
func ReadJSON[T any](path string) (T, error) {
	var result T
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(data, &result)
	return result, err
}

// WriteJSON writes a struct to a JSON file with pretty formatting.
// Creates parent directories if needed.
func WriteJSON(path string, data any) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}

// DeleteFile removes a file if it exists.
func DeleteFile(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// ListJSONFiles lists all .json files in a directory (names without extension).
func ListJSONFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names, nil
}

// ListMDFiles lists all .md files in a directory (names without extension).
func ListMDFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names, nil
}

// ListSubDirs lists subdirectory names in a directory.
func ListSubDirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}

// SafeFilename sanitizes a string for use as a filename.
func SafeFilename(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)
	safe := re.ReplaceAllString(name, "_")
	if safe == "" {
		safe = "_"
	}
	return safe
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// NowUTC returns the current time in UTC formatted as RFC3339.
func NowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// --- Session Markdown Helpers ---

// SessionMeta represents the YAML frontmatter of a session markdown file.
type SessionMeta struct {
	SessionID         string `yaml:"session_id" json:"session_id"`
	TelegramID        string `yaml:"telegram_id" json:"telegram_id"`
	Title             string `yaml:"title" json:"title"`
	WorkingDir        string `yaml:"working_dir" json:"working_dir"`
	IsActive          bool   `yaml:"is_active" json:"is_active"`
	CreatedAt         string `yaml:"created_at" json:"created_at"`
	LastUsed          string `yaml:"last_used" json:"last_used"`
	Summary           string `yaml:"summary,omitempty" json:"summary,omitempty"`
	MessagesCompacted int    `yaml:"messages_compacted,omitempty" json:"messages_compacted,omitempty"`
}

// SessionMessage represents a single message in a session.
type SessionMessage struct {
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

// ParseSessionMD parses a session markdown file into metadata and messages.
func ParseSessionMD(path string) (*SessionMeta, []SessionMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	content := string(data)

	// Split frontmatter
	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return nil, nil, fmt.Errorf("invalid session markdown: missing frontmatter")
	}

	var meta SessionMeta
	if err := yaml.Unmarshal([]byte(parts[1]), &meta); err != nil {
		return nil, nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Parse messages from body
	body := parts[2]
	messages := parseMessages(body)

	return &meta, messages, nil
}

var messageHeaderRe = regexp.MustCompile(`(?m)^## (user|assistant) \| (.+)$`)

func parseMessages(body string) []SessionMessage {
	locs := messageHeaderRe.FindAllStringSubmatchIndex(body, -1)
	if len(locs) == 0 {
		return nil
	}

	var messages []SessionMessage
	for i, loc := range locs {
		role := body[loc[2]:loc[3]]
		timestamp := body[loc[4]:loc[5]]

		// Content is between this header end and next header start (or end of body)
		contentStart := loc[1]
		var contentEnd int
		if i+1 < len(locs) {
			contentEnd = locs[i+1][0]
		} else {
			contentEnd = len(body)
		}
		content := strings.TrimSpace(body[contentStart:contentEnd])

		messages = append(messages, SessionMessage{
			Role:      role,
			Content:   content,
			Timestamp: strings.TrimSpace(timestamp),
		})
	}
	return messages
}

// WriteSessionMD writes a complete session markdown file.
func WriteSessionMD(path string, meta *SessionMeta, messages []SessionMessage) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	yamlData, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	buf.Write(yamlData)
	buf.WriteString("---\n")

	for _, msg := range messages {
		buf.WriteString(fmt.Sprintf("\n## %s | %s\n%s\n", msg.Role, msg.Timestamp, msg.Content))
	}

	return os.WriteFile(path, []byte(buf.String()), 0644)
}

// AppendSessionMessage appends a message to an existing session markdown file.
func AppendSessionMessage(path string, role string, content string) error {
	unlock := lockFile(path)
	defer unlock()

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	ts := NowUTC()
	_, err = fmt.Fprintf(f, "\n## %s | %s\n%s\n", role, ts, content)
	return err
}

// UpdateSessionFrontmatter rewrites just the frontmatter of a session file.
func UpdateSessionFrontmatter(path string, meta *SessionMeta) error {
	unlock := lockFile(path)
	defer unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)

	parts := strings.SplitN(content, "---", 3)
	if len(parts) < 3 {
		return fmt.Errorf("invalid session markdown")
	}

	var buf strings.Builder
	buf.WriteString("---\n")
	yamlData, err := yaml.Marshal(meta)
	if err != nil {
		return err
	}
	buf.Write(yamlData)
	buf.WriteString("---")
	buf.WriteString(parts[2])

	return os.WriteFile(path, []byte(buf.String()), 0644)
}

// InitDataDirs creates all required data directories.
func InitDataDirs() error {
	dirs := []string{
		filepath.Join(DataDir, "users"),
		filepath.Join(DataDir, "settings"),
		filepath.Join(DataDir, "rules", "global"),
		filepath.Join(DataDir, "rules", "users"),
		filepath.Join(DataDir, "memory"),
		filepath.Join(DataDir, "sessions"),
		filepath.Join(DataDir, "costs"),
		filepath.Join(DataDir, "mcp"),
		filepath.Join(DataDir, "logs"),
	}
	for _, d := range dirs {
		if err := EnsureDir(d); err != nil {
			return err
		}
	}
	return nil
}
