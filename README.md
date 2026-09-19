# local-jev

A tiny **Judge / Evaluator / Verifier** HTTP API for local agent loops.

The schema lives on the server. Callers send a log. The API returns a structured verdict.

- `/jev/stop` — Yes/No loop-exit gate (local LLM + GBNF `Yes|No`)
- `/jev/extract` — tool-argument JSON (local LLM + JSON Schema)
- `/jev/scan` — `[ERROR]` / `[FATAL]` line (regex, not the LLM)

Designed for a small local model such as Qwen3.5-0.8B in front of [llama.cpp](https://github.com/ggml-org/llama.cpp) `llama-server`.

## Why this split

| Endpoint | Backend | Why |
|---|---|---|
| stop | LLM | 0.8B scored 30/30 on labeled exit logs |
| extract | LLM + JSON Schema | 30/30; `"none"` is rejected in favor of `null` |
| scan | regex | LLM scan leaked JSON grammar across slots and collapsed at 4K+ with quantized KV |

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
