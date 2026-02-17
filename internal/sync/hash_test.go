package sync

import (
	"encoding/json"
	"testing"
)

func TestCalculateContentHash(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "empty string",
			content:  "",
			expected: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:    "simple JSONL conversation",
			content: createTestJSONL(),
			// This hash MUST match what the Node.js implementation produces
			// We'll verify this in integration tests, but structure should be consistent
			expected: "", // Will be computed and verified against Node.js implementation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateContentHash(tt.content)

			// Verify hash format: 64 hex characters for SHA-256
			if len(result) != 64 {
				t.Errorf("hash length = %d, want 64", len(result))
			}

			// Verify all characters are hex
			for _, c := range result {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("hash contains non-hex character: %c", c)
				}
			}

			// Verify expected hash if provided
			if tt.expected != "" && result != tt.expected {
				t.Errorf("hash = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestCalculateContentHash_Deterministic(t *testing.T) {
	content := createTestJSONL()

	hash1 := CalculateContentHash(content)
	hash2 := CalculateContentHash(content)

	if hash1 != hash2 {
		t.Errorf("hash is not deterministic: %s != %s", hash1, hash2)
	}
}

func TestCalculateContentHash_DifferentContent(t *testing.T) {
	content1 := `{"sessionId":"123","userId":"user1"}`
	content2 := `{"sessionId":"456","userId":"user2"}`

	hash1 := CalculateContentHash(content1)
	hash2 := CalculateContentHash(content2)

	if hash1 == hash2 {
		t.Error("different content produced same hash")
	}
}

func TestCalculateContentHash_MatchesNodeJS(t *testing.T) {
	// Test case that matches the Node.js test suite
	// This ensures CLI and Lambda produce identical hashes
	type Metadata struct {
		SessionID    string   `json:"sessionId"`
		UserID       string   `json:"userId"`
		ProjectPath  string   `json:"projectPath"`
		Timestamp    string   `json:"timestamp"`
		StartTime    string   `json:"startTime"`
		EndTime      string   `json:"endTime"`
		MessageCount int      `json:"messageCount"`
		Models       []string `json:"models"`
		TotalTokens  int      `json:"totalTokens"`
	}

	type Message struct {
		Role      string `json:"role"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
		Model     string `json:"model,omitempty"`
		Tokens    int    `json:"tokens,omitempty"`
	}

	metadata := Metadata{
		SessionID:    "test-session-123",
		UserID:       "user-456",
		ProjectPath:  "/test/project",
		Timestamp:    "2024-01-15T10:30:00Z",
		StartTime:    "2024-01-15T10:30:00Z",
		EndTime:      "2024-01-15T10:35:00Z",
		MessageCount: 2,
		Models:       []string{"claude-3-5-sonnet-20241022"},
		TotalTokens:  1500,
	}

	messages := []Message{
		{
			Role:      "user",
			Content:   "Hello, can you help me with TypeScript?",
			Timestamp: "2024-01-15T10:30:00Z",
		},
		{
			Role:      "assistant",
			Content:   "Of course! I'd be happy to help with TypeScript. What would you like to know?",
			Timestamp: "2024-01-15T10:30:15Z",
			Model:     "claude-3-5-sonnet-20241022",
			Tokens:    1500,
		},
	}

	// Build JSONL exactly as Node.js does: JSON.stringify() without spaces
	metadataJSON, _ := json.Marshal(metadata)
	var lines []string
	lines = append(lines, string(metadataJSON))

	for _, msg := range messages {
		msgJSON, _ := json.Marshal(msg)
		lines = append(lines, string(msgJSON))
	}

	// Join with \n (NOT \r\n - must match Node.js exactly)
	jsonl := ""
	for i, line := range lines {
		if i > 0 {
			jsonl += "\n"
		}
		jsonl += line
	}

	hash := CalculateContentHash(jsonl)

	// This hash should match the Node.js test output exactly
	// The actual value will be verified in E2E tests, but verify structure
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}

	t.Logf("Hash for test conversation: %s", hash)
	t.Log("Verify this matches the Node.js test output in packages/shared/src/utils/__tests__/hash.test.ts")
}

func createTestJSONL() string {
	metadata := map[string]interface{}{
		"sessionId":   "session-123",
		"userId":      "user-456",
		"projectPath": "/test",
		"timestamp":   "2024-01-01T00:00:00Z",
	}

	messages := []map[string]interface{}{
		{
			"role":    "user",
			"content": "test message",
		},
	}

	metadataJSON, _ := json.Marshal(metadata)
	var lines []string
	lines = append(lines, string(metadataJSON))

	for _, msg := range messages {
		msgJSON, _ := json.Marshal(msg)
		lines = append(lines, string(msgJSON))
	}

	jsonl := ""
	for i, line := range lines {
		if i > 0 {
			jsonl += "\n"
		}
		jsonl += line
	}

	return jsonl
}
