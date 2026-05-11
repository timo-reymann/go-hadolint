package hadolint

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is a typed representation of hadolint's YAML configuration schema.
//
// See https://github.com/hadolint/hadolint#configure for the documented keys.
// Pointer fields (*bool) are used where omitting a value is semantically
// different from setting it to its zero value.
type Config struct {
	// Override sets per-severity overrides for specific rule codes.
	Override *Override `yaml:"override,omitempty"`
	// Ignored is the list of rule codes to ignore entirely (e.g. "DL3000").
	Ignored []string `yaml:"ignored,omitempty"`
	// TrustedRegistries lists Docker registries that are not treated as
	// untrusted by DL3026.
	TrustedRegistries []string `yaml:"trustedRegistries,omitempty"`
	// LabelSchema declares expected LABEL keys and their validation types.
	LabelSchema map[string]string `yaml:"label-schema,omitempty"`
	// StrictLabels, when true, fails on labels that are missing from
	// LabelSchema.
	StrictLabels *bool `yaml:"strict-labels,omitempty"`
	// FailureThreshold maps a severity to the lowest level that causes a
	// non-zero exit (e.g. "error", "warning", "info", "style").
	FailureThreshold string `yaml:"failure-threshold,omitempty"`
}

// Override holds per-severity rule overrides.
type Override struct {
	Error   []string `yaml:"error,omitempty"`
	Warning []string `yaml:"warning,omitempty"`
	Info    []string `yaml:"info,omitempty"`
	Style   []string `yaml:"style,omitempty"`
}

// Marshal serializes the config to its hadolint YAML representation.
func (c *Config) Marshal() ([]byte, error) {
	return yaml.Marshal(c)
}

// writeTempFile serializes the config to a managed temp file and returns the
// file's path. The caller is responsible for removing the file.
func (c *Config) writeTempFile() (string, error) {
	data, err := c.Marshal()
	if err != nil {
		return "", fmt.Errorf("hadolint: marshal config: %w", err)
	}
	f, err := os.CreateTemp("", "go-hadolint-*.yaml")
	if err != nil {
		return "", fmt.Errorf("hadolint: create temp config: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("hadolint: write temp config: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("hadolint: close temp config: %w", err)
	}
	return f.Name(), nil
}
