package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

// MockTokenStore for testing
type MockTokenStore struct {
	accessToken  string
	refreshToken string
	tokenMeta    *TokenMeta
	hasTokens    bool
	isExpired    bool
}

func (m *MockTokenStore) SaveTokens(accessToken string, resp *TokenResponse) error {
	m.accessToken = accessToken
	m.refreshToken = resp.RefreshToken
	m.hasTokens = true
	m.isExpired = false
	m.tokenMeta = &TokenMeta{
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	return nil
}

func (m *MockTokenStore) GetAccessToken() (string, error) {
	if !m.hasTokens {
		return "", errors.New("no tokens stored")
	}
	return m.accessToken, nil
}

func (m *MockTokenStore) GetTokenMeta() (*TokenMeta, error) {
	if !m.hasTokens {
		return nil, errors.New("no tokens stored")
	}
	return m.tokenMeta, nil
}

func (m *MockTokenStore) IsTokenExpired() bool {
	return m.isExpired
}

func (m *MockTokenStore) GetRefreshToken() (string, error) {
	if !m.hasTokens {
		return "", errors.New("no tokens stored")
	}
	return m.refreshToken, nil
}

func (m *MockTokenStore) Clear() error {
	m.hasTokens = false
	m.accessToken = ""
	m.refreshToken = ""
	m.tokenMeta = nil
	return nil
}

// MockPKCEFlow for testing
type MockPKCEFlow struct {
	shouldFail bool
	callCount  int
}

func (m *MockPKCEFlow) StartAuthFlow(ctx context.Context) (*TokenResponse, error) {
	m.callCount++
	if m.shouldFail {
		return nil, errors.New("auth flow failed")
	}
	return &TokenResponse{
		AccessToken:  "new-access-token",
		RefreshToken: "new-refresh-token",
		ExpiresIn:    3600,
	}, nil
}

func (m *MockPKCEFlow) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	if m.shouldFail {
		return nil, errors.New("refresh failed")
	}
	return &TokenResponse{
		AccessToken:  "refreshed-access-token",
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}, nil
}

func TestLogin_WithValidTokens_SkipsReauth(t *testing.T) {
	mockStore := &MockTokenStore{
		hasTokens: true,
		accessToken: "existing-token",
		refreshToken: "existing-refresh",
		isExpired: false,
		tokenMeta: &TokenMeta{
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		},
	}
	mockFlow := &MockPKCEFlow{}

	manager := NewManagerWithDeps(&Config{}, mockFlow, mockStore)

	// Login without force - should skip re-auth
	err := manager.Login(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify auth flow was not called
	if mockFlow.callCount != 0 {
		t.Errorf("expected auth flow not to be called, but was called %d times", mockFlow.callCount)
	}
}

func TestLogin_WithForceFlag_AlwaysReauths(t *testing.T) {
	mockStore := &MockTokenStore{
		hasTokens: true,
		accessToken: "existing-token",
		refreshToken: "existing-refresh",
		isExpired: false,
		tokenMeta: &TokenMeta{
			ExpiresAt: time.Now().Add(time.Hour).Unix(),
		},
	}
	mockFlow := &MockPKCEFlow{}

	manager := NewManagerWithDeps(&Config{}, mockFlow, mockStore)

	// Login with force - should always re-auth
	err := manager.Login(context.Background(), true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify auth flow was called
	if mockFlow.callCount != 1 {
		t.Errorf("expected auth flow to be called once, but was called %d times", mockFlow.callCount)
	}

	// Verify new tokens were saved
	if mockStore.accessToken != "new-access-token" {
		t.Errorf("expected new access token to be saved, got %s", mockStore.accessToken)
	}
}

func TestLogin_WithExpiredTokens_Reauths(t *testing.T) {
	mockStore := &MockTokenStore{
		hasTokens: true,
		accessToken: "expired-token",
		refreshToken: "expired-refresh",
		isExpired: true,
		tokenMeta: &TokenMeta{
			ExpiresAt: time.Now().Add(-time.Hour).Unix(), // Expired
		},
	}
	mockFlow := &MockPKCEFlow{}

	manager := NewManagerWithDeps(&Config{}, mockFlow, mockStore)

	// Login without force - should re-auth because tokens are expired
	err := manager.Login(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify auth flow was called
	if mockFlow.callCount != 1 {
		t.Errorf("expected auth flow to be called once, but was called %d times", mockFlow.callCount)
	}
}

func TestLogin_WithNoTokens_Reauths(t *testing.T) {
	mockStore := &MockTokenStore{
		hasTokens: false,
	}
	mockFlow := &MockPKCEFlow{}

	manager := NewManagerWithDeps(&Config{}, mockFlow, mockStore)

	// Login without force - should re-auth because no tokens exist
	err := manager.Login(context.Background(), false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify auth flow was called
	if mockFlow.callCount != 1 {
		t.Errorf("expected auth flow to be called once, but was called %d times", mockFlow.callCount)
	}
}

func TestIsAuthenticated_WithValidTokens_ReturnsTrue(t *testing.T) {
	mockStore := &MockTokenStore{
		hasTokens: true,
		accessToken: "valid-token",
		isExpired: false,
	}

	manager := NewManagerWithDeps(&Config{}, &MockPKCEFlow{}, mockStore)

	if !manager.IsAuthenticated() {
		t.Error("expected IsAuthenticated to return true with valid tokens")
	}
}

func TestIsAuthenticated_WithExpiredTokens_ReturnsFalse(t *testing.T) {
	mockStore := &MockTokenStore{
		hasTokens: true,
		accessToken: "expired-token",
		isExpired: true,
	}

	manager := NewManagerWithDeps(&Config{}, &MockPKCEFlow{}, mockStore)

	if manager.IsAuthenticated() {
		t.Error("expected IsAuthenticated to return false with expired tokens")
	}
}

func TestIsAuthenticated_WithNoTokens_ReturnsFalse(t *testing.T) {
	mockStore := &MockTokenStore{
		hasTokens: false,
	}

	manager := NewManagerWithDeps(&Config{}, &MockPKCEFlow{}, mockStore)

	if manager.IsAuthenticated() {
		t.Error("expected IsAuthenticated to return false with no tokens")
	}
}
