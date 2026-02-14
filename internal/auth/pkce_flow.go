package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PKCEFlow implements OAuth 2.0 Authorization Code flow with PKCE
type PKCEFlow struct {
	config *Config
	client *http.Client
}

func NewPKCEFlow(config *Config) *PKCEFlow {
	return &PKCEFlow{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// generatePKCE creates code verifier and challenge for PKCE
func generatePKCE() (verifier, challenge string, err error) {
	// Generate random 32-byte verifier
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating random bytes: %w", err)
	}

	// Base64 URL encode without padding
	verifier = base64.RawURLEncoding.EncodeToString(b)

	// SHA256 hash and base64 URL encode
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])

	return verifier, challenge, nil
}

// StartAuthFlow initiates the PKCE flow and opens browser
func (pf *PKCEFlow) StartAuthFlow(ctx context.Context) (*TokenResponse, error) {
	// Generate PKCE codes
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, fmt.Errorf("generating PKCE: %w", err)
	}

	// Build authorization URL
	authURL := fmt.Sprintf("https://%s/oauth2/authorize", pf.config.Domain)
	params := url.Values{
		"client_id":             {pf.config.ClientID},
		"response_type":         {"code"},
		"redirect_uri":          {"http://localhost:3000/callback"},
		"scope":                 {strings.Join(pf.config.Scopes, " ")},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	fullAuthURL := fmt.Sprintf("%s?%s", authURL, params.Encode())

	// Start local callback server
	codeChan := make(chan string, 1)
	errChan := make(chan error, 1)

	server := &http.Server{
		Addr: ":3000",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			if code == "" {
				errorMsg := r.URL.Query().Get("error")
				errorDesc := r.URL.Query().Get("error_description")

				w.Header().Set("Content-Type", "text/html")
				fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head><title>Authentication Failed</title></head>
<body>
	<h1>Authentication Failed</h1>
	<p>Error: %s</p>
	<p>%s</p>
	<p>You can close this window.</p>
</body>
</html>`, errorMsg, errorDesc)

				errChan <- fmt.Errorf("authentication error: %s - %s", errorMsg, errorDesc)
				return
			}

			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body>
	<h1>✅ Authentication Successful!</h1>
	<p>You can close this window and return to the terminal.</p>
</body>
</html>`)

			codeChan <- code
		}),
	}

	// Start server in background
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("callback server error: %w", err)
		}
	}()

	// Give server a moment to start
	time.Sleep(100 * time.Millisecond)

	// Open browser
	fmt.Println("\n🔐 Opening browser for authentication...")
	fmt.Printf("📱 If browser doesn't open, visit: %s\n\n", fullAuthURL)

	if err := openBrowser(fullAuthURL); err != nil {
		fmt.Printf("⚠️  Could not open browser automatically: %v\n", err)
		fmt.Printf("Please open this URL manually: %s\n\n", fullAuthURL)
	}

	// Wait for callback or error
	var authCode string
	select {
	case <-ctx.Done():
		server.Shutdown(context.Background())
		return nil, fmt.Errorf("authentication cancelled")
	case err := <-errChan:
		server.Shutdown(context.Background())
		return nil, err
	case authCode = <-codeChan:
		// Got the code, proceed to exchange
	case <-time.After(5 * time.Minute):
		server.Shutdown(context.Background())
		return nil, fmt.Errorf("authentication timeout after 5 minutes")
	}

	// Shutdown callback server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	server.Shutdown(shutdownCtx)

	// Exchange authorization code for tokens
	return pf.ExchangeCode(ctx, authCode, verifier)
}

// ExchangeCode exchanges authorization code for access/refresh tokens
func (pf *PKCEFlow) ExchangeCode(ctx context.Context, code, verifier string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("https://%s/oauth2/token", pf.config.Domain)

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {pf.config.ClientID},
		"code":          {code},
		"redirect_uri":  {"http://localhost:3000/callback"},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := pf.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchanging authorization code: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result TokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing token response: %w", err)
	}

	return &result, nil
}

// RefreshToken refreshes an expired access token
func (pf *PKCEFlow) RefreshToken(ctx context.Context, refreshToken string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("https://%s/oauth2/token", pf.config.Domain)

	data := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {pf.config.ClientID},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("creating refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := pf.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("refreshing token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading refresh response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result TokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing refresh response: %w", err)
	}

	return &result, nil
}
