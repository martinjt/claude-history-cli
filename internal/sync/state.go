package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SyncState struct {
	Sessions   map[string]SessionState `json:"sessions"`
	LastSyncAt string                  `json:"last_sync_at"`
}

type SessionState struct {
	LastSyncedUUID string `json:"last_synced_uuid"`
	LastSyncAt     string `json:"last_sync_at"`
	MessageCount   int    `json:"message_count"`
}

func DefaultStatePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude-history-sync/state.json"
	}
	return filepath.Join(home, ".claude-history-sync", "state.json")
}

func LoadState(path string) (*SyncState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncState{
				Sessions: make(map[string]SessionState),
			}, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var state SyncState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parsing state file: %w", err)
	}

	if state.Sessions == nil {
		state.Sessions = make(map[string]SessionState)
	}

	return &state, nil
}

func (s *SyncState) Save(path string) error {
	s.LastSyncAt = time.Now().UTC().Format(time.RFC3339)

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating state directory: %w", err)
	}

	// Atomic write: write to temp file then rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("writing temp state file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming state file: %w", err)
	}

	return nil
}

func (s *SyncState) GetLastSyncedUUID(sessionID string) string {
	if session, ok := s.Sessions[sessionID]; ok {
		return session.LastSyncedUUID
	}
	return ""
}

func (s *SyncState) UpdateSession(sessionID, lastUUID string, messageCount int) {
	s.Sessions[sessionID] = SessionState{
		LastSyncedUUID: lastUUID,
		LastSyncAt:     time.Now().UTC().Format(time.RFC3339),
		MessageCount:   messageCount,
	}
}
