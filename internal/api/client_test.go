package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSync_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/sync" {
			t.Errorf("expected /sync path, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Bearer test-token, got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Machine-ID") != "test-machine" {
			t.Errorf("expected X-Machine-ID test-machine, got %s", r.Header.Get("X-Machine-ID"))
		}

		var req SyncRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decoding request: %v", err)
		}

		if req.SessionID != "session-1" {
			t.Errorf("expected session-1, got %s", req.SessionID)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SyncResponse{
			Success:   true,
			Processed: len(req.Messages),
			SessionID: req.SessionID,
		})
	}))
	defer server.Close()

	tokenFunc := func(ctx context.Context) (string, error) {
		return "test-token", nil
	}

	client := NewClient(server.URL, "test-machine", tokenFunc)

	resp, err := client.Sync(context.Background(), &SyncRequest{
		MachineID:   "test-machine",
		SessionID:   "session-1",
		ProjectPath: "/test",
		Messages: []Message{
			{UUID: "msg-1", Role: "user", Content: "Hello"},
			{UUID: "msg-2", Role: "assistant", Content: "Hi"},
		},
		Timestamp: "2024-01-01T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Error("expected success")
	}
	if resp.Processed != 2 {
		t.Errorf("expected 2 processed, got %d", resp.Processed)
	}
}

func TestSync_ServerError_Retries(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error":"internal"}`))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SyncResponse{
			Success:   true,
			Processed: 1,
			SessionID: "session-1",
		})
	}))
	defer server.Close()

	tokenFunc := func(ctx context.Context) (string, error) {
		return "test-token", nil
	}

	client := NewClient(server.URL, "test-machine", tokenFunc)

	resp, err := client.Sync(context.Background(), &SyncRequest{
		SessionID: "session-1",
		Messages:  []Message{{UUID: "msg-1", Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.Success {
		t.Error("expected success after retries")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (2 retries + 1 success), got %d", callCount)
	}
}

func TestSync_ClientError_NoRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	tokenFunc := func(ctx context.Context) (string, error) {
		return "test-token", nil
	}

	client := NewClient(server.URL, "test-machine", tokenFunc)

	_, err := client.Sync(context.Background(), &SyncRequest{
		SessionID: "session-1",
		Messages:  []Message{{UUID: "msg-1", Role: "user", Content: "Hello"}},
	})
	if err == nil {
		t.Fatal("expected error for 400 response")
	}

	if callCount != 1 {
		t.Errorf("expected 1 call (no retries for 4xx), got %d", callCount)
	}

	httpErr, ok := err.(*HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if httpErr.StatusCode != 400 {
		t.Errorf("expected status 400, got %d", httpErr.StatusCode)
	}
}
