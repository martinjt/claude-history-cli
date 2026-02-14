package sync

import (
	"path/filepath"
	"testing"
)

func TestSyncState_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	state := &SyncState{
		Sessions: map[string]SessionState{
			"session-1": {
				LastSyncedUUID: "uuid-123",
				LastSyncAt:     "2024-01-01T00:00:00Z",
				MessageCount:   10,
			},
		},
	}

	if err := state.Save(path); err != nil {
		t.Fatalf("saving state: %v", err)
	}

	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("loading state: %v", err)
	}

	if len(loaded.Sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(loaded.Sessions))
	}

	sess := loaded.Sessions["session-1"]
	if sess.LastSyncedUUID != "uuid-123" {
		t.Errorf("expected uuid-123, got %s", sess.LastSyncedUUID)
	}
}

func TestSyncState_LoadNonExistent(t *testing.T) {
	state, err := LoadState("/nonexistent/path/state.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if state.Sessions == nil {
		t.Error("expected initialized sessions map")
	}
}

func TestSyncState_GetLastSyncedUUID(t *testing.T) {
	state := &SyncState{
		Sessions: map[string]SessionState{
			"session-1": {LastSyncedUUID: "uuid-abc"},
		},
	}

	if uuid := state.GetLastSyncedUUID("session-1"); uuid != "uuid-abc" {
		t.Errorf("expected uuid-abc, got %s", uuid)
	}

	if uuid := state.GetLastSyncedUUID("unknown"); uuid != "" {
		t.Errorf("expected empty string, got %s", uuid)
	}
}

func TestSyncState_UpdateSession(t *testing.T) {
	state := &SyncState{
		Sessions: make(map[string]SessionState),
	}

	state.UpdateSession("session-1", "uuid-new", 5)

	sess, ok := state.Sessions["session-1"]
	if !ok {
		t.Fatal("session not found")
	}

	if sess.LastSyncedUUID != "uuid-new" {
		t.Errorf("expected uuid-new, got %s", sess.LastSyncedUUID)
	}

	if sess.MessageCount != 5 {
		t.Errorf("expected message count 5, got %d", sess.MessageCount)
	}
}
