package sync

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type Message struct {
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Model     string `json:"model,omitempty"`
	Tokens    int    `json:"tokens,omitempty"`
}

type Delta struct {
	SessionID   string
	ProjectPath string
	Messages    []Message
	NewLastUUID string
}

func CalculateDelta(file FileInfo, lastSyncedUUID string) (*Delta, error) {
	f, err := os.Open(file.Path)
	if err != nil {
		return nil, fmt.Errorf("opening file %s: %w", file.Path, err)
	}
	defer f.Close()

	var allMessages []Message
	scanner := bufio.NewScanner(f)
	// Increase buffer size for large messages (up to 1MB per line)
	buf := make([]byte, 0, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var msg Message
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip malformed lines
			continue
		}

		if msg.UUID == "" || msg.Role == "" {
			continue
		}

		allMessages = append(allMessages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning file %s: %w", file.Path, err)
	}

	// Find new messages after lastSyncedUUID
	newMessages := extractNewMessages(allMessages, lastSyncedUUID)

	if len(newMessages) == 0 {
		return nil, nil // No new messages
	}

	lastMsg := newMessages[len(newMessages)-1]

	return &Delta{
		SessionID:   file.SessionID,
		ProjectPath: file.ProjectPath,
		Messages:    newMessages,
		NewLastUUID: lastMsg.UUID,
	}, nil
}

func extractNewMessages(messages []Message, lastSyncedUUID string) []Message {
	if lastSyncedUUID == "" {
		return messages // All messages are new
	}

	for i, msg := range messages {
		if msg.UUID == lastSyncedUUID {
			if i+1 < len(messages) {
				return messages[i+1:]
			}
			return nil // Last synced was the last message
		}
	}

	// UUID not found, assume all are new (file was rewritten)
	return messages
}
