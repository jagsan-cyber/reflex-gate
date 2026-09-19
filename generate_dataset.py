#!/usr/bin/env python3
"""Generate labeled JEV (Judge / Evaluator / Verifier) synthetic cases."""

from __future__ import annotations

import argparse
import json
import random
import sys
import uuid
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from jev_schema import SYS_A, SYS_B, SYS_C, TASK_B_ONESHOT, TASK_C_HEAD, TASK_C_TAIL

ROOT = Path(__file__).resolve().parent
DATA_DIR = ROOT / "data"

FILLER_LINES = [
    "2026-09-19T{hh:02d}:{mm:02d}:{ss:02d}Z access GET /static/app-{i}.css 200 {ms}ms",
    "2026-09-19T{hh:02d}:{mm:02d}:{ss:02d}Z stats cpu={cpu}% mem={mem}% disk={disk}% load={load}",
    "2026-09-19T{hh:02d}:{mm:02d}:{ss:02d}Z access GET /api/ping?id={i} 200 bytes={ms}",
    "2026-09-19T{hh:02d}:{mm:02d}:{ss:02d}Z metrics qps={qps} inflight={w} rss_mb={rss}",
    "2026-09-19T{hh:02d}:{mm:02d}:{ss:02d}Z access GET /healthz 200 region=r{w}",
]


def est_tokens(text: str) -> int:
    # Log lines tokenize denser than prose (timestamps, ids). Stay under n_ctx.
    return max(1, len(text) // 2)


def filler_line(i: int, rng: random.Random) -> str:
    tmpl = FILLER_LINES[i % len(FILLER_LINES)]
    return tmpl.format(
        hh=(i // 3600) % 24,
        mm=(i // 60) % 60,
        ss=i % 60,
        w=rng.randint(0, 31),
        i=i,
        ms=rng.randint(1, 240),
        cpu=rng.randint(4, 38),
        mem=rng.randint(20, 70),
        disk=rng.randint(10, 55),
        load=rng.choice(["0.12", "0.28", "0.41", "0.07"]),
        qps=rng.randint(8, 90),
        rss=rng.randint(120, 480),
    )


def pad_log(needle: str, target_tokens: int, rng: random.Random) -> str:
    """Unique nonce, filler, marked needle near the end, then a short tail."""
    head = [
        f"# Session: {uuid.uuid4()}",
        TASK_C_HEAD,
        "----- system log start -----",
        "boot ok",
        "config=prod",
    ]
    i = 0
    while est_tokens("\n".join(head)) < max(32, int(target_tokens * 0.85)):
        head.append(filler_line(i, rng))
        i += 1
    block = [
        "##### BEGIN ERROR #####",
        needle,
        "##### END ERROR #####",
        "----- system log end -----",
    ]
    return "\n".join(head + block)


def task_a(rng: random.Random) -> list[dict]:
    yes_logs = [
        "pytest -q\n..... 12 passed in 3.1s\nexit_code=0\nAGENT: all acceptance tests green.",
        "npm test\nTest Suites: 8 passed, 8 total\nTests: 41 passed, 41 total\nDone in 4.2s.",
        "cargo test --offline\ntest result: ok. 27 passed; 0 failed; 0 ignored",
        "make check\nAll 19 checks passed.\nBUILD SUCCESSFUL",
        "git status --porcelain\n(empty)\ncommit abc123 created. CI: success.",
        "docker compose up -d\n Container api  Started\nhealthz: 200 OK\nready=true",
        "ruff check .\nAll checks passed!\nmypy src: Success: no issues found in 14 source files",
        "playwright test\n  9 passed (12.4s)\nartifact=report.html stored.",
        "terraform apply -auto-approve\nApply complete! Resources: 3 added, 0 changed, 0 destroyed.",
        "python -m compileall src\nlisting src ... \nexit_code=0",
        "agent step 4/4: wrote patch, ran tests, opened PR #442. status=done",
        "loop iteration 3: verifier accepted the diff. remaining_todos=0",
        "SQL migration applied. schema_version=17. rowcount=0 errors. complete=yes",
        "lint+typecheck+unit: PASS/PASS/PASS. coverage=91%. gate=ok",
        "final answer emitted. tool_errors=0. user_visible_output=true. stop=yes",
    ]
    no_logs = [
        "pytest -q\n..F..\nFAILED tests/test_cache.py::test_ttl - AssertionError\nexit_code=1",
        "npm test\nFAIL src/format.spec.ts\nExpected 200, received 500\nTests: 1 failed, 12 passed",
        "cargo test\ntest result: FAILED. 24 passed; 2 failed; 0 ignored",
        "Traceback (most recent call last):\n  File \"agent.py\", line 90\nKeyError: 'diff'\nUNRESOLVED",
        "ruff check .\nsrc/jev.py:12:1 E501 line too long\nFound 7 errors.",
        "mypy src\nsrc/bench.py:44: error: Incompatible types\nFound 3 errors in 1 file",
        "docker compose up\nError: port 8080 already allocated\ncontainer exited (1)",
        "git apply patch.diff\nerror: patch failed: src/server.cpp:90\nerror: src/server.cpp: patch does not apply",
        "curl localhost:8080/health\ncurl: (7) Failed to connect. retry leftover.",
        "agent step 2/4: tool web_search timed out after 30s. no fallback used.",
        "loop iteration 1: tests still red. remaining_todos=3. continue required.",
        "SQL migration FAILED. duplicate key value violates unique constraint.",
        "lint PASS, typecheck FAIL, unit PASS. gate=blocked",
        "final answer missing. tool_errors=2. user_visible_output=false.",
        "AssertionError: expected Yes/No, model returned a paragraph. format_invalid",
    ]
    cases = []
    for i, log in enumerate(yes_logs, 1):
        cases.append(
            {
                "id": f"A-{i:03d}",
                "task": "A",
                "system": SYS_A,
                "prompt": f"Log:\n```\n{log}\n```\nShould the agent loop stop?",
                "gold": "Yes",
                "gold_json": None,
                "target_tokens": None,
                "max_tokens": 5,
            }
        )
    for i, log in enumerate(no_logs, 1):
        cases.append(
            {
                "id": f"A-{i+15:03d}",
                "task": "A",
                "system": SYS_A,
                "prompt": f"Log:\n```\n{log}\n```\nShould the agent loop stop?",
                "gold": "No",
                "gold_json": None,
                "target_tokens": None,
                "max_tokens": 5,
            }
        )
    rng.shuffle(cases)
    return cases


def task_b(rng: random.Random) -> list[dict]:
    records = []
    statuses = ["passed", "failed", "timeout", "running"]
    tools = ["pytest", "ruff", "mypy", "compile", "curl"]
    for i in range(30):
        status = statuses[i % len(statuses)]
        err = None if status == "passed" else f"E{100 + i}"
        files = rng.randint(0, 9)
        tool = tools[i % len(tools)]
        duration = rng.randint(12, 880)
        blob = (
            f"Agent turn {i+1}.\n"
            f"I ran `{tool}` and it took {duration}ms.\n"
            f"status={status}\n"
            f"error_code={err if err else 'none'}\n"
            f"Changed files: {files}\n"
            "Notes: please extract JSON only.\n"
            f"noise hash={rng.randbytes(4).hex()}\n"
        )
        gold = {
            "status": status,
            "error_code": err,
            "files_changed": files,
            "tool": tool,
        }
        records.append(
            {
                "id": f"B-{i+1:03d}",
                "task": "B",
                "system": SYS_B,
                "prompt": (
                    TASK_B_ONESHOT
                    + f"\nAgent output:\n```\n{blob}\n```\n"
                    + 'Remember: no error means "error_code": null, never "none".'
                ),
                "gold": None,
                "gold_json": gold,
                "target_tokens": None,
                "max_tokens": 80,
            }
        )
    return records


def task_c(rng: random.Random) -> list[dict]:
    # 1K x3, 2K x3, 4K x2, 8K x2
    plan = [1000, 1000, 1000, 2000, 2000, 2000, 4000, 4000, 8000, 8000]
    cases = []
    for i, ntok in enumerate(plan, 1):
        needle_id = f"JEV-NEEDLE-{i:02d}-{rng.randint(1000, 9999)}"
        needle = f"[ERROR] diagnostic_id={needle_id} action=halt_loop"
        body = pad_log(needle, ntok, rng)
        cases.append(
            {
                "id": f"C-{i:03d}",
                "task": "C",
                "system": SYS_C,
                "prompt": body + "\n\n" + TASK_C_TAIL,
                "gold": needle,
                "gold_json": None,
                "target_tokens": ntok,
                "max_tokens": 150,
            }
        )
    return cases


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", type=Path, default=DATA_DIR / "jev_dataset.jsonl")
    parser.add_argument("--seed", type=int, default=42)
    args = parser.parse_args()
    rng = random.Random(args.seed)
    rows = task_a(rng) + task_b(rng) + task_c(rng)
    args.out.parent.mkdir(parents=True, exist_ok=True)
    with args.out.open("w", encoding="utf-8") as f:
        for row in rows:
            f.write(json.dumps(row, ensure_ascii=False) + "\n")
    counts = {}
    for row in rows:
        counts[row["task"]] = counts.get(row["task"], 0) + 1
    print(f"wrote {len(rows)} cases -> {args.out}")
    print("counts:", counts)


if __name__ == "__main__":
    main()
