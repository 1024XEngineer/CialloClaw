//go:build windows

package platform

import (
	"strings"

	"golang.org/x/sys/windows/registry"
)

func osKnownFolders() map[string]string {
	values := map[string][]string{
		knownFolderDesktop:   {"Desktop", "{B4BFCC3A-DB2C-424C-B029-7FE99A87C641}"},
		knownFolderDocuments: {"Personal", "{FDD39AD0-238F-46AF-ADB4-6C85480369C7}"},
		knownFolderDownloads: {"{374DE290-123F-4565-9164-39C4925E467B}", "Downloads"},
	}
	folders := make(map[string]string, len(values))
	for alias, names := range values {
		for _, registryPath := range []string{
			`Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`,
			`Software\Microsoft\Windows\CurrentVersion\Explorer\Shell Folders`,
		} {
			path := readRegistryKnownFolder(registryPath, names)
			if strings.TrimSpace(path) == "" {
				continue
			}
			folders[alias] = path
			break
		}
	}
	return folders
}

func readRegistryKnownFolder(registryPath string, valueNames []string) string {
	key, err := registry.OpenKey(registry.CURRENT_USER, registryPath, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()
	for _, valueName := range valueNames {
		value, _, err := key.GetStringValue(valueName)
		if err != nil {
			continue
		}
		value, err = registry.ExpandString(value)
		if err != nil {
			continue
		}
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
