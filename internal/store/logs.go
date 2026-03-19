package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ActivityLog struct {
	Type      string         `json:"type"`
	Level     string         `json:"level"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt string         `json:"created_at"`
}

func logPath(date string) string {
	return filepath.Join(DataDir, "logs", date+".json")
}

func todayDate() string {
	return time.Now().UTC().Format("2006-01-02")
}

func AddLog(logType, level, message string, metadata map[string]any) error {
	date := todayDate()
	path := logPath(date)
	unlock := lockFile(path)
	defer unlock()

	entry := ActivityLog{
		Type:      logType,
		Level:     level,
		Message:   message,
		Metadata:  metadata,
		CreatedAt: NowUTC(),
	}

	var logs []ActivityLog
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &logs)
	}
	logs = append(logs, entry)
	return WriteJSON(path, logs)
}

func GetLogs(date string, limit int) ([]*ActivityLog, error) {
	if date == "" {
		date = todayDate()
	}
	logs, err := ReadJSON[[]ActivityLog](logPath(date))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []*ActivityLog
	for i := range logs {
		result = append(result, &logs[i])
	}
	if limit > 0 && len(result) > limit {
		result = result[len(result)-limit:]
	}
	return result, nil
}

func GetStats() (map[string]any, error) {
	logs, err := GetLogs(todayDate(), 0)
	if err != nil {
		return nil, err
	}
	errorCount := 0
	for _, l := range logs {
		if l.Level == "error" {
			errorCount++
		}
	}
	return map[string]any{
		"total_logs_today": len(logs),
		"error_count":      errorCount,
	}, nil
}

// L6: PruneLogs removes log files older than maxDays to prevent unbounded growth.
func PruneLogs(maxDays int) error {
	dir := filepath.Join(DataDir, "logs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -maxDays).Format("2006-01-02")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		date := strings.TrimSuffix(name, ".json")
		if date < cutoff {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
	return nil
}
