package filedb

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func ReadJSON[T any](path string, empty T) (T, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return empty, nil
		}
		return empty, err
	}
	if len(data) == 0 {
		return empty, nil
	}
	var out T
	if err := json.Unmarshal(data, &out); err != nil {
		return empty, err
	}
	return out, nil
}

func WriteJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmpPath, path)
}
