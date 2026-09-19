# go/

Go port of local-jev. Python in the repo root stays as the reference (`python-baseline`).

## Intended layout

```
go/
  cmd/jevapi/     HTTP proxy (stop / extract / scan / systemone)
  internal/api/   handlers, llama-server client, slots
  internal/schema GBNF, JSON schema, softmax
  internal/gui/   later: drag-and-drop llama.cpp + Qwen3.5-0.8B, then Run
```

## GUI goal (not implemented yet)

1. Drop `llama-server` (or the llama.cpp bin dir).
2. Drop `Qwen3.5-0.8B` GGUF.
3. Press Run: start llama-server (`--parallel 2`, KV f16) and the JEV API, open the inspector.

Implementation waits for the next instruction.
