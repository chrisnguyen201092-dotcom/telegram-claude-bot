package store

import (
	"path/filepath"
)

func sessionPath(telegramID, sessionID string) string {
	return filepath.Join(DataDir, "sessions", telegramID, sessionID+".md")
}

// GetSessionPath returns the filesystem path for a session file (exported for use by other packages).
func GetSessionPath(telegramID, sessionID string) string {
	return sessionPath(telegramID, sessionID)
}

func GetActiveSession(telegramID string) (*SessionMeta, error) {
	dir := filepath.Join(DataDir, "sessions", telegramID)
	names, err := ListMDFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		path := filepath.Join(dir, name+".md")
		meta, _, err := ParseSessionMD(path)
		if err != nil {
			continue
		}
		if meta.IsActive {
			return meta, nil
		}
	}
	return nil, nil
}

func SaveSession(meta *SessionMeta) error {
	path := sessionPath(meta.TelegramID, meta.SessionID)
	if FileExists(path) {
		return UpdateSessionFrontmatter(path, meta)
	}
	return WriteSessionMD(path, meta, nil)
}

func GetSessionForDir(telegramID, workingDir string) (*SessionMeta, error) {
	dir := filepath.Join(DataDir, "sessions", telegramID)
	names, err := ListMDFiles(dir)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		path := filepath.Join(dir, name+".md")
		meta, _, err := ParseSessionMD(path)
		if err != nil {
			continue
		}
		if meta.IsActive && meta.WorkingDir == workingDir {
			return meta, nil
		}
	}
	return nil, nil
}

func ListSessions(telegramID string) ([]*SessionMeta, error) {
	dir := filepath.Join(DataDir, "sessions", telegramID)
	names, err := ListMDFiles(dir)
	if err != nil {
		return nil, err
	}
	var sessions []*SessionMeta
	for _, name := range names {
		path := filepath.Join(dir, name+".md")
		meta, _, err := ParseSessionMD(path)
		if err != nil {
			continue
		}
		sessions = append(sessions, meta)
	}
	return sessions, nil
}

func SwitchSession(telegramID, sessionID string) error {
	dir := filepath.Join(DataDir, "sessions", telegramID)
	names, err := ListMDFiles(dir)
	if err != nil {
		return err
	}
	for _, name := range names {
		path := filepath.Join(dir, name+".md")
		meta, _, err := ParseSessionMD(path)
		if err != nil {
			continue
		}
		wasActive := meta.IsActive
		meta.IsActive = (meta.SessionID == sessionID)
		if wasActive != meta.IsActive {
			if err := UpdateSessionFrontmatter(path, meta); err != nil {
				return err
			}
		}
	}
	return nil
}

func DeactivateSession(telegramID string) error {
	dir := filepath.Join(DataDir, "sessions", telegramID)
	names, err := ListMDFiles(dir)
	if err != nil {
		return err
	}
	for _, name := range names {
		path := filepath.Join(dir, name+".md")
		meta, _, err := ParseSessionMD(path)
		if err != nil {
			continue
		}
		if meta.IsActive {
			meta.IsActive = false
			if err := UpdateSessionFrontmatter(path, meta); err != nil {
				return err
			}
		}
	}
	return nil
}

func DeleteSession(telegramID, sessionID string) error {
	path := sessionPath(telegramID, sessionID)
	unlock := lockFile(path)
	defer unlock()
	return DeleteFile(path)
}

func GetSessionMessages(telegramID, sessionID string) ([]SessionMessage, error) {
	path := sessionPath(telegramID, sessionID)
	_, messages, err := ParseSessionMD(path)
	if err != nil {
		return nil, err
	}
	return messages, nil
}

func GetSessionMessageCount(telegramID, sessionID string) (int, error) {
	messages, err := GetSessionMessages(telegramID, sessionID)
	if err != nil {
		return 0, err
	}
	return len(messages), nil
}

func UpdateSessionLastUsed(telegramID, sessionID string) error {
	path := sessionPath(telegramID, sessionID)
	meta, _, err := ParseSessionMD(path)
	if err != nil {
		return err
	}
	meta.LastUsed = NowUTC()
	return UpdateSessionFrontmatter(path, meta)
}
