package auth

import (
	"fmt"
	"strings"
)

// TokenStore defines the interface for storing and retrieving tokens
type TokenStore interface {
	SaveTokens(accessToken string, resp *TokenResponse) error
	GetAccessToken() (string, error)
	GetTokenMeta() (*TokenMeta, error)
	IsTokenExpired() bool
	GetRefreshToken() (string, error)
	Clear() error
}

// NewTokenStore creates a token store with automatic fallback
// Tries keychain first, falls back to encrypted file storage if unavailable
func NewTokenStore() TokenStore {
	// Try keychain store first
	keychainStore := NewKeychainStore()

	// Test if keychain is available by trying to get a non-existent key
	// This will fail gracefully if DBus is unavailable
	_, err := keychainStore.GetAccessToken()

	// If error contains "dbus" or "session bus", keychain is unavailable
	if err != nil && (strings.Contains(err.Error(), "dbus") ||
	                  strings.Contains(err.Error(), "session bus") ||
	                  strings.Contains(err.Error(), "keyring") ||
	                  strings.Contains(err.Error(), "Secret Service")) {
		// Fallback to file storage
		return &FallbackStore{
			primary:   nil, // keychain unavailable
			secondary: NewFileStore(),
			usingFile: true,
		}
	}

	// Keychain is available, use it with file backup
	return &FallbackStore{
		primary:   keychainStore,
		secondary: NewFileStore(),
		usingFile: false,
	}
}

// FallbackStore tries keychain first, falls back to file storage
type FallbackStore struct {
	primary   TokenStore // keychain (may be nil if unavailable)
	secondary TokenStore // file storage
	usingFile bool       // true if keychain is unavailable
}

func (fs *FallbackStore) SaveTokens(accessToken string, resp *TokenResponse) error {
	// If keychain is available, try it first
	if fs.primary != nil {
		err := fs.primary.SaveTokens(accessToken, resp)
		if err == nil {
			// Success! Also save to file as backup
			_ = fs.secondary.SaveTokens(accessToken, resp)
			return nil
		}

		// Keychain failed, check if it's a DBus error
		if strings.Contains(err.Error(), "dbus") ||
		   strings.Contains(err.Error(), "session bus") {
			// Keychain no longer available, switch to file-only mode
			fs.primary = nil
			fs.usingFile = true
		}
	}

	// Use file storage (either fallback or primary if keychain unavailable)
	return fs.secondary.SaveTokens(accessToken, resp)
}

func (fs *FallbackStore) GetAccessToken() (string, error) {
	// Try primary (keychain) if available
	if fs.primary != nil {
		token, err := fs.primary.GetAccessToken()
		if err == nil {
			return token, nil
		}

		// Check for DBus errors
		if strings.Contains(err.Error(), "dbus") ||
		   strings.Contains(err.Error(), "session bus") {
			fs.primary = nil
			fs.usingFile = true
		}
	}

	// Fallback to secondary (file)
	return fs.secondary.GetAccessToken()
}

func (fs *FallbackStore) GetTokenMeta() (*TokenMeta, error) {
	if fs.primary != nil {
		meta, err := fs.primary.GetTokenMeta()
		if err == nil {
			return meta, nil
		}

		if strings.Contains(err.Error(), "dbus") ||
		   strings.Contains(err.Error(), "session bus") {
			fs.primary = nil
			fs.usingFile = true
		}
	}

	return fs.secondary.GetTokenMeta()
}

func (fs *FallbackStore) IsTokenExpired() bool {
	if fs.primary != nil && !fs.primary.IsTokenExpired() {
		return false
	}
	return fs.secondary.IsTokenExpired()
}

func (fs *FallbackStore) GetRefreshToken() (string, error) {
	if fs.primary != nil {
		token, err := fs.primary.GetRefreshToken()
		if err == nil {
			return token, nil
		}

		if strings.Contains(err.Error(), "dbus") ||
		   strings.Contains(err.Error(), "session bus") {
			fs.primary = nil
			fs.usingFile = true
		}
	}

	return fs.secondary.GetRefreshToken()
}

func (fs *FallbackStore) Clear() error {
	var errs []error

	if fs.primary != nil {
		if err := fs.primary.Clear(); err != nil {
			errs = append(errs, fmt.Errorf("clearing keychain: %w", err))
		}
	}

	if err := fs.secondary.Clear(); err != nil {
		errs = append(errs, fmt.Errorf("clearing file store: %w", err))
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors clearing tokens: %v", errs)
	}

	return nil
}
