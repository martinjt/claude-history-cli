package auth

import (
	"context"
	"fmt"
	"time"
)

type Manager struct {
	config     *Config
	deviceFlow *DeviceFlow
	keychain   *KeychainStore
}

func NewManager(config *Config) *Manager {
	return &Manager{
		config:     config,
		deviceFlow: NewDeviceFlow(config),
		keychain:   NewKeychainStore(),
	}
}

// Login performs the OAuth device flow login.
// It returns the verification URI and user code for display,
// then blocks until the user authorizes or context is cancelled.
func (m *Manager) Login(ctx context.Context) error {
	deviceResp, err := m.deviceFlow.RequestDeviceCode(ctx)
	if err != nil {
		return fmt.Errorf("requesting device code: %w", err)
	}

	fmt.Println("\nTo sign in, open the following URL in your browser:")
	fmt.Printf("\n  %s\n\n", deviceResp.VerificationURIComplete)
	fmt.Printf("Or go to %s and enter code: %s\n\n", deviceResp.VerificationURI, deviceResp.UserCode)
	fmt.Println("Waiting for authorization...")

	pollCtx, cancel := context.WithTimeout(ctx, time.Duration(deviceResp.ExpiresIn)*time.Second)
	defer cancel()

	tokenResp, err := m.deviceFlow.PollForToken(pollCtx, deviceResp.DeviceCode, deviceResp.Interval)
	if err != nil {
		return fmt.Errorf("waiting for authorization: %w", err)
	}

	if err := m.keychain.SaveTokens(tokenResp.AccessToken, tokenResp); err != nil {
		return fmt.Errorf("saving tokens: %w", err)
	}

	fmt.Println("Successfully authenticated!")
	return nil
}

// GetValidToken returns a valid access token, refreshing if necessary.
func (m *Manager) GetValidToken(ctx context.Context) (string, error) {
	if !m.keychain.IsTokenExpired() {
		token, err := m.keychain.GetAccessToken()
		if err == nil {
			return token, nil
		}
	}

	// Try refresh
	refreshToken, err := m.keychain.GetRefreshToken()
	if err != nil {
		return "", fmt.Errorf("no valid token or refresh token available, please login again: %w", err)
	}

	tokenResp, err := m.deviceFlow.RefreshToken(ctx, refreshToken)
	if err != nil {
		// Refresh failed, need to re-login
		return "", fmt.Errorf("token refresh failed, please login again: %w", err)
	}

	if err := m.keychain.SaveTokens(tokenResp.AccessToken, tokenResp); err != nil {
		return "", fmt.Errorf("saving refreshed tokens: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// Logout clears stored tokens.
func (m *Manager) Logout() error {
	return m.keychain.Clear()
}

// IsAuthenticated checks if there are stored, non-expired tokens.
func (m *Manager) IsAuthenticated() bool {
	_, err := m.keychain.GetAccessToken()
	if err != nil {
		return false
	}
	return !m.keychain.IsTokenExpired()
}
