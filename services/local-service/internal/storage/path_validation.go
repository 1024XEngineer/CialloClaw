package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var errUnsupportedWindowsAbsolutePath = errors.New("windows absolute path is unsupported on this platform")

// prepareSQLiteDatabasePath normalizes one configured SQLite path and rejects
// Windows drive-letter paths on non-Windows hosts before they can create
// accidental relative directories such as "./D:/" in the current workspace.
func prepareSQLiteDatabasePath(databasePath string) (string, error) {
	cleaned := strings.TrimSpace(databasePath)
	if cleaned == "" {
		return "", ErrDatabasePathRequired
	}
	cleaned = filepath.Clean(cleaned)
	if os.PathSeparator != '\\' && isWindowsStyleAbsolutePath(cleaned) {
		return "", fmt.Errorf("%w: %s", errUnsupportedWindowsAbsolutePath, cleaned)
	}
	if err := os.MkdirAll(filepath.Dir(cleaned), 0o755); err != nil {
		return "", fmt.Errorf("prepare sqlite directory: %w", err)
	}
	return cleaned, nil
}

func hasWindowsDriveLetterPrefix(value string) bool {
	if len(value) < 2 {
		return false
	}
	drive := value[0]
	return ((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) && value[1] == ':'
}

func isWindowsStyleAbsolutePath(value string) bool {
	return hasWindowsDriveLetterPrefix(value) && len(value) >= 3 && (value[2] == '\\' || value[2] == '/')
}
