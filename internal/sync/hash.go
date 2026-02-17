package sync

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
)

// CalculateContentHash calculates SHA-256 hash of conversation content.
// This function MUST produce identical hashes to the Node.js implementation
// in packages/shared/src/utils/hash.ts
//
// The content should be in JSONL format (metadata line + message lines joined with \n)
func CalculateContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}

// CalculateFileHash calculates the hash for a conversation file.
// It reads the file, converts it to the same JSONL format as the server,
// and calculates the hash.
func CalculateFileHash(file FileInfo) (string, error) {
	f, err := os.Open(file.Path)
	if err != nil {
		return "", fmt.Errorf("opening file %s: %w", file.Path, err)
	}
	defer f.Close()

	// Read all messages
	var messages []Message
	scanner := bufio.NewScanner(f)
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Try parsing as Claude Code format first
		var ccMsg ClaudeCodeMessage
		if err := json.Unmarshal(line, &ccMsg); err == nil {
			if msg := ccMsg.ToMessage(); msg != nil && msg.UUID != "" && msg.Role != "" {
				messages = append(messages, *msg)
				continue
			}
		}

		// Fall back to legacy format
		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}

		if msg.UUID == "" || msg.Role == "" {
			continue
		}

		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("scanning file %s: %w", file.Path, err)
	}

	if len(messages) == 0 {
		return "", fmt.Errorf("no valid messages in file %s", file.Path)
	}

	// Build JSONL format matching the server implementation
	// Metadata line first, then messages
	metadata := map[string]interface{}{
		"sessionId":    file.SessionID,
		"userId":       "",  // Will be set by server
		"projectPath":  file.ProjectPath,
		"timestamp":    messages[0].Timestamp,
		"startTime":    messages[0].Timestamp,
		"endTime":      messages[len(messages)-1].Timestamp,
		"messageCount": len(messages),
		"models":       extractModels(messages),
		"totalTokens":  calculateTotalTokens(messages),
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshaling metadata: %w", err)
	}

	var lines []string
	lines = append(lines, string(metadataJSON))

	for _, msg := range messages {
		msgJSON, err := json.Marshal(msg)
		if err != nil {
			return "", fmt.Errorf("marshaling message: %w", err)
		}
		lines = append(lines, string(msgJSON))
	}

	// Join with \n (must match server implementation exactly)
	jsonl := ""
	for i, line := range lines {
		if i > 0 {
			jsonl += "\n"
		}
		jsonl += line
	}

	return CalculateContentHash(jsonl), nil
}

func extractModels(messages []Message) []string {
	modelSet := make(map[string]bool)
	for _, msg := range messages {
		if msg.Model != "" {
			modelSet[msg.Model] = true
		}
	}

	if len(modelSet) == 0 {
		return []string{"unknown"}
	}

	models := make([]string, 0, len(modelSet))
	for model := range modelSet {
		models = append(models, model)
	}
	return models
}

func calculateTotalTokens(messages []Message) int {
	total := 0
	for _, msg := range messages {
		total += msg.Tokens
	}
	return total
}

// ConversationNeedsSync determines if a conversation needs to be synced
// by comparing local and remote hashes
func ConversationNeedsSync(localHash, remoteHash string) bool {
	// If no remote hash exists, conversation is new and needs sync
	if remoteHash == "" {
		return true
	}

	// If hashes differ, conversation has changed and needs sync
	return localHash != remoteHash
}
