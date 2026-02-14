package auth

import (
	"fmt"
	"os"
)

type Config struct {
	CognitoRegion   string
	UserPoolID      string
	ClientID        string
	Domain          string
	Scopes          []string
	DeviceFlowURL   string
	TokenURL        string
}

func NewConfigFromEnv() (*Config, error) {
	region := getEnvOrDefault("COGNITO_REGION", "eu-west-1")
	userPoolID := os.Getenv("COGNITO_USER_POOL_ID")
	clientID := os.Getenv("COGNITO_CLIENT_ID")
	domain := os.Getenv("COGNITO_DOMAIN")

	if userPoolID == "" {
		return nil, fmt.Errorf("COGNITO_USER_POOL_ID environment variable is required")
	}
	if clientID == "" {
		return nil, fmt.Errorf("COGNITO_CLIENT_ID environment variable is required")
	}
	if domain == "" {
		return nil, fmt.Errorf("COGNITO_DOMAIN environment variable is required")
	}

	return &Config{
		CognitoRegion: region,
		UserPoolID:    userPoolID,
		ClientID:      clientID,
		Domain:        domain,
		Scopes:        []string{"openid", "email", "profile"},
		DeviceFlowURL: fmt.Sprintf("https://%s/oauth2/device_authorization", domain),
		TokenURL:      fmt.Sprintf("https://%s/oauth2/token", domain),
	}, nil
}

func NewConfig(region, userPoolID, clientID, domain string) *Config {
	return &Config{
		CognitoRegion: region,
		UserPoolID:    userPoolID,
		ClientID:      clientID,
		Domain:        domain,
		Scopes:        []string{"openid", "email", "profile"},
		DeviceFlowURL: fmt.Sprintf("https://%s/oauth2/device_authorization", domain),
		TokenURL:      fmt.Sprintf("https://%s/oauth2/token", domain),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
