// Package bench orchestrates dataset → model → BQN evaluation.
package bench

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Task is one benchmark problem with its verified reference tests.
type Task struct {
	ID       string `json:"id"`
	Category string `json:"category"`
	Prompt   string `json:"prompt"`
	Solution string `json:"solution"` // verified reference solution
	Tests    []Test `json:"tests"`
}

// Test is one correctness check: expr must evaluate to expect.
type Test struct {
	Expr   string `json:"expr"`
	Expect string `json:"expect"`
}

// LoadTasks reads a dataset file.
func LoadTasks(path string) ([]Task, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tasks []Task
	if err := json.Unmarshal(b, &tasks); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	for _, t := range tasks {
		if t.ID == "" || len(t.Tests) == 0 {
			return nil, fmt.Errorf("task with empty id or tests: %+v", t.ID)
		}
	}
	return tasks, nil
}

// BuildProgram wraps candidate code and one test into a self-contained BQN
// program: the test passes iff it prints "CHECK 1".
func BuildProgram(code string, t Test) string {
	var b strings.Builder
	b.WriteString("{\n")
	b.WriteString(code)
	b.WriteString("\n•Out (\"CHECK \" ∾ •Fmt ((")
	b.WriteString(t.Expr)
	b.WriteString(") ≡ (")
	b.WriteString(t.Expect)
	b.WriteString(")))\n}\n")
	return b.String()
}

// ExtractCode pulls the BQN code out of a model response: prefer the first
// markdown code block; otherwise use the whole trimmed reply. A bare
// expression (no "Sol ←") gets wrapped.
func ExtractCode(resp string) string {
	r := strings.TrimSpace(resp)
	if i := strings.Index(r, "```"); i >= 0 {
		rest := r[i+len("```"):]
		rest = strings.TrimPrefix(rest, "bqn")
		if j := strings.Index(rest, "```"); j >= 0 {
			r = strings.TrimSpace(rest[:j])
		}
	}
	if !strings.Contains(r, "Sol") {
		r = "Sol ← " + r
	}
	return r
}
