//go:build !windows

package platform

func osKnownFolders() map[string]string {
	return nil
}
