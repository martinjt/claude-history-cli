package auth

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	keychainService = "claude-history-mcp"
	accessTokenKey  = "access_token"
	refreshTokenKey = "refresh_token"
	tokenMetaKey    = "token_meta"
)

type TokenMeta struct {
	ExpiresAt    int64  `json:"expires_at"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}

type KeychainStore struct {
	serviceName string
}

func NewKeychainStore() *KeychainStore {
	return &KeychainStore{
		serviceName: keychainService,
	}
}

func (ks *KeychainStore) SaveTokens(accessToken string, resp *TokenResponse) error {
	if err := keyring.Set(ks.serviceName, accessTokenKey, accessToken); err != nil {
		return fmt.Errorf("saving access token to keychain: %w", err)
	}

	meta := TokenMeta{
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second).Unix(),
		RefreshToken: resp.RefreshToken,
		IDToken:      resp.IDToken,
	}

	metaJSON, err := json.Marshal(meta)
	if err != nil {
		return fmt.Errorf("marshaling token meta: %w", err)
	}

	if err := keyring.Set(ks.serviceName, tokenMetaKey, string(metaJSON)); err != nil {
		return fmt.Errorf("saving token meta to keychain: %w", err)
	}

	return nil
}

func (ks *KeychainStore) GetAccessToken() (string, error) {
	token, err := keyring.Get(ks.serviceName, accessTokenKey)
	if err != nil {
		return "", fmt.Errorf("getting access token from keychain: %w", err)
	}
	return token, nil
}

func (ks *KeychainStore) GetTokenMeta() (*TokenMeta, error) {
	metaStr, err := keyring.Get(ks.serviceName, tokenMetaKey)
	if err != nil {
		return nil, fmt.Errorf("getting token meta from keychain: %w", err)
	}

	var meta TokenMeta
	if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
		return nil, fmt.Errorf("parsing token meta: %w", err)
	}

	return &meta, nil
}

func (ks *KeychainStore) IsTokenExpired() bool {
	meta, err := ks.GetTokenMeta()
	if err != nil {
		return true
	}
	// Consider expired if within 60 seconds of expiry
	return time.Now().Unix() >= meta.ExpiresAt-60
}

func (ks *KeychainStore) GetRefreshToken() (string, error) {
	meta, err := ks.GetTokenMeta()
	if err != nil {
		return "", err
	}
	if meta.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token stored")
	}
	return meta.RefreshToken, nil
}

func (ks *KeychainStore) Clear() error {
	_ = keyring.Delete(ks.serviceName, accessTokenKey)
	_ = keyring.Delete(ks.serviceName, tokenMetaKey)
	return nil
}
