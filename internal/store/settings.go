package store

import (
	"fmt"
	"os"
	"path/filepath"
)

type UserSettings struct {
	Model        string   `json:"model,omitempty"`
	Effort       string   `json:"effort,omitempty"`
	Thinking     string   `json:"thinking,omitempty"`
	MaxTurns     *int     `json:"max_turns,omitempty"`
	MaxBudgetUSD *float64 `json:"max_budget_usd,omitempty"`
	UpdatedAt    string   `json:"updated_at,omitempty"`
}

type EffectiveSettings struct {
	Model        string
	Effort       string
	Thinking     string
	MaxTurns     *int
	MaxBudgetUSD *float64
}

type ModelInfo struct {
	ID    string
	Label string
}

var AvailableModels = []ModelInfo{
	{ID: "claude-sonnet-4-6", Label: "Sonnet 4.6"},
	{ID: "claude-opus-4-6", Label: "Opus 4.6"},
	{ID: "claude-haiku-4-5", Label: "Haiku 4.5"},
}

var EffortLevels = []string{"low", "medium", "high", "max"}
var ThinkingModes = []string{"off", "adaptive", "on"}

func settingsPath(telegramID string) string {
	return filepath.Join(DataDir, "settings", telegramID+".json")
}

func GetSettings(telegramID string) (*UserSettings, error) {
	s, err := ReadJSON[UserSettings](settingsPath(telegramID))
	if err != nil {
		if os.IsNotExist(err) {
			return &UserSettings{}, nil
		}
		return nil, err
	}
	return &s, nil
}

func UpsertSettings(telegramID string, s *UserSettings) error {
	unlock := lockFile(settingsPath(telegramID))
	defer unlock()
	s.UpdatedAt = NowUTC()
	return WriteJSON(settingsPath(telegramID), s)
}

func ResolveModelAlias(input string) string {
	aliases := map[string]string{
		"sonnet": "claude-sonnet-4-6",
		"opus":   "claude-opus-4-6",
		"haiku":  "claude-haiku-4-5",
	}
	if id, ok := aliases[input]; ok {
		return id
	}
	for _, m := range AvailableModels {
		if m.ID == input {
			return input
		}
	}
	return ""
}

func GetEffectiveSettings(telegramID string, cfg *GlobalConfig) *EffectiveSettings {
	s, _ := GetSettings(telegramID)
	if s == nil {
		s = &UserSettings{}
	}

	effective := &EffectiveSettings{
		Model:        cfg.DefaultModel,
		Effort:       cfg.DefaultEffort,
		Thinking:     cfg.DefaultThinking,
		MaxTurns:     s.MaxTurns,
		MaxBudgetUSD: s.MaxBudgetUSD,
	}
	if s.Model != "" {
		effective.Model = s.Model
	}
	if s.Effort != "" {
		effective.Effort = s.Effort
	}
	if s.Thinking != "" {
		effective.Thinking = s.Thinking
	}
	return effective
}

func FormatSettings(s *EffectiveSettings) string {
	maxTurns := "default"
	if s.MaxTurns != nil {
		maxTurns = fmt.Sprintf("%d", *s.MaxTurns)
	}
	maxBudget := "default"
	if s.MaxBudgetUSD != nil {
		maxBudget = fmt.Sprintf("$%.2f", *s.MaxBudgetUSD)
	}
	return fmt.Sprintf(
		"Model: %s\nEffort: %s\nThinking: %s\nMax Turns: %s\nMax Budget: %s",
		s.Model, s.Effort, s.Thinking, maxTurns, maxBudget,
	)
}
