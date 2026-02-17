package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"
)

type SyncRequest struct {
	MachineID   string    `json:"machineId"`
	SessionID   string    `json:"sessionId"`
	ProjectPath string    `json:"projectPath"`
	Messages    []Message `json:"messages"`
	Timestamp   string    `json:"timestamp"`
}

type Message struct {
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Model     string `json:"model,omitempty"`
	Tokens    int    `json:"tokens,omitempty"`
}

type SyncResponse struct {
	Success   bool   `json:"success"`
	Processed int    `json:"processed"`
	SessionID string `json:"sessionId"`
}

type Conversation struct {
	SessionID string `json:"sessionId"`
	Hash      string `json:"hash"`
	Date      string `json:"date"`
}

type ConversationsListResponse struct {
	Conversations []Conversation `json:"conversations"`
	Total         int            `json:"total"`
}

type Client struct {
	endpoint   string
	machineID  string
	httpClient *http.Client
	getToken   func(ctx context.Context) (string, error)
}

func NewClient(endpoint, machineID string, tokenFunc func(ctx context.Context) (string, error)) *Client {
	return &Client{
		endpoint:  endpoint,
		machineID: machineID,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		getToken: tokenFunc,
	}
}

func (c *Client) Sync(ctx context.Context, req *SyncRequest) (*SyncResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling sync request: %w", err)
	}

	var resp *SyncResponse
	err = c.doWithRetry(ctx, "POST", "/sync", body, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) GetConversations(ctx context.Context) (*ConversationsListResponse, error) {
	var resp *ConversationsListResponse
	err := c.doWithRetry(ctx, "GET", "/conversations", nil, &resp)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) doWithRetry(ctx context.Context, method, path string, body []byte, result interface{}) error {
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * time.Second
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		err := c.doRequest(ctx, method, path, body, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Only retry on 429 and 5xx
		if httpErr, ok := err.(*HTTPError); ok {
			if httpErr.StatusCode == 429 || httpErr.StatusCode >= 500 {
				continue
			}
			return err // Non-retryable error
		}

		return err // Non-HTTP error
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte, result interface{}) error {
	url := c.endpoint + path

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Get OAuth token
	token, err := c.getToken(ctx)
	if err != nil {
		return fmt.Errorf("getting auth token: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-Machine-ID", c.machineID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
		}
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("parsing response: %w", err)
		}
	}

	return nil
}

type HTTPError struct {
	StatusCode int
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Body)
}
