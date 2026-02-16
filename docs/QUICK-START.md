# Quick Start Guide

## Prerequisites
- AWS Account with appropriate permissions
- Honeycomb account and API key
- GitHub repository with Actions enabled

## Deploy via GitHub Actions
1. Configure GitHub Secrets (AWS_ACCOUNT_ID, HONEYCOMB_API_KEY)
2. Go to Actions → Deploy workflow
3. Run workflow with "prod" environment

## Use the CLI
```bash
./claude-history-sync login    # Authenticate
./claude-history-sync sync     # Sync sessions
./claude-history-sync status   # Check progress
```

See full documentation in repository.
