package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/martinjt/claude-history-cli/internal/api"
	"github.com/martinjt/claude-history-cli/internal/auth"
	"github.com/martinjt/claude-history-cli/internal/config"
	"github.com/martinjt/claude-history-cli/internal/sync"
)

const version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "sync":
		if err := runSync(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "login":
		if err := runLogin(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "logout":
		if err := runLogout(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "status":
		runStatus()
	case "version":
		fmt.Printf("claude-history-sync %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Usage: claude-history-sync <command> [flags]

Commands:
  sync      Sync Claude conversation history
  login     Authenticate with OAuth
            Flags:
              --force    Force re-authentication even if already authenticated
  logout    Clear stored credentials
  status    Show sync and auth status
  version   Print version information
  help      Show this help message`)
}

func runSync() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Setup auth
	authConfig := auth.NewConfig(cfg.CognitoRegion, cfg.CognitoPoolID, cfg.CognitoClientID, cfg.CognitoDomain)
	authManager := auth.NewManager(authConfig)

	// Validate we can get a token (refreshes automatically if access token expired)
	if _, err := authManager.GetValidToken(ctx); err != nil {
		return fmt.Errorf("not authenticated. Run 'claude-history-sync login' first: %w", err)
	}

	// Setup API client
	apiClient := api.NewClient(cfg.APIEndpoint, cfg.MachineID, authManager.GetValidToken)

	// Load sync state
	statePath := sync.DefaultStatePath()
	state, err := sync.LoadState(statePath)
	if err != nil {
		return fmt.Errorf("loading sync state: %w", err)
	}

	// Scan for JSONL files
	fmt.Printf("Scanning %s for conversations...\n", cfg.ClaudeDataDir)
	files, err := sync.ScanForJSONL(cfg.ClaudeDataDir, cfg.ExcludePatterns)
	if err != nil {
		return fmt.Errorf("scanning files: %w", err)
	}
	fmt.Printf("Found %d conversation files\n", len(files))

	// Fetch existing conversations with hashes from server
	fmt.Println("Fetching conversation list from server...")
	conversationsList, err := apiClient.GetConversations(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to fetch conversations list: %v\n", err)
		fmt.Println("Continuing with UUID-based sync (may re-process unchanged conversations)")
		conversationsList = &api.ConversationsListResponse{Conversations: []api.Conversation{}}
	} else {
		fmt.Printf("Server has %d conversations\n", conversationsList.Total)
	}

	// Build hash map for quick lookup
	remoteHashes := make(map[string]string)
	for _, conv := range conversationsList.Conversations {
		remoteHashes[conv.SessionID] = conv.Hash
	}

	// Calculate and sync deltas
	synced := 0
	skipped := 0
	errors := 0
	for _, file := range files {
		// Calculate local hash
		localHash, err := sync.CalculateFileHash(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error calculating hash for %s: %v\n", file.Path, err)
			errors++
			continue
		}

		// Check if conversation needs sync based on hash comparison
		remoteHash := remoteHashes[file.SessionID]
		if !sync.ConversationNeedsSync(localHash, remoteHash) {
			skipped++
			continue // Skip unchanged conversations
		}
		lastUUID := state.GetLastSyncedUUID(file.SessionID)
		delta, err := sync.CalculateDelta(file, lastUUID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: error processing %s: %v\n", file.Path, err)
			errors++
			continue
		}

		if delta == nil {
			continue // No new messages
		}

		// Convert messages for API
		apiMessages := make([]api.Message, len(delta.Messages))
		for i, m := range delta.Messages {
			apiMessages[i] = api.Message{
				UUID:      m.UUID,
				Timestamp: m.Timestamp,
				Role:      m.Role,
				Content:   m.Content,
				Model:     m.Model,
				Tokens:    0, // Not available in conversation format
			}
		}

		resp, err := apiClient.Sync(ctx, &api.SyncRequest{
			MachineID:   cfg.MachineID,
			SessionID:   delta.SessionID,
			ProjectPath: delta.ProjectPath,
			Messages:    apiMessages,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: sync failed for %s: %v\n", file.SessionID, err)
			errors++
			continue
		}

		if resp.Success {
			state.UpdateSession(file.SessionID, delta.NewLastUUID, resp.Processed)
			synced++
			fmt.Printf("  Synced %d messages from %s\n", resp.Processed, file.SessionID)
		}
	}

	// Save state
	if err := state.Save(statePath); err != nil {
		return fmt.Errorf("saving sync state: %w", err)
	}

	fmt.Printf("\nSync complete: %d sessions synced, %d skipped (unchanged)", synced, skipped)
	if errors > 0 {
		fmt.Printf(", %d errors", errors)
	}
	fmt.Println()

	return nil
}

func runLogin() error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Check for --force flag
	force := false
	for _, arg := range os.Args[2:] {
		if arg == "--force" || arg == "-f" {
			force = true
			break
		}
	}

	authConfig := auth.NewConfig(cfg.CognitoRegion, cfg.CognitoPoolID, cfg.CognitoClientID, cfg.CognitoDomain)
	manager := auth.NewManager(authConfig)

	return manager.Login(ctx, force)
}

func runLogout() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	authConfig := auth.NewConfig(cfg.CognitoRegion, cfg.CognitoPoolID, cfg.CognitoClientID, cfg.CognitoDomain)
	manager := auth.NewManager(authConfig)

	if err := manager.Logout(); err != nil {
		return fmt.Errorf("logout failed: %w", err)
	}

	fmt.Println("Successfully logged out.")
	return nil
}

func runStatus() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Config: error loading (%v)\n", err)
		return
	}

	fmt.Printf("Config:\n")
	fmt.Printf("  API Endpoint: %s\n", cfg.APIEndpoint)
	fmt.Printf("  Machine ID:   %s\n", cfg.MachineID)
	fmt.Printf("  Data Dir:     %s\n", cfg.ClaudeDataDir)

	authConfig := auth.NewConfig(cfg.CognitoRegion, cfg.CognitoPoolID, cfg.CognitoClientID, cfg.CognitoDomain)
	manager := auth.NewManager(authConfig)

	fmt.Printf("\nAuth:\n")
	ctx := context.Background()
	if _, err := manager.GetValidToken(ctx); err == nil {
		fmt.Printf("  Status: authenticated\n")
	} else {
		fmt.Printf("  Status: not authenticated (%v)\n", err)
	}

	state, err := sync.LoadState(sync.DefaultStatePath())
	if err != nil {
		fmt.Printf("\nSync State: error loading (%v)\n", err)
		return
	}

	fmt.Printf("\nSync State:\n")
	fmt.Printf("  Last Sync:    %s\n", state.LastSyncAt)
	fmt.Printf("  Sessions:     %d\n", len(state.Sessions))
}
