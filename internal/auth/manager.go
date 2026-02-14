package auth

import (
	"context"
	"fmt"
)

type Manager struct {
	config   *Config
	pkceFlow *PKCEFlow
	keychain *KeychainStore
}

func NewManager(config *Config) *Manager {
	return &Manager{
		config:   config,
		pkceFlow: NewPKCEFlow(config),
		keychain: NewKeychainStore(),
	}
}

// Login performs the OAuth PKCE flow login.
// It opens a browser for user authorization and starts a local callback server.
func (m *Manager) Login(ctx context.Context) error {
	tokenResp, err := m.pkceFlow.StartAuthFlow(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := m.keychain.SaveTokens(tokenResp.AccessToken, tokenResp); err != nil {
		return fmt.Errorf("saving tokens: %w", err)
	}

	fmt.Println("\n✅ Successfully authenticated!")
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

	tokenResp, err := m.pkceFlow.RefreshToken(ctx, refreshToken)
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
