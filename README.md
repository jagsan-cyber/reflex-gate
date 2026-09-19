# local-jev

A tiny **Judge / Evaluator / Verifier** HTTP API for local agent loops.

The schema lives on the server. Callers send a log. The API returns a structured verdict.

- `/jev/stop` -> Yes/No loop-exit gate (local LLM + 1-line CoT + GBNF)
- `/jev/extract` -> tool-argument JSON (local LLM + JSON Schema)
- `/jev/scan` -> security / safety scanner (Hybrid: <1ms regex secret shield + LLM semantic scanner)

Designed for a compact local model such as `Qwen2.5-Coder-1.5B-Instruct` in front of [llama.cpp](https://github.com/ggml-org/llama.cpp) `llama-server`.

## Benchmark Results: 100.0% (111/111 Pass)

Evaluated across 39 distinct test cases x 3 passes (111 total evaluations) spanning subtle reasoning, schema extraction, and semantic safety scanning:

### Accuracy Progression

| Run | Score | Key Milestone |
|---|---|---|
| 1 | 89.2% | Initial baseline |
| 2 | 91.9% | Schema loosen & token length extension |
| 3 | 86.5% | Anti-hallucination hedging fluctuation |
| 4 | 94.6% | Finding-first CoT for Task C |
| 5 | 97.3% | Context protection & status priority tuning |
| 6 | **100.0% (111/111)** | **Hybrid Secret Shield (C-C4 resolved, zero hallucination)** |

### Performance Metrics (p50 Latency & Throughput)

| Metric | Previous (Run 5) | Final (Run 6) |
|---|---|---|
| Stop p50 | 0.68s | **0.65s** |
| Extract p50 | 1.18s | **1.14s** |
| Scan p50 | 0.59s | **0.58s** |
| TTFT p50 | 66ms | **65ms** |
| Decode Speed | 37.7 tok/s | **38.7 tok/s** |
| Sustained Throughput | 2.5 req/s | **2.4 req/s** (3-slot saturation) |

## Why this split

| Endpoint | Backend | Why |
|---|---|---|
| stop | LLM (1-line CoT) | High accuracy on nuanced exits; explains why before outputting Yes/No |
| extract | LLM + JSON Schema | Strict typed JSON; rejects `"none"` for `null`, preserves literal error codes |
| scan | Hybrid (Fast-path + LLM) | Regex shields plaintext API keys/PATs in <1ms; LLM semantically detects hidden crashes & prompt injections |

Measured on Qwen3.5-0.8B Q8_0, ROCm, **KV cache f16/f16** (not turbo3/q8): stop ~0.2s, extract ~0.7s, scan ~0ms.

Do **not** use `--cache-type-v turbo3` for this 0.8B path: long-context scan produced `????`. Do **not** use llama.cpp `--no-cache-prompt` on ROCm for this stack: generation collapsed.

## Quick start

1. Run any OpenAI-compatible `llama-server` on port 8080 (Q8_0 or better, KV f16 recommended for 0.8B).
2. Start the JEV proxy:

```bash
python -m venv .venv
# Windows: .venv\Scripts\activate
# Unix:    source .venv/bin/activate
pip install -r requirements.txt
set JEV_LLM_BASE_URL=http://127.0.0.1:8080/v1
python jev_api.py
```

3. Open the inspector: http://127.0.0.1:8090/demo

```bash
curl -s http://127.0.0.1:8090/jev/stop \
  -H "Content-Type: application/json" \
  -d "{\"log\":\"pytest: 12 passed\\nexit_code=0\"}"
```

Environment: `JEV_LLM_BASE_URL`, `JEV_HOST` (default `0.0.0.0`), `JEV_PORT` (default `8090`).

## Benchmark from another machine

```bash
python generate_dataset.py
python run_bench.py --jev-url http://<host>:8090
python report.py
```

## License

MIT. llama.cpp is a runtime dependency, not bundled here.
