package hadolint

import "encoding/json"

// Result captures the outcome of a hadolint invocation.
type Result struct {
	ExitCode  int
	RawResult []byte
	Findings  []Finding
}

// Finding represents a single hadolint diagnostic entry.
//
// See the upstream hadolint JSON format for field semantics:
// https://github.com/hadolint/hadolint
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (r *Result) parseJSON() ([]Finding, error) {
	findings := make([]Finding, 0)
	if len(r.RawResult) == 0 {
		return findings, nil
	}
	if err := json.Unmarshal(r.RawResult, &findings); err != nil {
		return nil, err
	}
	return findings, nil
}

// NewResult parses the raw hadolint JSON output into a Result.
func NewResult(exitCode int, output []byte) (*Result, error) {
	r := &Result{
		ExitCode:  exitCode,
		RawResult: output,
	}
	findings, err := r.parseJSON()
	if err != nil {
		return nil, err
	}
	r.Findings = findings
	return r, nil
}
