package configuration

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

type NativeBackend struct {
	ConfigPath string
}

func NewNativeBackend() *NativeBackend {
	return &NativeBackend{}
}

func (b *NativeBackend) Type() ConfigSourceType {
	return SourceNative
}

func (b *NativeBackend) Detect(rootPath string) (bool, error) {
	if rootPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, err
		}
		rootPath = filepath.Join(home, ".config", "hypr")
	}

	configPath := filepath.Join(rootPath, "hyprland.conf")
	if _, err := os.Stat(configPath); err == nil {
		b.ConfigPath = configPath
		return true, nil
	}
	return false, nil
}

func (b *NativeBackend) ListSources() ([]string, error) {
	if b.ConfigPath == "" {
		return nil, fmt.Errorf("config path not detected")
	}
	return []string{b.ConfigPath}, nil
}

func (b *NativeBackend) Parse() (*IR, error) {
	if b.ConfigPath == "" {
		return nil, fmt.Errorf("config path not set")
	}

	file, err := os.Open(b.ConfigPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []ConfigLine
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		raw := scanner.Text()
		trimmed := strings.TrimSpace(raw)

		line := ConfigLine{
			LineNum: lineNum,
			Raw:     raw,
		}

		if trimmed == "" {
			line.Type = LineTypeEmpty
		} else if strings.HasPrefix(trimmed, "#") {
			line.Type = LineTypeComment
		} else if strings.HasPrefix(trimmed, "$") {
			line.Type = LineTypeVariable
			parts := strings.SplitN(trimmed, "=", 2)
			if len(parts) == 2 {
				line.Key = strings.TrimSpace(parts[0])
				line.Value = strings.TrimSpace(parts[1])
			}
		} else if strings.HasSuffix(trimmed, "{") {
			line.Type = LineTypeSectionStart
			line.Key = strings.TrimSuffix(trimmed, "{")
			line.Key = strings.TrimSpace(line.Key)
		} else if trimmed == "}" {
			line.Type = LineTypeSectionEnd
		} else if strings.Contains(trimmed, "=") {
			line.Type = LineTypeKeyValue
			parts := strings.SplitN(trimmed, "=", 2)
			line.Key = strings.TrimSpace(parts[0])
			line.Value = strings.TrimSpace(parts[1])
		} else {
			// Fallback for things like 'exec-once ...' without equals if valid,
			// or complex binds. Hyprland usually requires =, but sometimes syntax varies.
			// Treating as generic content for now.
			line.Type = LineTypeUnknown
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &IR{Lines: lines}, nil
}

func (b *NativeBackend) GeneratePatch(oldIR, newIR *IR) (string, error) {
	dmp := diffmatchpatch.New()
	text1 := oldIR.String()
	text2 := newIR.String()

	diffs := dmp.DiffMain(text1, text2, false)
	// Simplify diffs (cleanup semantic)
	// dmp.DiffCleanupSemantic(diffs)

	// Create a patch
	patches := dmp.PatchMake(text1, diffs)
	return dmp.PatchToText(patches), nil
}

func (b *NativeBackend) ApplyPatch(path string, patchText string) error {
	targetPath := path
	if targetPath == "" {
		if b.ConfigPath == "" {
			return fmt.Errorf("config path not set")
		}
		targetPath = b.ConfigPath
	}

	// Read current file
	contentBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", targetPath, err)
	}
	text := string(contentBytes)

	dmp := diffmatchpatch.New()
	patches, err := dmp.PatchFromText(patchText)
	if err != nil {
		return fmt.Errorf("failed to parse patch: %w", err)
	}

	newText, results := dmp.PatchApply(patches, text)

	// Check if all patches applied successfully
	for _, success := range results {
		if !success {
			return fmt.Errorf("some patches failed to apply")
		}
	}

	// Write back
	file, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(newText)
	return err
}

// Save writes the IR back to the file (Overwrite)
func (b *NativeBackend) Save(ir *IR) error {
	if b.ConfigPath == "" {
		return fmt.Errorf("config path not set")
	}

	file, err := os.Create(b.ConfigPath)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)
	for _, line := range ir.Lines {
		_, err := w.WriteString(line.Raw + "\n")
		if err != nil {
			return err
		}
	}
	return w.Flush()
}
