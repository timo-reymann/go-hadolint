// Package hadolint provides API access to hadolint, the Dockerfile linter.
//
// The package embeds a hadolint binary for the current GOOS/GOARCH when one is
// available. On platforms without an embedded binary, it falls back to a
// `hadolint` executable resolved via PATH.
package hadolint

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"

	"github.com/amenzhinsky/go-memexec"
	"github.com/google/shlex"
)

// ErrNotAvailable is returned when no hadolint binary is embedded for the
// current platform and no `hadolint` executable can be found on PATH.
var ErrNotAvailable = errors.New("hadolint: no embedded binary for this platform and no `hadolint` found on PATH")

// Hadolinter provides API access to either the embedded hadolint binary or a
// `hadolint` executable resolved via PATH.
//
// To pin a hadolint configuration (and short-circuit hadolint's default
// discovery in ~/.config/hadolint.yaml, /etc/hadolint.yaml, and
// ./.hadolint.yaml) populate one of:
//
//   - ConfigFile: path to an existing hadolint config file on disk.
//   - Config:     a typed Config struct that the library serializes to a
//     temp file on each invocation and removes afterwards.
//
// If both are set, Config takes precedence. If neither is set, hadolint will
// fall back to its default discovery.
type Hadolinter struct {
	// Config, when non-nil, is serialized to a YAML temp file on each
	// invocation and passed via `-c`. Takes precedence over ConfigFile.
	Config *Config
	// ConfigFile, when non-empty, is the path to an existing hadolint
	// config file on disk. Passed via `-c` on every invocation.
	ConfigFile string

	memExec *memexec.Exec
	pathBin string
}

// Source describes where the active hadolint binary is loaded from.
type Source int

const (
	// SourceEmbedded indicates the binary was loaded from the embedded bytes.
	SourceEmbedded Source = iota
	// SourcePATH indicates the binary was resolved via $PATH.
	SourcePATH
)

// Source reports where the active hadolint binary was loaded from.
func (h *Hadolinter) Source() Source {
	if h.memExec != nil {
		return SourceEmbedded
	}
	return SourcePATH
}

// BinaryPath returns the resolved PATH location when running from PATH, or an
// empty string when running from the embedded binary.
func (h *Hadolinter) BinaryPath() string {
	return h.pathBin
}

func (h *Hadolinter) command(args ...string) *exec.Cmd {
	if h.memExec != nil {
		return h.memExec.Command(args...)
	}
	return exec.Command(h.pathBin, args...)
}

// Version returns the hadolint version reported by `hadolint --version`.
func (h *Hadolinter) Version() string {
	cmd := h.command("--version")
	output, _ := cmd.Output()
	// hadolint --version typically prints "Haskell Dockerfile Linter <version>" or just "<version>".
	line := strings.TrimSpace(string(output))
	if idx := strings.LastIndex(line, " "); idx >= 0 {
		return strings.TrimSpace(line[idx+1:])
	}
	return line
}

// Close releases resources held by the embedded binary runner. Safe to call on
// PATH-backed instances (no-op).
func (h *Hadolinter) Close() error {
	if h.memExec != nil {
		return h.memExec.Close()
	}
	return nil
}

func (h *Hadolinter) execute(stdin []byte, args ...string) (*Result, error) {
	cmd := h.command(args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	output, _ := cmd.Output()
	return NewResult(cmd.ProcessState.ExitCode(), output)
}

// resolveConfigPath materializes the config to use for one invocation.
// Returns the path to pass via `-c`, a cleanup function the caller must
// invoke, and any error encountered. The cleanup is always non-nil and is
// safe to defer immediately after a successful call.
func (h *Hadolinter) resolveConfigPath() (string, func(), error) {
	if h.Config != nil {
		path, err := h.Config.writeTempFile()
		if err != nil {
			return "", func() {}, err
		}
		return path, func() { _ = os.Remove(path) }, nil
	}
	if h.ConfigFile != "" {
		return h.ConfigFile, func() {}, nil
	}
	return "", func() {}, nil
}

// buildArgs builds the argv passed to hadolint, prepending the resolved
// config path (when present) and the JSON output flags before any
// caller-supplied extra flags.
func (h *Hadolinter) buildArgs(configPath, extraFlags string, trailing ...string) ([]string, error) {
	args := []string{"--format", "json", "--no-color"}
	if configPath != "" {
		args = append(args, "-c", configPath)
	}
	if extraFlags != "" {
		extra, err := shlex.Split(extraFlags)
		if err != nil {
			return nil, err
		}
		args = append(args, extra...)
	}
	args = append(args, trailing...)
	return args, nil
}

func (h *Hadolinter) analyze(stdin []byte, extraFlags string, trailing ...string) (*Result, error) {
	configPath, cleanup, err := h.resolveConfigPath()
	if err != nil {
		return nil, err
	}
	defer cleanup()
	args, err := h.buildArgs(configPath, extraFlags, trailing...)
	if err != nil {
		return nil, err
	}
	return h.execute(stdin, args...)
}

// AnalyzeFile runs hadolint against a Dockerfile at the given path.
// extraFlags is an optional shell-quoted string of additional hadolint flags.
func (h *Hadolinter) AnalyzeFile(path string, extraFlags string) (*Result, error) {
	return h.analyze(nil, extraFlags, path)
}

// AnalyzeSnippet runs hadolint against an in-memory Dockerfile snippet by
// piping it to hadolint's stdin.
func (h *Hadolinter) AnalyzeSnippet(snippet []byte, extraFlags string) (*Result, error) {
	return h.analyze(snippet, extraFlags, "-")
}

// NewHadolinter instantiates a Hadolinter. It prefers the embedded binary for
// the current platform; if no embedded binary is available it falls back to a
// `hadolint` executable on PATH.
func NewHadolinter() (*Hadolinter, error) {
	if len(hadolintBinary) > 0 {
		exe, err := memexec.New(hadolintBinary)
		if err != nil {
			return nil, err
		}
		return &Hadolinter{memExec: exe}, nil
	}
	return NewHadolinterFromPATH()
}

// NewHadolinterFromPATH bypasses the embedded binary and resolves `hadolint`
// from PATH. Returns ErrNotAvailable if the executable cannot be found.
//
// Use this when you want to defer to a system-installed hadolint (for
// example, to pick up a newer upstream version than the one embedded by this
// package), or to keep the embedded binary out of memory.
func NewHadolinterFromPATH() (*Hadolinter, error) {
	path, err := exec.LookPath("hadolint")
	if err != nil {
		return nil, ErrNotAvailable
	}
	if _, statErr := os.Stat(path); statErr != nil {
		return nil, ErrNotAvailable
	}
	return &Hadolinter{pathBin: path}, nil
}
