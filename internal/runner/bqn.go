// Package runner executes BQN code (via the cbqn binary) and optionally
// asks bqnlsp for a static compile check.
package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CBQN runs BQN programs through the cbqn binary.
type CBQN struct {
	Bin string // path to the cbqn interpreter
}

// Run writes src to a temp file and executes it, returning stdout.
// A non-zero exit, timeout, or missing binary is an error.
func (c CBQN) Run(ctx context.Context, src string, timeout time.Duration) (string, error) {
	if c.Bin == "" {
		return "", fmt.Errorf("cbqn binary path is empty")
	}
	dir, err := os.MkdirTemp("", "bqnbench-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(dir)
	prog := filepath.Join(dir, "prog.bqn")
	if err := os.WriteFile(prog, []byte(src), 0o644); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.Bin, prog)
	var out strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return out.String(), fmt.Errorf("cbqn: %w", err)
	}
	return out.String(), nil
}

// LSP asks bqnlsp for a static compile check. Optional: leave Bin empty to
// skip the static layer entirely.
type LSP struct {
	Bin string // path to the bqnlsp binary
}

// CheckReport is the subset of `bqnlsp check --json` the bench consumes.
type CheckReport struct {
	CompileOK   bool     `json:"compile_ok"`
	Diagnostics []string `json:"-"`
}

// Check compiles src with bqnlsp and reports whether it compiles cleanly
// (zero diagnostics). If LSP.Bin is empty, returns ok=true, skipped=true.
func (l LSP) Check(ctx context.Context, src string) (ok, skipped bool, err error) {
	if l.Bin == "" {
		return true, true, nil
	}
	dir, err := os.MkdirTemp("", "bqnbench-lsp-*")
	if err != nil {
		return false, false, err
	}
	defer os.RemoveAll(dir)
	prog := filepath.Join(dir, "prog.bqn")
	if err := os.WriteFile(prog, []byte(src), 0o644); err != nil {
		return false, false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, l.Bin, "check", "--json", prog).Output()
	if err != nil {
		// bqnlsp check exits non-zero on diagnostics; the JSON still parses.
		if len(out) == 0 {
			return false, false, fmt.Errorf("bqnlsp: %w", err)
		}
	}
	rep := CheckReport{}
	if err := json.Unmarshal(out, &rep); err != nil {
		return false, false, fmt.Errorf("bqnlsp report: %w", err)
	}
	return rep.CompileOK && len(rep.Diagnostics) == 0, false, nil
}
