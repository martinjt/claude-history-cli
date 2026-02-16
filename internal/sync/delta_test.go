package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCalculateDelta_AllNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-session.jsonl")

	content := `{"uuid":"msg-1","timestamp":"2024-01-01T00:00:00Z","role":"user","content":"Hello","model":null,"tokens":null}
{"uuid":"msg-2","timestamp":"2024-01-01T00:01:00Z","role":"assistant","content":"Hi there","model":"claude-sonnet-4-5-20250929","tokens":42}
{"uuid":"msg-3","timestamp":"2024-01-01T00:02:00Z","role":"user","content":"Thanks","model":null,"tokens":null}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	file := FileInfo{
		Path:        path,
		SessionID:   "test-session",
		ProjectPath: "/test",
	}

	delta, err := CalculateDelta(file, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if delta == nil {
		t.Fatal("expected delta, got nil")
	}

	if len(delta.Messages) != 3 {
		t.Errorf("expected 3 messages, got %d", len(delta.Messages))
	}

	if delta.NewLastUUID != "msg-3" {
		t.Errorf("expected last UUID msg-3, got %s", delta.NewLastUUID)
	}
}

func TestCalculateDelta_Incremental(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-session.jsonl")

	content := `{"uuid":"msg-1","timestamp":"2024-01-01T00:00:00Z","role":"user","content":"Hello","model":null,"tokens":null}
{"uuid":"msg-2","timestamp":"2024-01-01T00:01:00Z","role":"assistant","content":"Hi there","model":"claude-sonnet-4-5-20250929","tokens":42}
{"uuid":"msg-3","timestamp":"2024-01-01T00:02:00Z","role":"user","content":"Thanks","model":null,"tokens":null}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	file := FileInfo{
		Path:        path,
		SessionID:   "test-session",
		ProjectPath: "/test",
	}

	delta, err := CalculateDelta(file, "msg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if delta == nil {
		t.Fatal("expected delta, got nil")
	}

	if len(delta.Messages) != 2 {
		t.Errorf("expected 2 new messages, got %d", len(delta.Messages))
	}

	if delta.Messages[0].UUID != "msg-2" {
		t.Errorf("expected first new message msg-2, got %s", delta.Messages[0].UUID)
	}
}

func TestCalculateDelta_NoNew(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-session.jsonl")

	content := `{"uuid":"msg-1","timestamp":"2024-01-01T00:00:00Z","role":"user","content":"Hello","model":null,"tokens":null}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	file := FileInfo{
		Path:        path,
		SessionID:   "test-session",
		ProjectPath: "/test",
	}

	delta, err := CalculateDelta(file, "msg-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if delta != nil {
		t.Error("expected nil delta for no new messages")
	}
}

func TestCalculateDelta_SkipsMalformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-session.jsonl")

	content := `{"uuid":"msg-1","timestamp":"2024-01-01T00:00:00Z","role":"user","content":"Hello","model":null,"tokens":null}
this is not valid json
{"uuid":"msg-2","timestamp":"2024-01-01T00:01:00Z","role":"assistant","content":"Hi","model":"claude-sonnet-4-5-20250929","tokens":10}
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}

	file := FileInfo{
		Path:        path,
		SessionID:   "test-session",
		ProjectPath: "/test",
	}

	delta, err := CalculateDelta(file, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if delta == nil {
		t.Fatal("expected delta, got nil")
	}

	if len(delta.Messages) != 2 {
		t.Errorf("expected 2 valid messages, got %d", len(delta.Messages))
	}
}

func TestExtractNewMessages(t *testing.T) {
	messages := []Message{
		{UUID: "a", Role: "user", Content: "Hello"},
		{UUID: "b", Role: "assistant", Content: "Hi"},
		{UUID: "c", Role: "user", Content: "Thanks"},
	}

	tests := []struct {
		name           string
		lastSyncedUUID string
		expectedCount  int
	}{
		{"all new when empty uuid", "", 3},
		{"from first", "a", 2},
		{"from second", "b", 1},
		{"none when last", "c", 0},
		{"all when uuid not found", "unknown", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractNewMessages(messages, tt.lastSyncedUUID)
			if len(result) != tt.expectedCount {
				t.Errorf("expected %d messages, got %d", tt.expectedCount, len(result))
			}
		})
	}
}
