package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestDeviceCode(t *testing.T) {
	expected := DeviceFlowResponse{
		DeviceCode:              "test-device-code",
		UserCode:                "ABCD-1234",
		VerificationURI:         "https://example.com/device",
		VerificationURIComplete: "https://example.com/device?user_code=ABCD-1234",
		ExpiresIn:               1800,
		Interval:                5,
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		contentType := r.Header.Get("Content-Type")
		if contentType != "application/x-www-form-urlencoded" {
			t.Errorf("expected application/x-www-form-urlencoded, got %s", contentType)
		}

		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}

		if r.Form.Get("client_id") != "test-client" {
			t.Errorf("expected client_id=test-client, got %s", r.Form.Get("client_id"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	config := &Config{
		ClientID:      "test-client",
		Scopes:        []string{"openid", "email"},
		DeviceFlowURL: server.URL,
		TokenURL:      server.URL + "/token",
	}

	df := NewDeviceFlow(config)
	resp, err := df.RequestDeviceCode(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.DeviceCode != expected.DeviceCode {
		t.Errorf("expected device code %s, got %s", expected.DeviceCode, resp.DeviceCode)
	}
	if resp.UserCode != expected.UserCode {
		t.Errorf("expected user code %s, got %s", expected.UserCode, resp.UserCode)
	}
}

func TestPollForToken_Success(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount < 3 {
			json.NewEncoder(w).Encode(TokenResponse{
				Error:     "authorization_pending",
				ErrorDesc: "The authorization request is still pending",
			})
			return
		}

		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken:  "test-access-token",
			RefreshToken: "test-refresh-token",
			IDToken:      "test-id-token",
			TokenType:    "Bearer",
			ExpiresIn:    3600,
		})
	}))
	defer server.Close()

	config := &Config{
		ClientID: "test-client",
		TokenURL: server.URL,
	}

	df := NewDeviceFlow(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use minimum interval for testing
	resp, err := df.PollForToken(ctx, "test-device-code", 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.AccessToken != "test-access-token" {
		t.Errorf("expected access token test-access-token, got %s", resp.AccessToken)
	}
	if resp.RefreshToken != "test-refresh-token" {
		t.Errorf("expected refresh token test-refresh-token, got %s", resp.RefreshToken)
	}
}

func TestPollForToken_Denied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			Error:     "access_denied",
			ErrorDesc: "The user denied the authorization request",
		})
	}))
	defer server.Close()

	config := &Config{
		ClientID: "test-client",
		TokenURL: server.URL,
	}

	df := NewDeviceFlow(config)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := df.PollForToken(ctx, "test-device-code", 1)
	if err == nil {
		t.Fatal("expected error for denied authorization")
	}
}

func TestRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("parsing form: %v", err)
		}

		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("expected grant_type=refresh_token, got %s", r.Form.Get("grant_type"))
		}

		if r.Form.Get("refresh_token") != "test-refresh" {
			t.Errorf("expected refresh_token=test-refresh, got %s", r.Form.Get("refresh_token"))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "new-access-token",
			TokenType:   "Bearer",
			ExpiresIn:   3600,
		})
	}))
	defer server.Close()

	config := &Config{
		ClientID: "test-client",
		TokenURL: server.URL,
	}

	df := NewDeviceFlow(config)
	resp, err := df.RefreshToken(context.Background(), "test-refresh")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.AccessToken != "new-access-token" {
		t.Errorf("expected new-access-token, got %s", resp.AccessToken)
	}
}
