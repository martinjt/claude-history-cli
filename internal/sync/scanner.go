package sync

import (
	"os"
	"path/filepath"
	"strings"
)

type FileInfo struct {
	Path        string
	ProjectPath string
	SessionID   string
	ModTime     int64
	Size        int64
}

func ScanForJSONL(baseDir string, excludePatterns []string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip directories we can't read
			if info != nil && info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") && name != "." && name != ".claude" {
				return filepath.SkipDir
			}
			return nil
		}

		if !strings.HasSuffix(info.Name(), ".jsonl") {
			return nil
		}

		if isExcluded(path, excludePatterns) {
			return nil
		}

		relPath, _ := filepath.Rel(baseDir, path)
		projectPath := extractProjectPath(relPath)
		sessionID := extractSessionID(info.Name())

		files = append(files, FileInfo{
			Path:        path,
			ProjectPath: projectPath,
			SessionID:   sessionID,
			ModTime:     info.ModTime().Unix(),
			Size:        info.Size(),
		})

		return nil
	})

	return files, err
}

func extractProjectPath(relPath string) string {
	dir := filepath.Dir(relPath)
	if dir == "." {
		return "/"
	}
	// Convert filesystem path separators to forward slashes
	return "/" + filepath.ToSlash(dir)
}

func extractSessionID(filename string) string {
	return strings.TrimSuffix(filename, ".jsonl")
}

func isExcluded(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}
