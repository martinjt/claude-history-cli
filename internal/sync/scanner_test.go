package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanForJSONL(t *testing.T) {
	dir := t.TempDir()

	// Create test structure
	projectDir := filepath.Join(dir, "my-project")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create JSONL files
	if err := os.WriteFile(filepath.Join(projectDir, "session1.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "session2.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create non-JSONL file (should be excluded)
	if err := os.WriteFile(filepath.Join(projectDir, "notes.txt"), []byte("text"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ScanForJSONL(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("expected 2 JSONL files, got %d", len(files))
	}
}

func TestScanForJSONL_WithExclude(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "keep.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exclude-me.jsonl"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	files, err := ScanForJSONL(dir, []string{"exclude-me*"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("expected 1 file, got %d", len(files))
	}
}

func TestExtractProjectPath(t *testing.T) {
	tests := []struct {
		relPath  string
		expected string
	}{
		{"session.jsonl", "/"},
		{"my-project/session.jsonl", "/my-project"},
		{"org/project/session.jsonl", "/org/project"},
	}

	for _, tt := range tests {
		result := extractProjectPath(tt.relPath)
		if result != tt.expected {
			t.Errorf("extractProjectPath(%q) = %q, want %q", tt.relPath, result, tt.expected)
		}
	}
}

func TestExtractSessionID(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"abc-123.jsonl", "abc-123"},
		{"my-session.jsonl", "my-session"},
	}

	for _, tt := range tests {
		result := extractSessionID(tt.filename)
		if result != tt.expected {
			t.Errorf("extractSessionID(%q) = %q, want %q", tt.filename, result, tt.expected)
		}
	}
}
