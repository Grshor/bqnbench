// Command bqnbench measures how well LLMs write BQN.
//
// Subcommands:
//
//	run    — evaluate models from a config against the dataset
//	score  — print a markdown scoreboard from result files
//	verify — self-check: the dataset's reference solutions must pass
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Grshor/bqnbench/internal/bench"
	"github.com/Grshor/bqnbench/internal/llm"
	"github.com/Grshor/bqnbench/internal/runner"
)

type modelsFile struct {
	Models []llm.Model `json:"models"`
}

func main() { os.Exit(run()) }

func run() int {
	if len(os.Args) < 2 {
		usage()
		return 2
	}
	switch os.Args[1] {
	case "run":
		return cmdRun(os.Args[2:])
	case "score":
		return cmdScore(os.Args[2:])
	case "verify":
		return cmdVerify(os.Args[2:])
	default:
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `bqnbench — how well do LLMs write BQN?

Usage:
  bqnbench run    -tasks tasks.json -models models.json [-out results] [flags]
  bqnbench score  -results results
  bqnbench verify -tasks tasks.json [-cbqn cbqn]

run flags:
  -cbqn PATH     cbqn interpreter (default "cbqn" on PATH)
  -bqnlsp PATH   bqnlsp binary for static checks (optional, default: skip)
  -timeout D     per-test execution timeout (default 10s)
  -c N           concurrent tasks per model (default 4)`)
	return
}

func cmdRun(args []string) int {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	tasksPath := fs.String("tasks", "tasks/tasks.json", "dataset file")
	modelsPath := fs.String("models", "models.json", "model config file")
	outDir := fs.String("out", "results", "output directory for result JSON files")
	cbqn := fs.String("cbqn", "cbqn", "path to the cbqn interpreter")
	bqnlsp := fs.String("bqnlsp", "", "path to bqnlsp binary (optional static checks)")
	timeout := fs.Duration("timeout", 10*time.Second, "per-test execution timeout")
	conc := fs.Int("c", 4, "concurrent tasks per model")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	tasks, err := bench.LoadTasks(*tasksPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tasks:", err)
		return 2
	}
	mf, err := os.ReadFile(*modelsPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "models:", err)
		return 2
	}
	var mfile modelsFile
	if err := json.Unmarshal(mf, &mfile); err != nil {
		fmt.Fprintln(os.Stderr, "models:", err)
		return 2
	}

	ctx := context.Background()
	cbn := runner.CBQN{Bin: *cbqn}
	lsp := runner.LSP{Bin: *bqnlsp}

	fmt.Fprintf(os.Stderr, "bqnbench: %d tasks, %d models\n", len(tasks), len(mfile.Models))
	for _, m := range mfile.Models {
		res, err := bench.RunModel(ctx, m, tasks, cbn, lsp, *timeout, *conc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bqnbench: %s: %v\n", m.Name, err)
			return 1
		}
		path, err := bench.SaveResult(*outDir, res)
		if err != nil {
			fmt.Fprintln(os.Stderr, "save:", err)
			return 1
		}
		fmt.Fprintf(os.Stderr, "%s: %d/%d → %s\n", m.Name, res.Passed, res.Total, path)
	}
	return 0
}

func cmdScore(args []string) int {
	fs := flag.NewFlagSet("score", flag.ExitOnError)
	dir := fs.String("results", "results", "results directory")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	entries, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	fmt.Println("| model | passed |")
	fmt.Println("| --- | --- |")
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(*dir, e.Name()))
		if err != nil {
			continue
		}
		var r bench.ModelResult
		if json.Unmarshal(b, &r) != nil {
			continue
		}
		fmt.Printf("| %s | %d/%d |\n", r.Model, r.Passed, r.Total)
	}
	return 0
}

func cmdVerify(args []string) int {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	tasksPath := fs.String("tasks", "tasks/tasks.json", "dataset file")
	cbqn := fs.String("cbqn", "cbqn", "path to the cbqn interpreter")
	timeout := fs.Duration("timeout", 10*time.Second, "per-test timeout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	tasks, err := bench.LoadTasks(*tasksPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	cbn := runner.CBQN{Bin: *cbqn}
	bad := 0
	ctx := context.Background()
	for _, t := range tasks {
		for i, test := range t.Tests {
			prog := bench.BuildProgram("Sol ← "+t.Solution, test)
			out, err := cbn.Run(ctx, prog, *timeout)
			if err != nil || !contains(out, "CHECK 1") {
				fmt.Fprintf(os.Stderr, "FAIL %s#%d: %s%s\n", t.ID, i, out, err)
				bad++
			}
		}
	}
	if bad > 0 {
		fmt.Fprintf(os.Stderr, "dataset self-check: %d failing checks\n", bad)
		return 1
	}
	fmt.Println("dataset self-check: all reference solutions pass")
	return 0
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
