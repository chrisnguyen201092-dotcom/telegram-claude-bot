package store

import (
	"os"
	"path/filepath"
)

type User struct {
	TelegramID       string `json:"telegram_id"`
	Username         string `json:"username"`
	DisplayName      string `json:"display_name"`
	Role             string `json:"role"`
	IsWhitelisted    bool   `json:"is_whitelisted"`
	WorkingDirectory string `json:"working_directory"`
	CreatedAt        string `json:"created_at"`
}

func userPath(telegramID string) string {
	return filepath.Join(DataDir, "users", telegramID+".json")
}

func GetUser(telegramID string) (*User, error) {
	u, err := ReadJSON[User](userPath(telegramID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func CreateUser(user *User) error {
	unlock := lockFile(userPath(user.TelegramID))
	defer unlock()
	if user.CreatedAt == "" {
		user.CreatedAt = NowUTC()
	}
	return WriteJSON(userPath(user.TelegramID), user)
}

func UpdateUser(user *User) error {
	unlock := lockFile(userPath(user.TelegramID))
	defer unlock()
	return WriteJSON(userPath(user.TelegramID), user)
}

func SetWhitelist(telegramID string, whitelisted bool) error {
	unlock := lockFile(userPath(telegramID))
	defer unlock()
	u, err := ReadJSON[User](userPath(telegramID))
	if err != nil {
		return err
	}
	u.IsWhitelisted = whitelisted
	return WriteJSON(userPath(telegramID), u)
}

func SetWorkingDir(telegramID, dir string) error {
	unlock := lockFile(userPath(telegramID))
	defer unlock()
	u, err := ReadJSON[User](userPath(telegramID))
	if err != nil {
		return err
	}
	u.WorkingDirectory = dir
	return WriteJSON(userPath(telegramID), u)
}

func ListAllUsers() ([]*User, error) {
	names, err := ListJSONFiles(filepath.Join(DataDir, "users"))
	if err != nil {
		return nil, err
	}
	var users []*User
	for _, name := range names {
		u, err := ReadJSON[User](filepath.Join(DataDir, "users", name+".json"))
		if err != nil {
			continue
		}
		users = append(users, &u)
	}
	return users, nil
}

func DeleteUser(telegramID string) error {
	unlock := lockFile(userPath(telegramID))
	defer unlock()
	return DeleteFile(userPath(telegramID))
}
