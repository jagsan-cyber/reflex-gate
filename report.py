#!/usr/bin/env python3
"""Aggregate JEV benchmark JSONL into Markdown tables."""

from __future__ import annotations

import argparse
import json
from pathlib import Path

import numpy as np

ROOT = Path(__file__).resolve().parent


def load_rows(path: Path) -> list[dict]:
    rows = []
    with path.open(encoding="utf-8") as f:
        for line in f:
            if line.strip():
                rows.append(json.loads(line))
    return rows


def md_table(headers: list[str], lines: list[list[str]]) -> str:
    out = ["| " + " | ".join(headers) + " |"]
    out.append("| " + " | ".join("---" for _ in headers) + " |")
    for row in lines:
        out.append("| " + " | ".join(row) + " |")
    return "\n".join(out)


def mean_std(xs: list[float]) -> str:
    if not xs:
        return "-"
    a = np.array(xs, dtype=float)
    if len(a) == 1:
        return f"{a[0]:.3f}"
    return f"{a.mean():.3f} +/- {a.std(ddof=1):.3f}"


def pct(xs: list[bool]) -> str:
    if not xs:
        return "-"
    return f"{100.0 * sum(1 for x in xs if x) / len(xs):.1f}% ({sum(1 for x in xs if x)}/{len(xs)})"


def build_report(rows: list[dict], meta: dict) -> str:
    by_task: dict[str, list[dict]] = {}
    for r in rows:
        by_task.setdefault(r["task"], []).append(r)

    parts = [
        "# JEV benchmark result",
        "",
        f"- endpoint: `{meta.get('base_url', '')}`",
        f"- model: `{meta.get('model', '')}`",
        f"- enable_thinking: `{not meta.get('no_think', True)}`",
        f"- n: {len(rows)}",
        "",
        "## 1. TTFT by prompt length (prefill)",
        "",
    ]

    buckets = ["1K", "2K", "4K", "8K"]
    ttft_rows = []
    for b in buckets:
        xs = [
            r["ttft_s"]
            for r in rows
            if r.get("task") == "C" and r.get("prompt_bucket") == b
        ]
        toks = [
            r["prompt_tokens"]
            for r in rows
            if r.get("task") == "C" and r.get("prompt_bucket") == b
        ]
        ttft_rows.append(
            [
                b,
                str(len(xs)),
                f"{np.mean(toks):.0f}" if toks else "-",
                mean_std(xs),
                f"{(np.mean(toks) / np.mean(xs)):.1f}" if xs and np.mean(xs) > 0 else "-",
            ]
        )
    parts.append(
        md_table(
            ["prompt", "n", "mean prompt tok", "TTFT s", "prefill tok/s"],
            ttft_rows,
        )
    )

    parts += ["", "## 2. Decode speed (short generation)", ""]
    dec_lines = []
    for n in (5, 30, 100):
        xs = [r for r in by_task.get("D", []) if int(r.get("decode_target") or 0) == n]
        if not xs:
            # fallback: task A ~5, B ~30, C ~100
            continue
        r = xs[0]
        dec_lines.append(
            [
                f"{n} tok target",
                str(r["completion_tokens"]),
                f"{r['ttft_s']:.3f}",
                f"{r['decode_s']:.3f}",
                f"{r['total_s']:.3f}",
                f"{r['tok_s']:.2f}",
            ]
        )
    if not dec_lines:
        mapping = [("A", "5 tok-ish"), ("B", "30 tok-ish"), ("C", "150 tok cap")]
        for task, label in mapping:
            xs = by_task.get(task, [])
            if not xs:
                continue
            dec_lines.append(
                [
                    label,
                    f"{np.mean([r['completion_tokens'] for r in xs]):.1f}",
                    mean_std([r["ttft_s"] for r in xs]),
                    mean_std([r["decode_s"] for r in xs]),
                    mean_std([r["total_s"] for r in xs]),
                    f"{np.mean([r['tok_s'] for r in xs]):.2f}",
                ]
            )
    parts.append(
        md_table(
            ["setting", "out tok", "TTFT s", "decode s", "total s", "tok/s"],
            dec_lines,
        )
    )

    parts += ["", "## 3. Accuracy", ""]
    acc_lines = []
    for task, name in (("A", "Yes/No stop"), ("B", "JSON extract"), ("C", "needle line")):
        xs = by_task.get(task, [])
        acc_lines.append([task, name, str(len(xs)), pct([bool(r["accurate"]) for r in xs])])
    parts.append(md_table(["task", "name", "n", "accuracy"], acc_lines))

    parts += ["", "## 4. Format compliance", ""]
    cmp_lines = []
    for task, name in (
        ("A", "Yes/No word only"),
        ("B", "JSON schema"),
        ("C", "line only, no preamble"),
    ):
        xs = by_task.get(task, [])
        cmp_lines.append([task, name, str(len(xs)), pct([bool(r["compliant"]) for r in xs])])
    parts.append(md_table(["task", "name", "n", "compliance"], cmp_lines))

    parts += ["", "## Failures", ""]
    fails = [r for r in rows if r.get("accurate") is False]
    if not fails:
        parts.append("None.")
    else:
        fail_lines = []
        for r in fails[:20]:
            preview = (r.get("text") or "").replace("\n", " ")[:80]
            fail_lines.append([r["id"], r["task"], preview])
        parts.append(md_table(["id", "task", "output preview"], fail_lines))
        if len(fails) > 20:
            parts.append(f"\n... {len(fails) - 20} more")

    return "\n".join(parts) + "\n"


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--raw", type=Path, default=ROOT / "results" / "raw.jsonl")
    parser.add_argument("--meta", type=Path, default=ROOT / "results" / "meta.json")
    parser.add_argument("--out", type=Path, default=ROOT / "benchmark_result.md")
    args = parser.parse_args()
    rows = load_rows(args.raw)
    meta = {}
    if args.meta.exists():
        meta = json.loads(args.meta.read_text(encoding="utf-8"))
    md = build_report(rows, meta)
    args.out.write_text(md, encoding="utf-8")
    print(md)
    print(f"saved {args.out}")


if __name__ == "__main__":
    main()
