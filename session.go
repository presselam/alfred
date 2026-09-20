package main

import (
	"encoding/gob"
	"os"
	"path/filepath"
	"time"
)

// sessionsDir returns the directory where chat session files are stored,
// creating it if necessary.
func sessionsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".alfred", "sessions")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// sessionPath returns the file path for a given chat id.
func sessionPath(chatID string) (string, error) {
	dir, err := sessionsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, filepath.Base(chatID)+".gob"), nil
}

// sessionData is what gets persisted to disk for a chat session.
type sessionData struct {
	Provider provider
	Messages []message
}

// loadSession reads a previously saved session for chatID. If no session
// file exists, it returns an error satisfying os.IsNotExist - callers
// decide whether that's fine (a fresh, unused chat id) or fatal (an
// explicitly requested --id that doesn't exist).
func loadSession(chatID string) (sessionData, error) {
	path, err := sessionPath(chatID)
	if err != nil {
		return sessionData{}, err
	}
	f, err := os.Open(path)
	if err != nil {
		return sessionData{}, err
	}
	defer f.Close()
	var data sessionData
	if err := gob.NewDecoder(f).Decode(&data); err != nil {
		return sessionData{}, err
	}
	return data, nil
}

// cleanupOldSessions removes session files under sessionsDir that haven't
// been modified in more than retentionDays days, best-effort. keepChatID -
// the session about to be used for this run - is never deleted, regardless
// of its age. retentionDays <= 0 disables cleanup entirely (keep forever).
func cleanupOldSessions(retentionDays int, keepChatID string) {
	if retentionDays <= 0 {
		return
	}
	dir, err := sessionsDir()
	if err != nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	keepFile := filepath.Base(keepChatID) + ".gob"
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == keepFile {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// saveSession persists the chat session to disk in gob's binary encoding,
// best-effort.
func (m *model) saveSession() {
	path, err := sessionPath(m.chatID)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_ = gob.NewEncoder(f).Encode(sessionData{Provider: m.provider, Messages: m.messages})
}
