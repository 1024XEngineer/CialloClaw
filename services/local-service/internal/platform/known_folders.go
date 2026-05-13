package platform

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	knownFolderDesktop   = "desktop"
	knownFolderDocuments = "documents"
	knownFolderDownloads = "downloads"
)

var loadKnownFoldersForToolPolicy = detectKnownFoldersForToolPolicy

func detectKnownFoldersForToolPolicy() map[string]string {
	folders := make(map[string]string, 3)
	mergeKnownFolderPaths(folders, osKnownFolders())
	mergeKnownFolderPaths(folders, oneDriveKnownFolders())
	mergeKnownFolderPaths(folders, homeKnownFolders())
	return folders
}

func mergeKnownFolderPaths(dst, src map[string]string) {
	for alias, root := range src {
		canonical := canonicalKnownFolderAlias(alias)
		if canonical == "" || strings.TrimSpace(dst[canonical]) != "" {
			continue
		}
		if policyRoot, ok := resolveOptionalPolicyRoot(root); ok {
			dst[canonical] = policyRoot.clean
		}
	}
}

func oneDriveKnownFolders() map[string]string {
	for _, envKey := range []string{"OneDriveConsumer", "OneDriveCommercial", "OneDrive"} {
		root := strings.TrimSpace(os.Getenv(envKey))
		if root == "" {
			continue
		}
		return map[string]string{
			knownFolderDesktop:   filepath.Join(root, "Desktop"),
			knownFolderDocuments: filepath.Join(root, "Documents"),
			knownFolderDownloads: filepath.Join(root, "Downloads"),
		}
	}
	return nil
}

func homeKnownFolders() map[string]string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	homeDir = strings.TrimSpace(homeDir)
	if homeDir == "" {
		return nil
	}
	return map[string]string{
		knownFolderDesktop:   filepath.Join(homeDir, "Desktop"),
		knownFolderDocuments: filepath.Join(homeDir, "Documents"),
		knownFolderDownloads: filepath.Join(homeDir, "Downloads"),
	}
}

func resolveToolKnownFolderAlias(rawPath string, knownFolders map[string]string) (string, bool) {
	trimmed := strings.TrimSpace(rawPath)
	if len(knownFolders) == 0 || trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "~/") || strings.HasPrefix(trimmed, "~\\") {
		return resolveToolKnownFolderSuffix(trimmed[2:], knownFolders)
	}
	return "", false
}

func resolveToolKnownFolderSuffix(rawSuffix string, knownFolders map[string]string) (string, bool) {
	normalized := strings.TrimSpace(strings.ReplaceAll(rawSuffix, "\\", "/"))
	normalized = strings.TrimPrefix(normalized, "/")
	if normalized == "" {
		return "", false
	}
	segment, remainder, _ := strings.Cut(normalized, "/")
	alias := canonicalKnownFolderAlias(segment)
	if alias == "" {
		return "", false
	}
	root := strings.TrimSpace(knownFolders[alias])
	if root == "" {
		return "", false
	}
	if remainder == "" {
		return filepath.Clean(root), true
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(remainder))), true
}

func canonicalKnownFolderAlias(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case knownFolderDesktop:
		return knownFolderDesktop
	case knownFolderDocuments:
		return knownFolderDocuments
	case knownFolderDownloads:
		return knownFolderDownloads
	default:
		return ""
	}
}
