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
	Type      string `json:"type,omitempty"`
	Tokens    int    `json:"tokens,omitempty"`
}

// ClaudeCodeMessage represents the actual format from Claude Code conversation files
type ClaudeCodeMessage struct {
	UUID      string                 `json:"uuid"`
	Timestamp string                 `json:"timestamp"`
	Type      string                 `json:"type"`
	Message   map[string]interface{} `json:"message"`
}

// ToMessage converts ClaudeCodeMessage to our simplified Message format
func (ccm *ClaudeCodeMessage) ToMessage() *Message {
	if ccm.UUID == "" || ccm.Message == nil {
		return nil
	}

	msg := &Message{
		UUID:      ccm.UUID,
		Timestamp: ccm.Timestamp,
		Type:      ccm.Type,
	}

	// Extract role
	if role, ok := ccm.Message["role"].(string); ok {
		msg.Role = role
	}

	// Extract content (can be string or array)
	if content, ok := ccm.Message["content"].(string); ok {
		msg.Content = content
	} else if contentArray, ok := ccm.Message["content"].([]interface{}); ok {
		// For assistant messages with structured content, extract text parts
		var textParts []string
		for _, item := range contentArray {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if itemType, ok := itemMap["type"].(string); ok && itemType == "text" {
					if text, ok := itemMap["text"].(string); ok {
						textParts = append(textParts, text)
					}
				}
			}
		}
		if len(textParts) > 0 {
			msg.Content = textParts[0] // Use first text part
		}
	}

	// Extract model
	if model, ok := ccm.Message["model"].(string); ok {
		msg.Model = model
	}

	return msg
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
	// Increase buffer size for large messages (up to 10MB per line for images/tool results)
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
				allMessages = append(allMessages, *msg)
				continue
			}
		}

		// Fall back to legacy format for backwards compatibility
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
