// Package bench — run orchestration and scoring.
package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Grshor/bqnbench/internal/llm"
	"github.com/Grshor/bqnbench/internal/runner"
)

// TestResult is the outcome of one test evaluation.
type TestResult struct {
	Pass bool   `json:"pass"`
	Out  string `json:"out,omitempty"` // cbqn stdout (CHECK line / error)
}

// TaskResult is the outcome of one task for one model.
type TaskResult struct {
	ID       string       `json:"id"`
	Pass     bool         `json:"pass"`
	StaticOK *bool        `json:"static_ok,omitempty"` // nil when bqnlsp absent
	Tests    []TestResult `json:"tests"`
	Err      string       `json:"error,omitempty"`
}

// ModelResult aggregates one model's run.
type ModelResult struct {
	Model  string       `json:"model"`
	Passed int          `json:"passed"`
	Total  int          `json:"total"`
	Tasks  []TaskResult `json:"tasks"`
}

// RunModel evaluates every task against one model.
func RunModel(ctx context.Context, m llm.Model, tasks []Task, cbqn runner.CBQN, lsp runner.LSP, timeout time.Duration, concurrency int) (ModelResult, error) {
	res := ModelResult{Model: m.Name, Total: len(tasks)}
	sem := make(chan struct{}, concurrency)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, task := range tasks {
		wg.Add(1)
		go func(t Task) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			tr := TaskResult{ID: t.ID}
			messages := []llm.Message{
				{Role: "system", Content: "You are an expert BQN programmer. Reply with ONLY the BQN code that solves the task — no explanation, no markdown fences. Assign the answer to the name the task specifies."},
				{Role: "user", Content: t.Prompt},
			}
			resp, err := llm.Chat(ctx, m, messages, 0, 1024)
			if err != nil {
				tr.Err = fmt.Sprintf("llm: %v", err)
				mu.Lock()
				res.Tasks = append(res.Tasks, tr)
				mu.Unlock()
				return
			}
			code := ExtractCode(resp)

			staticOK, skipped, serr := lsp.Check(ctx, code)
			if serr != nil {
				tr.Err = fmt.Sprintf("static: %v", serr)
			} else if !skipped {
				tr.StaticOK = &staticOK
			}

			allPass := true
			for _, test := range t.Tests {
				prog := BuildProgram(code, test)
				out, err := cbqn.Run(ctx, prog, timeout)
				pass := err == nil && strings.Contains(out, "CHECK 1")
				if err != nil {
					allPass = false
					tr.Tests = append(tr.Tests, TestResult{Pass: false, Out: out + err.Error()})
					continue
				}
				tr.Tests = append(tr.Tests, TestResult{Pass: pass, Out: strings.TrimSpace(out)})
				if !pass {
					allPass = false
				}
			}
			tr.Pass = allPass && tr.Err == ""
			mu.Lock()
			if tr.Pass {
				res.Passed++
			}
			res.Tasks = append(res.Tasks, tr)
			mu.Unlock()
		}(task)
	}
	wg.Wait()
	return res, nil
}

// SaveResult writes one ModelResult as JSON under dir.
func SaveResult(dir string, r ModelResult) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, sanitize(r.Model)+".json")
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(b, '\n'), 0o644)
}

func sanitize(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, s)
}
