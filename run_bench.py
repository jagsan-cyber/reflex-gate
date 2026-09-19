#!/usr/bin/env python3
"""Call the JEV API from anywhere and score the labeled dataset.

Example (another machine):
  python run_bench.py --jev-url http://192.168.1.20:8090
"""

from __future__ import annotations

import argparse
import json
import time
from pathlib import Path
from typing import Any

import httpx

ROOT = Path(__file__).resolve().parent


def bucket_prompt_tokens(n: int) -> str:
    for edge in (1000, 2000, 4000, 8000):
        if n <= edge * 1.25:
            return f"{edge // 1000}K"
    return "8K+"


def post(client: httpx.Client, path: str, payload: dict[str, Any]) -> tuple[dict[str, Any], float]:
    t0 = time.perf_counter()
    r = client.post(path, json=payload)
    wall = time.perf_counter() - t0
    if r.status_code >= 400:
        raise RuntimeError(f"{path} HTTP {r.status_code}: {r.text[:400]}")
    return r.json(), wall


def metrics_from(body: dict[str, Any], wall_s: float) -> dict[str, Any]:
    m = body.get("metrics") or {}
    n_out = int(m.get("completion_tokens") or 0)
    decode_s = float(m.get("decode_s") or 0.0)
    tok_s = float(m.get("tok_s") or 0.0)
    if tok_s <= 0 and decode_s > 0 and n_out:
        tok_s = n_out / decode_s
    return {
        "ttft_s": float(m.get("ttft_s") or 0.0),
        "decode_s": decode_s,
        "total_s": float(m.get("total_s") or 0.0),
        "client_wall_s": wall_s,
        "prompt_tokens": int(m.get("prompt_tokens") or 0),
        "completion_tokens": n_out,
        "tok_s": tok_s,
    }


def main() -> None:
    parser = argparse.ArgumentParser(description="Benchmark a remote JEV API")
    parser.add_argument("--jev-url", default="http://127.0.0.1:8090", help="JEV API base, no trailing slash")
    parser.add_argument("--dataset", type=Path, default=ROOT / "data" / "jev_dataset.jsonl")
    parser.add_argument("--out", type=Path, default=ROOT / "results" / "raw.jsonl")
    parser.add_argument("--timeout", type=float, default=600.0)
    args = parser.parse_args()
    base = args.jev_url.rstrip("/")

    health = httpx.get(f"{base}/health", timeout=10.0)
    health.raise_for_status()
    info = health.json()
    print(f"jev={base} llm_ok={info.get('llm_ok')} model={info.get('model')}")
    if not info.get("llm_ok"):
        raise SystemExit(f"JEV API cannot reach llama-server: {info}")

    cases = []
    with args.dataset.open(encoding="utf-8") as f:
        for line in f:
            if line.strip():
                cases.append(json.loads(line))

    args.out.parent.mkdir(parents=True, exist_ok=True)
    rows: list[dict[str, Any]] = []

    def emit(row: dict[str, Any]) -> None:
        rows.append(row)
        print(
            f"{row['id']:10} ttft={row['ttft_s']:.3f}s tot={row['total_s']:.3f}s "
            f"out={row['completion_tokens']} tps={row['tok_s']:.1f} "
            f"acc={row.get('accurate')} cmp={row.get('compliant')}"
        )

    with httpx.Client(base_url=base, timeout=args.timeout) as http:
        for n, max_tok, prompt in (
            (5, 5, "Reply with exactly these five tokens separated by spaces: one two three four five"),
            (30, 30, "Count from 1 to 30 using digits only, separated by spaces. Stop at 30."),
            (100, 150, "Count from 1 to 100 using digits only, separated by spaces. Stop at 100."),
        ):
            body, wall = post(http, "/jev/raw", {"prompt": prompt, "max_tokens": max_tok})
            emit(
                {
                    "id": f"D-{n:03d}",
                    "task": "D",
                    "gold": None,
                    "accurate": None,
                    "compliant": None,
                    "decode_target": n,
                    "text": body.get("text", ""),
                    **metrics_from(body, wall),
                }
            )

        for case in cases:
            try:
                if case["task"] == "A":
                    # Dataset prompt wraps the log; send the inner user content as log.
                    log = case["prompt"]
                    body, wall = post(http, "/jev/stop", {"log": log})
                    pred = "Yes" if body.get("stop") else "No"
                    acc = pred.lower() == str(case.get("gold")).lower()
                    emit(
                        {
                            "id": case["id"],
                            "task": "A",
                            "gold": case.get("gold"),
                            "accurate": acc,
                            "compliant": True,
                            "text": body.get("raw", ""),
                            **metrics_from(body, wall),
                        }
                    )
                elif case["task"] == "B":
                    log = case["prompt"]
                    body, wall = post(http, "/jev/extract", {"log": log})
                    got = body.get("result") or {}
                    acc = got == case.get("gold_json")
                    emit(
                        {
                            "id": case["id"],
                            "task": "B",
                            "gold_json": case.get("gold_json"),
                            "accurate": acc,
                            "compliant": True,
                            "text": body.get("raw", ""),
                            **metrics_from(body, wall),
                        }
                    )
                else:
                    log = case["prompt"]
                    body, wall = post(http, "/jev/scan", {"log": log})
                    line = (body.get("line") or "").strip()
                    gold = case.get("gold") or ""
                    acc = gold in line
                    cmp = line == gold
                    m = metrics_from(body, wall)
                    emit(
                        {
                            "id": case["id"],
                            "task": "C",
                            "gold": gold,
                            "accurate": acc,
                            "compliant": cmp,
                            "text": line,
                            "prompt_bucket": bucket_prompt_tokens(m["prompt_tokens"] or case.get("target_tokens") or 0),
                            "target_tokens": case.get("target_tokens"),
                            **m,
                        }
                    )
            except Exception as exc:
                emit(
                    {
                        "id": case["id"],
                        "task": case["task"],
                        "gold": case.get("gold"),
                        "gold_json": case.get("gold_json"),
                        "accurate": False,
                        "compliant": False,
                        "text": "",
                        "ttft_s": 0.0,
                        "decode_s": 0.0,
                        "total_s": 0.0,
                        "client_wall_s": 0.0,
                        "prompt_tokens": 0,
                        "completion_tokens": 0,
                        "tok_s": 0.0,
                        "error": str(exc),
                    }
                )

    with args.out.open("w", encoding="utf-8") as f:
        for row in rows:
            f.write(json.dumps(row, ensure_ascii=False) + "\n")
    meta = {
        "base_url": base,
        "model": info.get("model"),
        "n_rows": len(rows),
        "no_think": True,
        "via": "jev-api",
    }
    (args.out.parent / "meta.json").write_text(json.dumps(meta, indent=2), encoding="utf-8")
    print(f"wrote {len(rows)} rows -> {args.out}")


if __name__ == "__main__":
    main()
