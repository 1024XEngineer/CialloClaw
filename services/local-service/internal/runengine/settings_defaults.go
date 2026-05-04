package runengine

// DefaultSettingsSnapshot returns a cloned default settings snapshot suitable
// for settings reset flows that need the same contract as engine bootstrap.
func DefaultSettingsSnapshot() map[string]any {
	return buildDefaultSettings()
}
