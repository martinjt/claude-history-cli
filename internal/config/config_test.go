package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.APIEndpoint == "" {
		t.Error("expected default API endpoint")
	}
	if cfg.MachineID == "" {
		t.Error("expected default machine ID from hostname")
	}
	if cfg.ClaudeDataDir == "" {
		t.Error("expected default Claude data dir")
	}
	if cfg.SyncInterval != 5 {
		t.Errorf("expected sync interval 5, got %d", cfg.SyncInterval)
	}
	if cfg.CognitoRegion == "" {
		t.Error("expected default Cognito region")
	}
	if cfg.CognitoClientID == "" {
		t.Error("expected default Cognito client ID")
	}
}

func TestLoadFrom_NonExistent(t *testing.T) {
	cfg, err := LoadFrom("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error for missing config: %v", err)
	}

	// Should return defaults
	if cfg.SyncInterval != 5 {
		t.Errorf("expected default sync interval 5, got %d", cfg.SyncInterval)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := &Config{
		APIEndpoint:   "https://example.com",
		MachineID:     "test-machine",
		ClaudeDataDir: "/tmp/claude",
		SyncInterval:  10,
	}

	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	loaded, err := LoadFrom(path)
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}

	if loaded.APIEndpoint != "https://example.com" {
		t.Errorf("expected https://example.com, got %s", loaded.APIEndpoint)
	}
	if loaded.MachineID != "test-machine" {
		t.Errorf("expected test-machine, got %s", loaded.MachineID)
	}
	if loaded.SyncInterval != 10 {
		t.Errorf("expected 10, got %d", loaded.SyncInterval)
	}
}

func TestSaveTo_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "subdir", "config.yaml")

	cfg := DefaultConfig()
	if err := cfg.SaveTo(path); err != nil {
		t.Fatalf("saving config: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
}
