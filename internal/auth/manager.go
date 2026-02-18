package auth

import (
	"context"
	"fmt"
)

// AuthFlow interface for OAuth flows (to allow mocking in tests)
type AuthFlow interface {
	StartAuthFlow(ctx context.Context) (*TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error)
}

type Manager struct {
	config     *Config
	pkceFlow   AuthFlow
	tokenStore TokenStore
}

func NewManager(config *Config) *Manager {
	return &Manager{
		config:     config,
		pkceFlow:   NewPKCEFlow(config),
		tokenStore: NewTokenStore(), // Auto-detects tokenStore availability
	}
}

// NewManagerWithDeps creates a manager with injected dependencies (for testing)
func NewManagerWithDeps(config *Config, flow AuthFlow, store TokenStore) *Manager {
	return &Manager{
		config:     config,
		pkceFlow:   flow,
		tokenStore: store,
	}
}

// Login performs the OAuth PKCE flow login.
// It opens a browser for user authorization and starts a local callback server.
// If force is false, it will check for valid tokens first and skip re-authentication if they exist.
func (m *Manager) Login(ctx context.Context, force bool) error {
	// If not forcing re-authentication, check if we already have valid tokens
	if !force {
		if m.IsAuthenticated() {
			// Try to validate the token with a simple check
			_, err := m.GetValidToken(ctx)
			if err == nil {
				fmt.Println("✅ Already authenticated with valid tokens")
				fmt.Println("Use 'login --force' to re-authenticate")
				return nil
			}
			// Token validation failed, proceed with re-authentication
			fmt.Println("Existing tokens are invalid, re-authenticating...")
		}
	}

	tokenResp, err := m.pkceFlow.StartAuthFlow(ctx)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	if err := m.tokenStore.SaveTokens(tokenResp.AccessToken, tokenResp); err != nil {
		return fmt.Errorf("saving tokens: %w", err)
	}

	fmt.Println("\n✅ Successfully authenticated!")
	return nil
}

// GetValidToken returns a valid access token, refreshing if necessary.
func (m *Manager) GetValidToken(ctx context.Context) (string, error) {
	if !m.tokenStore.IsTokenExpired() {
		token, err := m.tokenStore.GetAccessToken()
		if err == nil {
			return token, nil
		}
	}

	// Try refresh
	refreshToken, err := m.tokenStore.GetRefreshToken()
	if err != nil {
		return "", fmt.Errorf("no valid token or refresh token available, please login again: %w", err)
	}

	tokenResp, err := m.pkceFlow.RefreshToken(ctx, refreshToken)
	if err != nil {
		// Refresh failed, need to re-login
		return "", fmt.Errorf("token refresh failed, please login again: %w", err)
	}

	if err := m.tokenStore.SaveTokens(tokenResp.AccessToken, tokenResp); err != nil {
		return "", fmt.Errorf("saving refreshed tokens: %w", err)
	}

	return tokenResp.AccessToken, nil
}

// Logout clears stored tokens.
func (m *Manager) Logout() error {
	return m.tokenStore.Clear()
}

// IsAuthenticated checks if there are stored, non-expired tokens.
func (m *Manager) IsAuthenticated() bool {
	_, err := m.tokenStore.GetAccessToken()
	if err != nil {
		return false
	}
	return !m.tokenStore.IsTokenExpired()
}
