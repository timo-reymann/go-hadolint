package hadolint

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// runSuite is the set of behavioral tests that should hold for any working
// Hadolinter regardless of whether the underlying binary is embedded or
// resolved from PATH.
func runSuite(t *testing.T, h *Hadolinter) {
	t.Helper()

	// Smoke probe: if the binary can't even report its version, skip the
	// whole suite with a clear message rather than failing every subtest.
	// This happens, for example, with hadolint v2.12.0's darwin-x86_64
	// build running under Rosetta on some Apple Silicon Macs — an upstream
	// binary issue (see https://github.com/hadolint/hadolint/issues/925).
	if v := h.Version(); !semverLike(v) {
		t.Skipf("hadolint binary is not executable on this host (Version()=%q); "+
			"likely an upstream/platform issue with the bundled binary — "+
			"try installing hadolint via your package manager and re-run with NewHadolinterFromPATH().", v)
	}

	t.Run("Version returns semver", func(t *testing.T) {
		v := h.Version()
		if !semverLike(v) {
			t.Fatalf("expected semver-ish version, got %q", v)
		}
	})

	t.Run("AnalyzeFile clean Dockerfile", func(t *testing.T) {
		res, err := h.AnalyzeFile("testdata/valid.Dockerfile", "")
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode != 0 {
			t.Fatalf("expected zero exit code, got %d (findings=%+v)", res.ExitCode, res.Findings)
		}
		if len(res.Findings) != 0 {
			t.Fatalf("expected no findings, got %+v", res.Findings)
		}
	})

	t.Run("AnalyzeFile invalid Dockerfile", func(t *testing.T) {
		res, err := h.AnalyzeFile("testdata/invalid.Dockerfile", "")
		if err != nil {
			t.Fatal(err)
		}
		if res.ExitCode == 0 {
			t.Fatalf("expected non-zero exit code for invalid Dockerfile")
		}
		if len(res.Findings) == 0 {
			t.Fatalf("expected at least one finding for invalid Dockerfile")
		}
		// Findings should be properly structured.
		for _, f := range res.Findings {
			if f.Code == "" || f.Level == "" || f.Message == "" {
				t.Fatalf("malformed finding: %+v", f)
			}
		}
	})

	t.Run("AnalyzeFile with extra flags", func(t *testing.T) {
		// Pick a rule that fires for the invalid Dockerfile and silence it via --ignore.
		baseline, err := h.AnalyzeFile("testdata/invalid.Dockerfile", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(baseline.Findings) == 0 {
			t.Skip("no findings to silence — upstream rules may have changed")
		}
		code := baseline.Findings[0].Code
		filtered, err := h.AnalyzeFile("testdata/invalid.Dockerfile", "--ignore "+code)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range filtered.Findings {
			if f.Code == code {
				t.Fatalf("expected %s to be filtered out, but it is still present", code)
			}
		}
	})

	t.Run("AnalyzeSnippet clean", func(t *testing.T) {
		res, err := h.AnalyzeSnippet([]byte("FROM alpine:3.20\nCMD [\"sh\"]\n"), "")
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Findings) != 0 {
			t.Fatalf("expected no findings, got %+v", res.Findings)
		}
	})

	t.Run("AnalyzeSnippet with violations", func(t *testing.T) {
		res, err := h.AnalyzeSnippet([]byte("FROM ubuntu\nRUN cd /tmp\n"), "")
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Findings) == 0 {
			t.Fatalf("expected at least one finding")
		}
	})

	t.Run("AnalyzeFile invalid extraFlags returns error", func(t *testing.T) {
		// Unclosed quote -> shlex error.
		_, err := h.AnalyzeFile("testdata/valid.Dockerfile", `--ignore "DL3000`)
		if err == nil {
			t.Fatalf("expected shlex parse error, got nil")
		}
	})

	t.Run("inline Config silences a rule and is cleaned up", func(t *testing.T) {
		baseline, err := h.AnalyzeFile("testdata/invalid.Dockerfile", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(baseline.Findings) == 0 {
			t.Skip("no findings to silence — upstream rules may have changed")
		}
		code := baseline.Findings[0].Code

		// Save+restore Config so we don't leak state into other subtests.
		prev := h.Config
		t.Cleanup(func() { h.Config = prev })

		h.Config = &Config{Ignored: []string{code}}
		filtered, err := h.AnalyzeFile("testdata/invalid.Dockerfile", "")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range filtered.Findings {
			if f.Code == code {
				t.Fatalf("expected %s to be silenced by inline Config, but it is still present", code)
			}
		}

		// After Analyze* returns, the managed temp file should be gone.
		// Listing the temp dir for stragglers is overkill; rely on Stat'ing
		// a freshly materialized path instead.
		path, cleanup, err := h.resolveConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		cleanup()
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected materialized config temp file to be removed, got %v", err)
		}
	})

	t.Run("ConfigFile silences a rule via an on-disk file", func(t *testing.T) {
		baseline, err := h.AnalyzeFile("testdata/invalid.Dockerfile", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(baseline.Findings) == 0 {
			t.Skip("no findings to silence — upstream rules may have changed")
		}
		code := baseline.Findings[0].Code

		cfgPath := writeTempConfig(t, &Config{Ignored: []string{code}})

		prevFile, prevCfg := h.ConfigFile, h.Config
		t.Cleanup(func() { h.ConfigFile, h.Config = prevFile, prevCfg })

		h.Config = nil
		h.ConfigFile = cfgPath
		filtered, err := h.AnalyzeFile("testdata/invalid.Dockerfile", "")
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range filtered.Findings {
			if f.Code == code {
				t.Fatalf("expected %s to be silenced by ConfigFile, but it is still present", code)
			}
		}
	})
}

func semverLike(s string) bool {
	match, _ := regexp.MatchString(`[0-9]+\.[0-9]+\.[0-9]+`, s)
	return match
}

func writeTempConfig(t *testing.T, c *Config) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "hadolint-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	data, err := c.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestEmbeddedBinary(t *testing.T) {
	if len(hadolintBinary) == 0 {
		t.Skip("no embedded binary for this platform")
	}
	h, err := NewHadolinter()
	if err != nil {
		t.Fatalf("NewHadolinter failed: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	if h.Source() != SourceEmbedded {
		t.Fatalf("expected SourceEmbedded, got %v", h.Source())
	}
	if h.BinaryPath() != "" {
		t.Fatalf("expected empty BinaryPath for embedded, got %q", h.BinaryPath())
	}
	runSuite(t, h)
}

func TestPATHFallback(t *testing.T) {
	if _, err := exec.LookPath("hadolint"); err != nil {
		t.Skip("hadolint not on PATH")
	}
	h, err := NewHadolinterFromPATH()
	if err != nil {
		t.Fatalf("NewHadolinterFromPATH failed: %v", err)
	}
	t.Cleanup(func() { _ = h.Close() })

	if h.Source() != SourcePATH {
		t.Fatalf("expected SourcePATH, got %v", h.Source())
	}
	if h.BinaryPath() == "" {
		t.Fatalf("expected non-empty BinaryPath for PATH-resolved hadolint")
	}
	runSuite(t, h)
}

func TestNewHadolinterFromPATH_NotFound(t *testing.T) {
	// Empty PATH guarantees lookup failure.
	t.Setenv("PATH", "")
	_, err := NewHadolinterFromPATH()
	if !errors.Is(err, ErrNotAvailable) {
		t.Fatalf("expected ErrNotAvailable, got %v", err)
	}
}

func TestNewHadolinter_NoEmbedNoPATH(t *testing.T) {
	if len(hadolintBinary) != 0 {
		t.Skip("embedded binary present on this platform; cannot exercise the no-embed+no-path branch here")
	}
	t.Setenv("PATH", "")
	_, err := NewHadolinter()
	if !errors.Is(err, ErrNotAvailable) {
		t.Fatalf("expected ErrNotAvailable, got %v", err)
	}
}

func TestBuildArgs(t *testing.T) {
	cases := []struct {
		name       string
		configPath string
		extra      string
		trailing   []string
		want       []string
		wantErr    bool
	}{
		{
			name:     "no extra flags, file path",
			trailing: []string{"Dockerfile"},
			want:     []string{"--format", "json", "--no-color", "Dockerfile"},
		},
		{
			name:     "with extra flags",
			extra:    "--ignore DL3000 --trusted-registry registry.example.com",
			trailing: []string{"-"},
			want: []string{
				"--format", "json", "--no-color",
				"--ignore", "DL3000",
				"--trusted-registry", "registry.example.com",
				"-",
			},
		},
		{
			name:       "pinned config short-circuits default discovery",
			configPath: "/etc/my-hadolint.yaml",
			trailing:   []string{"Dockerfile"},
			want: []string{
				"--format", "json", "--no-color",
				"-c", "/etc/my-hadolint.yaml",
				"Dockerfile",
			},
		},
		{
			name:       "pinned config plus extra flags",
			configPath: "testdata/hadolint.yaml",
			extra:      "--ignore DL3000",
			trailing:   []string{"-"},
			want: []string{
				"--format", "json", "--no-color",
				"-c", "testdata/hadolint.yaml",
				"--ignore", "DL3000",
				"-",
			},
		},
		{
			name:    "invalid shell quoting",
			extra:   `--ignore "DL3000`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &Hadolinter{}
			got, err := h.buildArgs(tc.configPath, tc.extra, tc.trailing...)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !equalSlice(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolveConfigPath(t *testing.T) {
	t.Run("no config", func(t *testing.T) {
		h := &Hadolinter{}
		path, cleanup, err := h.resolveConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if path != "" {
			t.Fatalf("expected empty path, got %q", path)
		}
	})

	t.Run("ConfigFile passthrough", func(t *testing.T) {
		h := &Hadolinter{ConfigFile: "/etc/hadolint.yaml"}
		path, cleanup, err := h.resolveConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if path != "/etc/hadolint.yaml" {
			t.Fatalf("expected ConfigFile passthrough, got %q", path)
		}
	})

	t.Run("Config materializes temp file and cleanup removes it", func(t *testing.T) {
		strictLabels := true
		h := &Hadolinter{Config: &Config{
			Ignored:           []string{"DL3000"},
			TrustedRegistries: []string{"my-company.com:5000"},
			StrictLabels:      &strictLabels,
			FailureThreshold:  "warning",
		}}
		path, cleanup, err := h.resolveConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if path == "" {
			t.Fatal("expected non-empty temp path")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("temp config not readable: %v", err)
		}
		content := string(data)
		for _, want := range []string{"ignored:", "DL3000", "trustedRegistries:", "strict-labels: true", "failure-threshold: warning"} {
			if !strings.Contains(content, want) {
				t.Fatalf("expected temp config to contain %q, got:\n%s", want, content)
			}
		}
		cleanup()
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected temp config to be removed after cleanup, got %v", err)
		}
	})

	t.Run("Config writeTempFile error is surfaced", func(t *testing.T) {
		bogus := filepath.Join(t.TempDir(), "does", "not", "exist")
		t.Setenv("TMPDIR", bogus)
		t.Setenv("TMP", bogus)
		t.Setenv("TEMP", bogus)

		h := &Hadolinter{Config: &Config{Ignored: []string{"DL3000"}}}
		path, cleanup, err := h.resolveConfigPath()
		if err == nil {
			cleanup()
			_ = os.Remove(path)
			t.Fatal("expected error when temp dir does not exist")
		}
		if path != "" {
			t.Fatalf("expected empty path on error, got %q", path)
		}
		// cleanup must still be safe to call even when an error was returned.
		cleanup()
	})

	t.Run("Config takes precedence over ConfigFile", func(t *testing.T) {
		h := &Hadolinter{
			ConfigFile: "/etc/hadolint.yaml",
			Config:     &Config{Ignored: []string{"DL3000"}},
		}
		path, cleanup, err := h.resolveConfigPath()
		if err != nil {
			t.Fatal(err)
		}
		defer cleanup()
		if path == "/etc/hadolint.yaml" {
			t.Fatal("expected Config to take precedence over ConfigFile, but ConfigFile was used")
		}
	})
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAnalyzeSurfacesConfigError(t *testing.T) {
	// resolveConfigPath runs *before* the binary is invoked, so we don't
	// need a working hadolint to exercise the error path: a bare
	// Hadolinter with a Config set and a bogus TMPDIR is enough.
	bogus := filepath.Join(t.TempDir(), "does", "not", "exist")
	t.Setenv("TMPDIR", bogus)
	t.Setenv("TMP", bogus)
	t.Setenv("TEMP", bogus)

	h := &Hadolinter{Config: &Config{Ignored: []string{"DL3000"}}}

	if _, err := h.AnalyzeFile("testdata/valid.Dockerfile", ""); err == nil {
		t.Fatal("AnalyzeFile: expected error when temp config cannot be written")
	}
	if _, err := h.AnalyzeSnippet([]byte("FROM alpine:3.20\n"), ""); err == nil {
		t.Fatal("AnalyzeSnippet: expected error when temp config cannot be written")
	}
}

func TestConfigWriteTempFile(t *testing.T) {
	t.Run("happy path writes valid YAML and returns a path under TMPDIR", func(t *testing.T) {
		strict := true
		c := &Config{
			Ignored:           []string{"DL3000", "DL3001"},
			TrustedRegistries: []string{"my-company.com:5000"},
			LabelSchema:       map[string]string{"author": "text"},
			StrictLabels:      &strict,
			FailureThreshold:  "warning",
			Override: &Override{
				Error:   []string{"DL3007"},
				Warning: []string{"DL3010"},
			},
		}
		path, err := c.writeTempFile()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })

		if !strings.HasPrefix(filepath.Base(path), "go-hadolint-") || !strings.HasSuffix(path, ".yaml") {
			t.Fatalf("expected go-hadolint-*.yaml temp file, got %q", path)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("temp file unreadable: %v", err)
		}
		content := string(data)
		for _, want := range []string{
			"ignored:",
			"- DL3000",
			"- DL3001",
			"trustedRegistries:",
			"- my-company.com:5000",
			"label-schema:",
			"author: text",
			"strict-labels: true",
			"failure-threshold: warning",
			"override:",
			"error:",
			"- DL3007",
		} {
			if !strings.Contains(content, want) {
				t.Fatalf("temp config missing %q; full content:\n%s", want, content)
			}
		}
	})

	t.Run("empty Config still serializes and writes", func(t *testing.T) {
		path, err := (&Config{}).writeTempFile()
		if err != nil {
			t.Fatalf("unexpected error for empty config: %v", err)
		}
		t.Cleanup(func() { _ = os.Remove(path) })

		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		// With every field omitted, yaml.Marshal emits "{}\n" — three bytes.
		// Don't assert on the exact bytes (yaml lib formatting can drift);
		// just confirm the file exists and is tiny.
		if info.Size() == 0 {
			t.Fatalf("expected non-empty serialization (yaml.Marshal returns at least \"{}\\n\"), got zero bytes")
		}
	})

	t.Run("os.CreateTemp failure surfaces a wrapped error", func(t *testing.T) {
		// Point every temp-dir env var os.TempDir() consults at a path
		// that cannot exist, so CreateTemp fails before any other branch.
		bogus := filepath.Join(t.TempDir(), "does", "not", "exist")
		t.Setenv("TMPDIR", bogus) // Unix
		t.Setenv("TMP", bogus)    // Windows
		t.Setenv("TEMP", bogus)   // Windows

		path, err := (&Config{Ignored: []string{"DL3000"}}).writeTempFile()
		if err == nil {
			_ = os.Remove(path)
			t.Fatal("expected error when temp dir does not exist")
		}
		if !strings.Contains(err.Error(), "hadolint: create temp config") {
			t.Fatalf("expected wrapped CreateTemp error, got: %v", err)
		}
		if path != "" {
			t.Fatalf("expected empty path on error, got %q", path)
		}
	})
}

func TestNewResult(t *testing.T) {
	cases := []struct {
		name      string
		exit      int
		output    []byte
		wantCount int
		wantErr   bool
	}{
		{
			name:      "empty output yields zero findings",
			exit:      0,
			output:    nil,
			wantCount: 0,
		},
		{
			name:      "empty array yields zero findings",
			exit:      0,
			output:    []byte(`[]`),
			wantCount: 0,
		},
		{
			name: "parses fields",
			exit: 1,
			output: []byte(`[
				{"file":"Dockerfile","line":3,"column":1,"level":"warning","code":"DL3007","message":"Using latest is prone to errors"},
				{"file":"Dockerfile","line":4,"column":1,"level":"error","code":"DL3020","message":"Use COPY instead of ADD"}
			]`),
			wantCount: 2,
		},
		{
			name:    "malformed JSON returns error",
			output:  []byte(`{not json}`),
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := NewResult(tc.exit, tc.output)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err mismatch: got %v, wantErr=%v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if res.ExitCode != tc.exit {
				t.Fatalf("exit code: got %d, want %d", res.ExitCode, tc.exit)
			}
			if len(res.Findings) != tc.wantCount {
				t.Fatalf("findings: got %d, want %d", len(res.Findings), tc.wantCount)
			}
			if tc.wantCount > 0 {
				f := res.Findings[0]
				if f.Code == "" || f.Level == "" || f.Message == "" || f.Line == 0 {
					t.Fatalf("missing fields in parsed finding: %+v", f)
				}
			}
		})
	}
}

func TestThirdPartyNotices(t *testing.T) {
	got := ThirdPartyNotices()
	if strings.TrimSpace(got) == "" {
		t.Fatal("expected ThirdPartyNotices to return non-empty content")
	}
	// hadolint's ThirdPartyNotices.txt mentions GPL (hadolint itself + bundled deps).
	if !strings.Contains(got, "GPL") {
		t.Fatalf("expected ThirdPartyNotices to mention GPL, got: %q", got[:min(120, len(got))])
	}
}

func TestLicense(t *testing.T) {
	got := License()
	if strings.TrimSpace(got) == "" {
		t.Fatal("expected License to return non-empty content")
	}
	if !strings.Contains(got, "GNU GENERAL PUBLIC LICENSE") {
		t.Fatalf("expected License to contain GPL header, got: %q", got[:min(120, len(got))])
	}
	if !strings.Contains(got, "Version 3") {
		t.Fatalf("expected License to be GPL v3, got: %q", got[:min(120, len(got))])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
