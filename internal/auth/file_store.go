package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/martinjt/claude-history-cli/internal/config"
)

// FileStore stores tokens in an encrypted file when keychain is unavailable
type FileStore struct {
	filePath string
}

type fileTokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token"`
	ExpiresAt    int64     `json:"expires_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func NewFileStore() *FileStore {
	configDir := config.DefaultConfigDir()
	return &FileStore{
		filePath: filepath.Join(configDir, "tokens.enc"),
	}
}

// deriveKey creates a key from machine-specific data
func (fs *FileStore) deriveKey() ([]byte, error) {
	// Use machine hostname + config dir as key material
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "default-host"
	}

	// Combine with config dir for uniqueness
	keyMaterial := hostname + fs.filePath

	// Derive 32-byte key using SHA256
	hash := sha256.Sum256([]byte(keyMaterial))
	return hash[:], nil
}

func (fs *FileStore) encrypt(data []byte) (string, error) {
	key, err := fs.deriveKey()
	if err != nil {
		return "", fmt.Errorf("deriving encryption key: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generating nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (fs *FileStore) decrypt(encoded string) ([]byte, error) {
	key, err := fs.deriveKey()
	if err != nil {
		return nil, fmt.Errorf("deriving encryption key: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decoding base64: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("creating cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypting: %w", err)
	}

	return plaintext, nil
}

func (fs *FileStore) SaveTokens(accessToken string, resp *TokenResponse) error {
	data := fileTokenData{
		AccessToken:  accessToken,
		RefreshToken: resp.RefreshToken,
		IDToken:      resp.IDToken,
		ExpiresAt:    time.Now().Add(time.Duration(resp.ExpiresIn) * time.Second).Unix(),
		UpdatedAt:    time.Now(),
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshaling token data: %w", err)
	}

	encrypted, err := fs.encrypt(jsonData)
	if err != nil {
		return fmt.Errorf("encrypting tokens: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Write with restricted permissions
	if err := os.WriteFile(fs.filePath, []byte(encrypted), 0600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}

	return nil
}

func (fs *FileStore) loadTokens() (*fileTokenData, error) {
	encryptedData, err := os.ReadFile(fs.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no tokens stored")
		}
		return nil, fmt.Errorf("reading token file: %w", err)
	}

	decrypted, err := fs.decrypt(string(encryptedData))
	if err != nil {
		return nil, fmt.Errorf("decrypting tokens: %w", err)
	}

	var data fileTokenData
	if err := json.Unmarshal(decrypted, &data); err != nil {
		return nil, fmt.Errorf("parsing token data: %w", err)
	}

	return &data, nil
}

func (fs *FileStore) GetAccessToken() (string, error) {
	data, err := fs.loadTokens()
	if err != nil {
		return "", err
	}
	return data.AccessToken, nil
}

func (fs *FileStore) GetTokenMeta() (*TokenMeta, error) {
	data, err := fs.loadTokens()
	if err != nil {
		return nil, err
	}

	return &TokenMeta{
		ExpiresAt:    data.ExpiresAt,
		RefreshToken: data.RefreshToken,
		IDToken:      data.IDToken,
	}, nil
}

func (fs *FileStore) IsTokenExpired() bool {
	data, err := fs.loadTokens()
	if err != nil {
		return true
	}
	// Consider expired if within 60 seconds of expiry
	return time.Now().Unix() >= data.ExpiresAt-60
}

func (fs *FileStore) GetRefreshToken() (string, error) {
	data, err := fs.loadTokens()
	if err != nil {
		return "", err
	}
	if data.RefreshToken == "" {
		return "", fmt.Errorf("no refresh token stored")
	}
	return data.RefreshToken, nil
}

func (fs *FileStore) Clear() error {
	if err := os.Remove(fs.filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing token file: %w", err)
	}
	return nil
}
