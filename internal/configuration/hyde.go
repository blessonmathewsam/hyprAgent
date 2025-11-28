package configuration

import (
	"os"
	"path/filepath"
)

type HyDEBackend struct {
	NativeBackend
}

func (b *HyDEBackend) Type() ConfigSourceType {
	return SourceHyDE
}

func (b *HyDEBackend) Detect(rootPath string) (bool, error) {
	if rootPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, err
		}
		rootPath = filepath.Join(home, ".config", "hypr")
	}

	// HyDE Detection:
	// 1. Check for global HyDE install markers (Environment Variable)
	// This is the most reliable method for HyDE
	isHyDEEnv := os.Getenv("HYDE_CONFIG_HOME") != ""
	if isHyDEEnv {
		b.NativeBackend.ConfigPath = filepath.Join(rootPath, "hyprland.conf")
		return true, nil
	}

	// 2. Check for specific HyDE directories/files in root
	// HyDE usually has a "hyde.conf" in the root
	hydeConf := filepath.Join(rootPath, "hyde.conf")
	if _, err := os.Stat(hydeConf); err == nil {
		b.NativeBackend.ConfigPath = filepath.Join(rootPath, "hyprland.conf")
		return true, nil
	}

	// 3. Check for directory structure
	configsDir := filepath.Join(rootPath, "Configs")
	scriptsDir := filepath.Join(rootPath, "scripts")
	
	_, configErr := os.Stat(configsDir)
	_, scriptsErr := os.Stat(scriptsDir)

	if !os.IsNotExist(configErr) || !os.IsNotExist(scriptsErr) {
		b.NativeBackend.ConfigPath = filepath.Join(rootPath, "hyprland.conf")
		return true, nil
	}

	return false, nil
}

// Reuse NativeBackend's ListSources, Parse, GeneratePatch, ApplyPatch
