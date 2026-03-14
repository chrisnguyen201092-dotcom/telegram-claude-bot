package store

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type Rule struct {
	Name      string `json:"name"`
	Content   string `json:"content"`
	IsActive  bool   `json:"is_active"`
	Priority  int    `json:"priority"`
	CreatedAt string `json:"created_at"`
}

func globalRulePath(name string) string {
	return filepath.Join(DataDir, "rules", "global", SafeFilename(name)+".json")
}

func userRulePath(telegramID, name string) string {
	return filepath.Join(DataDir, "rules", "users", telegramID, SafeFilename(name)+".json")
}

func AddGlobalRule(name, content string) (bool, error) {
	path := globalRulePath(name)
	if FileExists(path) {
		return false, nil
	}
	unlock := lockFile(path)
	defer unlock()
	rule := &Rule{
		Name:      name,
		Content:   content,
		IsActive:  true,
		CreatedAt: NowUTC(),
	}
	return true, WriteJSON(path, rule)
}

func AddUserRule(telegramID, name, content string) (bool, error) {
	path := userRulePath(telegramID, name)
	if FileExists(path) {
		return false, nil
	}
	unlock := lockFile(path)
	defer unlock()
	rule := &Rule{
		Name:      name,
		Content:   content,
		IsActive:  true,
		CreatedAt: NowUTC(),
	}
	return true, WriteJSON(path, rule)
}

func RemoveGlobalRule(name string) (bool, error) {
	path := globalRulePath(name)
	if !FileExists(path) {
		return false, nil
	}
	unlock := lockFile(path)
	defer unlock()
	return true, DeleteFile(path)
}

func RemoveUserRule(telegramID, name string) (bool, error) {
	path := userRulePath(telegramID, name)
	if !FileExists(path) {
		return false, nil
	}
	unlock := lockFile(path)
	defer unlock()
	return true, DeleteFile(path)
}

func ToggleUserRule(telegramID, name string) (found bool, isActive bool, err error) {
	path := userRulePath(telegramID, name)
	if !FileExists(path) {
		return false, false, nil
	}
	unlock := lockFile(path)
	defer unlock()
	rule, err := ReadJSON[Rule](path)
	if err != nil {
		return false, false, err
	}
	rule.IsActive = !rule.IsActive
	err = WriteJSON(path, rule)
	return true, rule.IsActive, err
}

func ListGlobalRules() ([]*Rule, error) {
	dir := filepath.Join(DataDir, "rules", "global")
	names, err := ListJSONFiles(dir)
	if err != nil {
		return nil, err
	}
	var rules []*Rule
	for _, name := range names {
		r, err := ReadJSON[Rule](filepath.Join(dir, name+".json"))
		if err != nil {
			continue
		}
		rules = append(rules, &r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})
	return rules, nil
}

func ListUserRules(telegramID string) ([]*Rule, error) {
	dir := filepath.Join(DataDir, "rules", "users", telegramID)
	names, err := ListJSONFiles(dir)
	if err != nil {
		return nil, err
	}
	var rules []*Rule
	for _, name := range names {
		r, err := ReadJSON[Rule](filepath.Join(dir, name+".json"))
		if err != nil {
			continue
		}
		rules = append(rules, &r)
	}
	sort.Slice(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})
	return rules, nil
}

func GetActiveRules(telegramID string) (global []*Rule, user []*Rule, err error) {
	allGlobal, err := ListGlobalRules()
	if err != nil {
		return nil, nil, err
	}
	for _, r := range allGlobal {
		if r.IsActive {
			global = append(global, r)
		}
	}
	allUser, err := ListUserRules(telegramID)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range allUser {
		if r.IsActive {
			user = append(user, r)
		}
	}
	return global, user, nil
}

func BuildSystemPrompt(telegramID string) (string, error) {
	var sb strings.Builder

	sb.WriteString("You are a Telegram bot assistant powered by Claude AI.\n")
	sb.WriteString("Keep responses concise and well-formatted. Use markdown where appropriate.\n")
	sb.WriteString("You are running inside Telegram, so avoid overly long responses.\n\n")

	sb.WriteString("## File & Image Delivery\n")
	sb.WriteString("You can send files and images directly in the chat. ")
	sb.WriteString("If you generate or have access to a file or image, you may deliver it directly to the user.\n\n")

	globalRules, userRules, err := GetActiveRules(telegramID)
	if err != nil {
		return "", fmt.Errorf("failed to get active rules: %w", err)
	}

	if len(globalRules) > 0 {
		sb.WriteString("## Global Rules\n")
		for _, r := range globalRules {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", r.Name, r.Content))
		}
		sb.WriteString("\n")
	}

	if len(userRules) > 0 {
		sb.WriteString("## Personal Rules\n")
		for _, r := range userRules {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", r.Name, r.Content))
		}
		sb.WriteString("\n")
	}

	memories, err := ListMemory(telegramID)
	if err == nil && len(memories) > 0 {
		sb.WriteString("## User Memories\n")
		for _, m := range memories {
			sb.WriteString(fmt.Sprintf("- **%s**: %s\n", m.Key, m.Value))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func FormatRuleList(rules []*Rule) string {
	if len(rules) == 0 {
		return "No rules found."
	}
	var sb strings.Builder
	for _, r := range rules {
		status := "on"
		if !r.IsActive {
			status = "off"
		}
		sb.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", r.Name, status, r.Content))
	}
	return sb.String()
}
