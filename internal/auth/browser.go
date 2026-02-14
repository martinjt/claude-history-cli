package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openBrowser opens the default browser to the given URL
func openBrowser(url string) error {
	var cmdName string
	var cmdArgs []string

	switch runtime.GOOS {
	case "linux":
		cmdName = "xdg-open"
		cmdArgs = []string{url}
	case "darwin":
		cmdName = "open"
		cmdArgs = []string{url}
	case "windows":
		cmdName = "rundll32"
		cmdArgs = []string{"url.dll,FileProtocolHandler", url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	// Check if the command exists
	if _, err := exec.LookPath(cmdName); err != nil {
		return fmt.Errorf("command %q not found in PATH: %w", cmdName, err)
	}

	cmd := exec.Command(cmdName, cmdArgs...)
	return cmd.Start()
}
