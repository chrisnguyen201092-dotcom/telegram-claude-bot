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

func costPath(telegramID string) string {
	return filepath.Join(DataDir, "costs", telegramID+".json")
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
	records, err := ReadJSON[[]CostRecord](costPath(telegramID))
	if err != nil {
		return 0
	}
	var total float64
	for _, r := range records {
		total += r.CostUSD
	}
	return total
}

func GetUserCostToday(telegramID string) float64 {
	records, err := ReadJSON[[]CostRecord](costPath(telegramID))
	if err != nil {
		return 0
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
	names, err := ListJSONFiles(dir)
	if err != nil {
		return nil
	}
	stats := make(map[string]float64)
	for _, name := range names {
		records, err := ReadJSON[[]CostRecord](filepath.Join(dir, name+".json"))
		if err != nil {
			continue
		}
		var total float64
		for _, r := range records {
			total += r.CostUSD
		}
		stats[name] = total
	}
	return stats
}
