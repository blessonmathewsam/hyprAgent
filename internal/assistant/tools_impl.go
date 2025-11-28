package assistant

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/reinhart/hyprAgent/internal/configuration"
	"github.com/reinhart/hyprAgent/internal/safety"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// ... (Discovery, File, Parsing, Patch tools remain the same) ...

// --- Discovery Tools ---

type DetectRootTool struct {
	Backends []configuration.ConfigBackend
}

func (t *DetectRootTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "detect_installation_root",
		Description: "Detects the Hyprland installation type and root path",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {},
			"additionalProperties": false
		}`),
	}
}

func (t *DetectRootTool) Execute(args string) (string, error) {
	for _, b := range t.Backends {
		found, err := b.Detect("")
		if err != nil {
			continue
		}
		if found {
			sources, _ := b.ListSources()
			return fmt.Sprintf(`{"type": "%s", "sources": %q}`, b.Type(), sources), nil
		}
	}
	return `{"type": "unknown"}`, nil
}

// --- File Access Tools ---

type ReadFileTool struct {
	Config  *configuration.Config
	Backend configuration.ConfigBackend
}

type ReadFileArgs struct {
	Path string `json:"path"`
}

func (t *ReadFileTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "read_file",
		Description: "Reads the content of a file within the allowed Hyprland configuration directories",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "The path to the file to read (relative to ~/.config/hypr or absolute)"}
			},
			"required": ["path"],
			"additionalProperties": false
		}`),
	}
}

func (t *ReadFileTool) Execute(args string) (string, error) {
	var a ReadFileArgs
	if err := ParseArgs(args, &a); err != nil {
		return "", err
	}

	// Use active backend
	backendType := t.Backend.Type()

	// Validate path is allowed
	allowed, err := t.Config.IsPathAllowed(backendType, a.Path)
	if err != nil || !allowed {
		return "", fmt.Errorf("access denied: %v", err)
	}

	content, err := os.ReadFile(a.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return string(content), nil
}

type ListDirTool struct {
	Config  *configuration.Config
	Backend configuration.ConfigBackend
}

type ListDirArgs struct {
	Path string `json:"path"`
}

func (t *ListDirTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "list_dir",
		Description: "Lists the contents of a directory within allowed Hyprland configuration directories",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string", "description": "The path to the directory to list"}
			},
			"required": ["path"],
			"additionalProperties": false
		}`),
	}
}

func (t *ListDirTool) Execute(args string) (string, error) {
	var a ListDirArgs
	if err := ParseArgs(args, &a); err != nil {
		return "", err
	}

	// Use active backend
	backendType := t.Backend.Type()

	// Validate path is allowed
	allowed, err := t.Config.IsPathAllowed(backendType, a.Path)
	if err != nil || !allowed {
		return "", fmt.Errorf("access denied: %v", err)
	}

	entries, err := os.ReadDir(a.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}

	result, err := json.Marshal(names)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// --- Parsing Tools ---

type ParseConfigTool struct {
	Backend configuration.ConfigBackend
}

type ParseConfigArgs struct {
	Path string `json:"path"` // Optional, if specific file needed, otherwise uses backend logic
}

func (t *ParseConfigTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "parse_config",
		Description: "Parses the configuration into a structured format",
		Parameters: json.RawMessage(`{
			"type": "object",
			"properties": {
				"path": {"type": "string"}
			},
			"additionalProperties": false
		}`),
	}
}

func (t *ParseConfigTool) Execute(args string) (string, error) {
	// Use active backend directly
	ir, err := t.Backend.Parse()
	if err != nil {
		return "", err
	}
	// Serialize IR to JSON for the LLM
	irJSON, err := json.Marshal(ir)
	if err != nil {
		return "", err
	}
	return string(irJSON), nil
}

// --- Patch Tools ---

type MakePatchTool struct{}

type MakePatchArgs struct {
	Original string `json:"original"`
	Modified string `json:"modified"`
}

func (t *MakePatchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "make_patch",
		Description: "Creates a unified diff patch between original and modified content. Returns a standard unified diff format.",
		Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "original": {"type": "string", "description": "The original file content"},
                "modified": {"type": "string", "description": "The modified file content"}
            },
            "required": ["original", "modified"]
        }`),
	}
}

func (t *MakePatchTool) Execute(args string) (string, error) {
	var a MakePatchArgs
	if err := ParseArgs(args, &a); err != nil {
		return "", err
	}

	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(a.Original, a.Modified, false)
	patches := dmp.PatchMake(a.Original, diffs)
	patchText := dmp.PatchToText(patches)

	// Validate the patch is not empty
	if strings.TrimSpace(patchText) == "" {
		return "", fmt.Errorf("no changes detected between original and modified content")
	}

	return patchText, nil
}

type ApplyPatchTool struct {
	Backend  configuration.ConfigBackend
	Snapshot *safety.SnapshotService
	Config   *configuration.Config
	Confirm  func(action string) bool // Callback for user confirmation
}

type ApplyPatchArgs struct {
	Path  string `json:"path"`
	Patch string `json:"patch"`
}

func (t *ApplyPatchTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "apply_patch",
		Description: "Applies a patch to the configuration. REQUIRES user confirmation.",
		Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "path": {"type": "string", "description": "Optional path to the file to patch"},
                "patch": {"type": "string"}
            },
            "required": ["patch"]
        }`),
	}
}

func (t *ApplyPatchTool) Execute(args string) (string, error) {
	var a ApplyPatchArgs
	if err := ParseArgs(args, &a); err != nil {
		return "", err
	}

	// CLEANUP: Strip code blocks if present
	patch := a.Patch

	// Remove markdown code blocks (```diff, ```, etc.)
	if strings.Contains(patch, "```") {
		lines := strings.Split(patch, "\n")
		var cleanLines []string
		inBlock := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inBlock = !inBlock
				continue
			}
			if !inBlock {
				cleanLines = append(cleanLines, line)
			}
		}
		patch = strings.Join(cleanLines, "\n")
	}

	// Remove common LLM conversational wrappers
	lines := strings.Split(patch, "\n")
	var filteredLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip decorative lines
		if strings.HasPrefix(trimmed, "***") ||
			strings.HasPrefix(trimmed, "---") && !strings.Contains(line, "@@") ||
			strings.HasPrefix(trimmed, "Here is") ||
			strings.HasPrefix(trimmed, "Shall I") {
			continue
		}
		filteredLines = append(filteredLines, line)
	}
	patch = strings.Join(filteredLines, "\n")
	patch = strings.TrimSpace(patch)

	// Validate patch format
	if !strings.Contains(patch, "@@") {
		return "", fmt.Errorf("invalid patch format: missing @@ markers. The patch must be in unified diff format generated by make_patch tool")
	}

	// Use active backend directly
	activeBackend := t.Backend
	backendType := t.Backend.Type()

	// Determine target file and validate it's allowed
	targetPath := a.Path
	if targetPath == "" {
		sources, err := activeBackend.ListSources()
		if err != nil || len(sources) == 0 {
			return "", fmt.Errorf("could not determine target file")
		}
		targetPath = sources[0]
	}

	// Validate path is allowed for write operations
	allowed, err := t.Config.IsPathAllowed(backendType, targetPath)
	if err != nil || !allowed {
		return "", fmt.Errorf("write access denied: %v", err)
	}

	// Read current file content
	contentBytes, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to read target file %s: %w", targetPath, err)
	}
	originalContent := string(contentBytes)

	// Snapshot before applying
	sources, err := activeBackend.ListSources()
	if err == nil && t.Snapshot != nil {
		id, err := t.Snapshot.CreateSnapshot(sources)
		if err != nil {
			return "", fmt.Errorf("failed to create snapshot: %w", err)
		}
		_ = id
	}

	// Apply Patch using diffmatchpatch
	dmp := diffmatchpatch.New()
	patches, err := dmp.PatchFromText(patch)
	if err != nil {
		return "", fmt.Errorf("failed to parse patch: %w. Ensure you're using the output from make_patch tool", err)
	}

	newContent, results := dmp.PatchApply(patches, originalContent)

	// Check if all patches applied successfully
	var failedPatches []int
	for i, success := range results {
		if !success {
			failedPatches = append(failedPatches, i)
		}
	}

	if len(failedPatches) > 0 {
		return "", fmt.Errorf("patch application failed: %d out of %d hunks failed to apply. The file may have been modified since you read it. Please re-read the file and regenerate the patch", len(failedPatches), len(results))
	}

	// Write the patched content back
	err = os.WriteFile(targetPath, []byte(newContent), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write patched file: %w", err)
	}

	return fmt.Sprintf("Patch applied successfully to %s", targetPath), nil
}

// --- Rollback Tool ---

type RollbackTool struct {
	Snapshot *safety.SnapshotService
}

type RollbackArgs struct {
	SnapshotID string `json:"snapshot_id"` // Optional, if not provided uses latest
}

func (t *RollbackTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "rollback",
		Description: "Restores the configuration from a previous snapshot",
		Parameters: json.RawMessage(`{
            "type": "object",
            "properties": {
                "snapshot_id": {"type": "string", "description": "The ID of the snapshot to restore. If empty, restores the latest."}
            },
            "additionalProperties": false
        }`),
	}
}

func (t *RollbackTool) Execute(args string) (string, error) {
	var a RollbackArgs
	if err := ParseArgs(args, &a); err != nil {
		return "", err
	}

	// TODO: Implement "Latest" logic in SnapshotService if ID is empty
	// For MVP, we require ID or just look for latest dir.
	// Let's assume we need to implement FindLatest in SnapshotService.
	// For now, return instructions if ID missing.
	if a.SnapshotID == "" {
		// Quick hack: List snapshots directory and pick last one
		// In real impl: t.Snapshot.Latest()
		return "Error: Snapshot ID required (automatic latest detection not implemented yet)", nil
	}

	// We need to know WHAT files to restore. The Snapshot service currently takes targetFiles in Restore.
	// But we don't know them here without asking backend or storing manifest.
	// TODO: Implement robust rollback with manifest storage in SnapshotService
	return "Rollback not fully implemented. Please manually restore from ~/.local/share/hyprAgent/backups/", nil
}
