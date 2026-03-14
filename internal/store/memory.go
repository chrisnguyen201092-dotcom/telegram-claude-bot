package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Memory struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Category  string `json:"category"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func memoryPath(telegramID, key string) string {
	return filepath.Join(DataDir, "memory", telegramID, SafeFilename(key)+".json")
}

func SetMemory(telegramID, key, value, category string) error {
	path := memoryPath(telegramID, key)
	unlock := lockFile(path)
	defer unlock()

	var m Memory
	existing, err := ReadJSON[Memory](path)
	if err == nil {
		m = existing
	} else {
		m = Memory{
			Key:       key,
			Category:  category,
			CreatedAt: NowUTC(),
		}
	}
	m.Value = value
	if category != "" {
		m.Category = category
	}
	m.UpdatedAt = NowUTC()
	return WriteJSON(path, m)
}

func GetMemory(telegramID, key string) (*Memory, error) {
	m, err := ReadJSON[Memory](memoryPath(telegramID, key))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &m, nil
}

func ListMemory(telegramID string) ([]*Memory, error) {
	dir := filepath.Join(DataDir, "memory", telegramID)
	names, err := ListJSONFiles(dir)
	if err != nil {
		return nil, err
	}
	var memories []*Memory
	for _, name := range names {
		m, err := ReadJSON[Memory](filepath.Join(dir, name+".json"))
		if err != nil {
			continue
		}
		memories = append(memories, &m)
	}
	return memories, nil
}

func DeleteMemory(telegramID, key string) (bool, error) {
	path := memoryPath(telegramID, key)
	if !FileExists(path) {
		return false, nil
	}
	unlock := lockFile(path)
	defer unlock()
	return true, DeleteFile(path)
}

func ClearMemory(telegramID string) (int, error) {
	dir := filepath.Join(DataDir, "memory", telegramID)
	names, err := ListJSONFiles(dir)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, name := range names {
		path := filepath.Join(dir, name+".json")
		unlock := lockFile(path)
		if err := DeleteFile(path); err == nil {
			count++
		}
		unlock()
	}
	return count, nil
}

func FormatMemoryList(memories []*Memory) string {
	if len(memories) == 0 {
		return "No memories found."
	}
	var sb strings.Builder
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", m.Key, m.Value))
	}
	return sb.String()
}

func SearchMemory(telegramID, query string) ([]*Memory, error) {
	all, err := ListMemory(telegramID)
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var results []*Memory
	for _, m := range all {
		if strings.Contains(strings.ToLower(m.Key), q) ||
			strings.Contains(strings.ToLower(m.Value), q) {
			results = append(results, m)
		}
	}
	return results, nil
}
