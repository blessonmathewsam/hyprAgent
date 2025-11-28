package configuration

import (
	"os"
	"path/filepath"
)

type OmarchyBackend struct {
	NativeBackend
}

func (b *OmarchyBackend) Type() ConfigSourceType {
	return SourceOmarchy
}

func (b *OmarchyBackend) Detect(rootPath string) (bool, error) {
	if rootPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, err
		}
		rootPath = filepath.Join(home, ".config", "hypr")
	}

	// Omarchy Detection (Assumption): Look for "omarchy" folder or specific file
	omarchyDir := filepath.Join(rootPath, "omarchy")
	if _, err := os.Stat(omarchyDir); os.IsNotExist(err) {
		return false, nil
	}

	// Check for main config
	configPath := filepath.Join(rootPath, "hyprland.conf")
	if _, err := os.Stat(configPath); err == nil {
		b.NativeBackend.ConfigPath = configPath
		return true, nil
	}

	return false, nil
}
