package configuration

import (
	"strings"
)

// ConfigSourceType enumerates the supported configuration backends
type ConfigSourceType string

const (
	SourceNative  ConfigSourceType = "native"
	SourceHyDE    ConfigSourceType = "hyde"
	SourceOmarchy ConfigSourceType = "omarchy"
)

type LineType int

const (
	LineTypeEmpty LineType = iota
	LineTypeComment
	LineTypeVariable
	LineTypeKeyValue
	LineTypeSectionStart
	LineTypeSectionEnd
	LineTypeUnknown
)

// ConfigLine represents a single line in the configuration file
type ConfigLine struct {
	LineNum int
	Raw     string
	Type    LineType
	Key     string
	Value   string
}

// IR (Intermediate Representation) holds the parsed configuration
type IR struct {
	Lines []ConfigLine
}

func (ir *IR) String() string {
	var sb strings.Builder
	for _, line := range ir.Lines {
		sb.WriteString(line.Raw)
		sb.WriteString("\n")
	}
	return sb.String()
}

// ConfigBackend defines the interface for different configuration sources
type ConfigBackend interface {
	// Type returns the type of this backend
	Type() ConfigSourceType

	// Detect checks if this backend is valid for the given root path
	Detect(rootPath string) (bool, error)

	// ListSources returns a list of file paths contributing to the config
	ListSources() ([]string, error)

	// Parse reads the configuration into an Intermediate Representation
	Parse() (*IR, error)

	// GeneratePatch creates a diff between two IR states
	GeneratePatch(oldIR, newIR *IR) (string, error)

	// ApplyPatch applies a patch to the specified file. If path is empty, applies to main config.
	ApplyPatch(path string, patch string) error
}
