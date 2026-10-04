# bqnbench — how well do LLMs write BQN?

A benchmark that measures LLM code generation for
[BQN](https://github.com/mlochbaum/BQN) — an array language that is **not**
APL, no matter how confidently the models write it.

The oracle is real execution: every model answer is compiled and run in
[CBQN](https://github.com/dzaima/CBQN) against gold input/output pairs with
exact (`≡`) value matching — shape, depth and type included.

## First results

23 tasks, 3 correctness categories, temperature 0, exact-match scoring.
Models: two small local LLMs (Q4_K_M, one RTX 4070 SUPER, llama.cpp):

| model | passed |
| --- | --- |
| MiniCPM5-2B | 1/23 |
| VibeThinker-3B | 0/23 |

The one pass was `+´` (sum). The dominant failure mode: models answer in
**APL dialect** — `→` arrows, `: ` dfn headers, 1-indexed instincts — which
BQN rejects at compile time. Exactly the gap a measured benchmark is supposed
to expose.

Run your own models against it — see Usage.

## How it works

1. **Dataset** — `tasks/tasks.json`: 23 tasks in three categories
   (`primitives`, `idioms`, `algorithms`), each with a reference solution
   verified in CBQN and 1–3 test cases (`expr` must `≡ expect`).
2. **Generation** — each task prompt is sent to an OpenAI-compatible
   chat endpoint (llama.cpp works out of the box); temperature 0.
3. **Evaluation** — the extracted code is wrapped with each test into a
   standalone BQN program; the program prints `CHECK 1` iff the result
   **matches exactly** (`≡`). Compilation errors, timeouts and NaN-poisoned
   answers all fail.
4. **Scoring** — a task passes when all its tests pass; results are written
   per model as JSON and rendered to a markdown scoreboard.

Optional: a static layer via [`bqnlsp`](https://github.com/Grshor/bqnlsp)
(`check --json` — compile status plus inferred shape/type information) when a
bqnlsp binary is available.

## Usage

```sh
go build -o bqnbench ./cmd/bqnbench

# 1. self-check the dataset (reference solutions must pass their own tests)
./bqnbench verify -tasks tasks/tasks.json -cbqn /path/to/cbqn

# 2. point it at models (llama.cpp: /v1/chat/completions, any port)
cat > models.json <<'EOF'
{
  "models": [
    { "name": "minicpm5-2b", "base_url": "http://127.0.0.1:8091/v1", "model": "MiniCPM5-2B-Q4_K_M.gguf" },
    { "name": "gpt", "base_url": "https://api.openai.com/v1", "model": "gpt-4o", "api_key_env": "OPENAI_API_KEY" }
  ]
}
EOF

# 3. run + score
./bqnbench run -models models.json -out results -cbqn /path/to/cbqn
./bqnbench score -results results
```

`verify` first: every reference solution in the dataset is executed against
its own tests — the benchmark cannot drift from reality.

## Dataset

23 tasks across three categories:

- **primitives** — one-liners with core builtins: sum, max, reverse, range,
  sort up/down, unique, mean, row sums, transpose, shape
- **idioms** — running sums, consecutive diffs, rotate, take-last, drop,
  character count, string repeat, unique-with-counts
- **algorithms** — Fibonacci (recursion with `𝕊`), median, FizzBuzz

Every reference solution and every expected value in `tasks/tasks.json` was
executed and verified in CBQN before landing in the dataset.

## Why

Niche languages are where LLM benchmarks are thinnest, and array languages are
the thinnest corner of that: models train on APL/J/K content and produce
confident, wrong dialect. A measured scoreboard is the cheapest way to make
that visible — for users deciding whether to trust the model, and for labs
deciding what to train on.

The companion project [bqnlsp](https://github.com/Grshor/bqnlsp) (BQN language
server with shape/type inference) doubles as the static-analysis oracle.

## License

MIT
