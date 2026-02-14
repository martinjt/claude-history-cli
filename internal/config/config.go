package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	APIEndpoint     string   `yaml:"api_endpoint"`
	MachineID       string   `yaml:"machine_id"`
	ClaudeDataDir   string   `yaml:"claude_data_dir"`
	ExcludePatterns []string `yaml:"exclude_patterns"`
	SyncInterval    int      `yaml:"sync_interval_minutes"`
	CognitoRegion   string   `yaml:"cognito_region"`
	CognitoPoolID   string   `yaml:"cognito_pool_id"`
	CognitoClientID string   `yaml:"cognito_client_id"`
	CognitoDomain   string   `yaml:"cognito_domain"`
}

func DefaultConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude-history-sync"
	}
	return filepath.Join(home, ".claude-history-sync")
}

func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

func DefaultClaudeDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude/projects"
	}
	return filepath.Join(home, ".claude", "projects")
}

func Load() (*Config, error) {
	return LoadFrom(DefaultConfigPath())
}

func LoadFrom(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	return cfg, nil
}

func DefaultConfig() *Config {
	hostname, _ := os.Hostname()
	return &Config{
		APIEndpoint:     "https://claude-history-mcp.devrel.hny.wtf",
		MachineID:       hostname,
		ClaudeDataDir:   DefaultClaudeDataDir(),
		ExcludePatterns: []string{},
		SyncInterval:    5,
		// Production Cognito configuration - hardcoded for SaaS
		CognitoRegion:   "eu-west-1",
		CognitoPoolID:   "eu-west-1_CmpHruSh7",
		CognitoClientID: "79c7ftkao9ae7drb9qrij9q7tc",
		CognitoDomain:   "claude-history-prod.auth.eu-west-1.amazoncognito.com",
	}
}

func (c *Config) Save() error {
	return c.SaveTo(DefaultConfigPath())
}

func (c *Config) SaveTo(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}
