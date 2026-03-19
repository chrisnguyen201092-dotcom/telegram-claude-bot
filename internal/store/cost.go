package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CostRecord struct {
	SessionID    string  `json:"session_id"`
	CostUSD      float64 `json:"cost_usd"`
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	Model        string  `json:"model"`
	CreatedAt    string  `json:"created_at"`
}

// L4: Partition cost files by month to prevent unbounded growth.
func costPath(telegramID string) string {
	month := time.Now().UTC().Format("2006-01")
	dir := filepath.Join(DataDir, "costs", telegramID)
	return filepath.Join(dir, month+".json")
}

// costDir returns the per-user cost directory.
func costDir(telegramID string) string {
	return filepath.Join(DataDir, "costs", telegramID)
}

func AddCostRecord(telegramID string, record *CostRecord) error {
	path := costPath(telegramID)
	unlock := lockFile(path)
	defer unlock()

	if record.CreatedAt == "" {
		record.CreatedAt = NowUTC()
	}

	var records []CostRecord
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &records)
	}
	records = append(records, *record)
	return WriteJSON(path, records)
}

func GetUserTotalCost(telegramID string) float64 {
	dir := costDir(telegramID)
	files, err := ListJSONFiles(dir)
	if err != nil {
		// Fallback: try legacy single-file format
		records, err := ReadJSON[[]CostRecord](filepath.Join(DataDir, "costs", telegramID+".json"))
		if err != nil {
			return 0
		}
		var total float64
		for _, r := range records {
			total += r.CostUSD
		}
		return total
	}
	var total float64
	for _, name := range files {
		records, err := ReadJSON[[]CostRecord](filepath.Join(dir, name+".json"))
		if err != nil {
			continue
		}
		for _, r := range records {
			total += r.CostUSD
		}
	}
	return total
}

func GetUserCostToday(telegramID string) float64 {
	// Today's records are in the current month file
	path := costPath(telegramID)
	records, err := ReadJSON[[]CostRecord](path)
	if err != nil {
		// Fallback: try legacy single-file format
		records, err = ReadJSON[[]CostRecord](filepath.Join(DataDir, "costs", telegramID+".json"))
		if err != nil {
			return 0
		}
	}
	today := time.Now().UTC().Format("2006-01-02")
	var total float64
	for _, r := range records {
		if strings.HasPrefix(r.CreatedAt, today) {
			total += r.CostUSD
		}
	}
	return total
}

func GetAllCostStats() map[string]float64 {
	dir := filepath.Join(DataDir, "costs")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	stats := make(map[string]float64)
	for _, entry := range entries {
		if entry.IsDir() {
			// New format: per-user directory with monthly files
			userID := entry.Name()
			stats[userID] = GetUserTotalCost(userID)
		} else if strings.HasSuffix(entry.Name(), ".json") {
			// Legacy format: single file per user
			name := strings.TrimSuffix(entry.Name(), ".json")
			records, err := ReadJSON[[]CostRecord](filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			var total float64
			for _, r := range records {
				total += r.CostUSD
			}
			stats[name] = total
		}
	}
	return stats
}
