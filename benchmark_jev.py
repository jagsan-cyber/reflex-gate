#!/usr/bin/env python3
"""Streamed OpenAI-compatible JEV benchmark (TTFT, decode, accuracy, compliance)."""

from __future__ import annotations

import argparse
import json
import re
import time
from pathlib import Path
from typing import Any

from openai import OpenAI
from pydantic import BaseModel, Field, ValidationError, field_validator

ROOT = Path(__file__).resolve().parent

YES_NO = re.compile(r"^\s*(yes|no)\s*$", re.I)
PREAMBLE = re.compile(
    r"^\s*(sure|of course|here(?:'s| is)|let me|i think|okay|ok,|the answer)",
    re.I,
)


class ExtractSchema(BaseModel):
    status: str
    error_code: str | None
    files_changed: int
    tool: str

    @field_validator("status")
    @classmethod
    def status_ok(cls, v: str) -> str:
        if v not in {"passed", "failed", "timeout", "running"}:
            raise ValueError("bad status")
        return v

    @field_validator("error_code")
    @classmethod
    def error_code_ok(cls, v: str | None) -> str | None:
        if v is None:
            return None
        if v.strip() in {"", "none", "n/a", "null"}:
            raise ValueError("none/empty is not a valid error_code")
        return v


EXTRACT_JSON_SCHEMA: dict[str, Any] = {
    "type": "object",
    "properties": {
        "status": {
            "type": "string",
            "enum": ["passed", "failed", "timeout", "running"],
        },
        "error_code": {
            "type": ["string", "null"],
            "pattern": "^E[0-9]+$",
            "not": {"enum": ["none", ""]},
        },
        "files_changed": {"type": "integer"},
        "tool": {"type": "string"},
    },
    "required": ["status", "error_code", "files_changed", "tool"],
    "additionalProperties": False,
}

TASK_B_RESPONSE_FORMAT: dict[str, Any] = {
    "type": "json_schema",
    "json_schema": {
        "name": "jev_extract",
        "schema": EXTRACT_JSON_SCHEMA,
        "strict": True,
    },
}

TASK_C_TAIL = (
    "Look at the log above. Extract the one line that starts with [ERROR] or [FATAL] "
    "with exact character match. Output that line only. No preface, no commentary."
)


class DecodeProbe(BaseModel):
    n: int = Field(ge=1)
    prompt: str
    max_tokens: int


def first_json_object(text: str) -> str | None:
    start = text.find("{")
    end = text.rfind("}")
    if start < 0 or end <= start:
        return None
    return text[start : end + 1]


def score_task_a(text: str, gold: str) -> tuple[bool, bool]:
    stripped = text.strip().strip("`\"'")
    first = stripped.splitlines()[0].strip() if stripped else ""
    compliant = bool(YES_NO.match(first)) and not PREAMBLE.search(stripped)
    accurate = first.lower() == gold.lower()
    return accurate, compliant


def score_task_b(text: str, gold_json: dict[str, Any]) -> tuple[bool, bool]:
    blob = first_json_object(text)
    if blob is None:
        return False, False
    try:
        data = json.loads(blob)
        parsed = ExtractSchema.model_validate(data)
    except (json.JSONDecodeError, ValidationError, TypeError):
        return False, False
    extra = set(data.keys()) - set(ExtractSchema.model_fields)
    compliant = not extra and not PREAMBLE.search(text.strip())
    accurate = parsed.model_dump() == gold_json
    return accurate, compliant


def score_task_c(text: str, gold: str) -> tuple[bool, bool]:
    stripped = text.strip().strip("`\"'")
    accurate = gold in stripped
    first = stripped.splitlines()[0] if stripped else ""
    compliant = (first == gold) or (stripped == gold)
    if PREAMBLE.search(stripped):
        compliant = False
    return accurate, compliant


def complete_stream(
    client: OpenAI,
    model: str,
    system: str,
    prompt: str,
    max_tokens: int,
    extra_body: dict[str, Any],
    response_format: dict[str, Any] | None = None,
) -> dict[str, Any]:
    t0 = time.perf_counter()
    ttft = None
    t_last = t0
    pieces: list[str] = []
    completion_tokens = 0
    prompt_tokens = 0
    kwargs: dict[str, Any] = {
        "model": model,
        "temperature": 0.0,
        "max_tokens": max_tokens,
        "stream": True,
        "stream_options": {"include_usage": True},
        "extra_body": extra_body,
        "messages": [
            {"role": "system", "content": system},
            {"role": "user", "content": prompt},
        ],
    }
    if response_format is not None:
        kwargs["response_format"] = response_format
        # llama.cpp also reads this from the body; keep a copy for local servers.
        extra = dict(extra_body)
        extra["response_format"] = response_format
        kwargs["extra_body"] = extra
    stream = client.chat.completions.create(**kwargs)
    for chunk in stream:
        if chunk.usage:
            prompt_tokens = chunk.usage.prompt_tokens or prompt_tokens
            completion_tokens = chunk.usage.completion_tokens or completion_tokens
        if not chunk.choices:
            continue
        delta = chunk.choices[0].delta
        content = delta.content or ""
        if content:
            now = time.perf_counter()
            if ttft is None:
                ttft = now - t0
            t_last = now
            pieces.append(content)
    t_end = time.perf_counter()
    text = "".join(pieces)
    if ttft is None:
        ttft = t_end - t0
    decode_s = max(0.0, t_end - t0 - ttft)
    n_out = completion_tokens if completion_tokens else len(text.split())
    tok_s = (n_out / decode_s) if decode_s > 0 and n_out else 0.0
    return {
        "text": text,
        "ttft_s": ttft,
        "total_s": t_end - t0,
        "decode_s": decode_s,
        "prompt_tokens": prompt_tokens,
        "completion_tokens": n_out,
        "tok_s": tok_s,
        "t_last_s": t_last - t0,
    }


def bucket_prompt_tokens(n: int) -> str:
    for edge in (1000, 2000, 4000, 8000):
        if n <= edge * 1.25:
            return f"{edge // 1000}K"
    return "8K+"


def resolve_model(client: OpenAI, requested: str | None) -> str:
    if requested:
        return requested
    models = client.models.list()
    ids = [m.id for m in models.data]
    if not ids:
        raise SystemExit("no models advertised by the server")
    return ids[0]


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--base-url", default="http://127.0.0.1:8080/v1")
    parser.add_argument("--api-key", default="local")
    parser.add_argument("--model", default=None)
    parser.add_argument("--dataset", type=Path, default=ROOT / "data" / "jev_dataset.jsonl")
    parser.add_argument("--out", type=Path, default=ROOT / "results" / "raw.jsonl")
    parser.add_argument("--no-think", action="store_true", default=True)
    parser.add_argument("--think", dest="no_think", action="store_false")
    args = parser.parse_args()

    extra_body: dict[str, Any] = {"cache_prompt": False}
    if args.no_think:
        extra_body["chat_template_kwargs"] = {"enable_thinking": False}

    client = OpenAI(base_url=args.base_url, api_key=args.api_key, timeout=600.0)
    model = resolve_model(client, args.model)
    print(f"endpoint={args.base_url} model={model}")

    cases = []
    with args.dataset.open(encoding="utf-8") as f:
        for line in f:
            if line.strip():
                cases.append(json.loads(line))

    decode_probes = [
        DecodeProbe(
            n=5,
            max_tokens=5,
            prompt="Reply with exactly these five tokens separated by spaces: one two three four five",
        ),
        DecodeProbe(
            n=30,
            max_tokens=30,
            prompt="Count from 1 to 30 using digits only, separated by spaces. Stop at 30.",
        ),
        DecodeProbe(
            n=100,
            max_tokens=150,
            prompt="Count from 1 to 100 using digits only, separated by spaces. Stop at 100.",
        ),
    ]

    args.out.parent.mkdir(parents=True, exist_ok=True)
    rows: list[dict[str, Any]] = []

    def emit(row: dict[str, Any]) -> None:
        rows.append(row)
        print(
            f"{row['id']:10} ttft={row['ttft_s']:.3f}s tot={row['total_s']:.3f}s "
            f"out={row['completion_tokens']} tps={row['tok_s']:.1f} "
            f"acc={row.get('accurate')} cmp={row.get('compliant')}"
        )

    sys_decode = "Follow the user instruction exactly. No extra words."
    for probe in decode_probes:
        m = complete_stream(
            client, model, sys_decode, probe.prompt, probe.max_tokens, extra_body
        )
        emit(
            {
                "id": f"D-{probe.n:03d}",
                "task": "D",
                "gold": None,
                "accurate": None,
                "compliant": None,
                "decode_target": probe.n,
                **m,
            }
        )

    for case in cases:
        try:
            prompt = case["prompt"]
            rf = None
            if case["task"] == "B":
                rf = TASK_B_RESPONSE_FORMAT
            elif case["task"] == "C":
                # Recency: repeat the extract instruction after the long log.
                if TASK_C_TAIL not in prompt[-400:]:
                    prompt = prompt.rstrip() + "\n\n" + TASK_C_TAIL
            m = complete_stream(
                client,
                model,
                case["system"],
                prompt,
                int(case["max_tokens"]),
                extra_body,
                response_format=rf,
            )
        except Exception as exc:
            emit(
                {
                    "id": case["id"],
                    "task": case["task"],
                    "gold": case.get("gold"),
                    "gold_json": case.get("gold_json"),
                    "target_tokens": case.get("target_tokens"),
                    "prompt_bucket": None,
                    "accurate": False,
                    "compliant": False,
                    "text": "",
                    "ttft_s": 0.0,
                    "total_s": 0.0,
                    "decode_s": 0.0,
                    "prompt_tokens": 0,
                    "completion_tokens": 0,
                    "tok_s": 0.0,
                    "error": str(exc),
                }
            )
            continue
        text = m["text"]
        if case["task"] == "A":
            acc, cmp = score_task_a(text, case["gold"])
        elif case["task"] == "B":
            acc, cmp = score_task_b(text, case["gold_json"])
        else:
            acc, cmp = score_task_c(text, case["gold"])
        emit(
            {
                "id": case["id"],
                "task": case["task"],
                "gold": case.get("gold"),
                "gold_json": case.get("gold_json"),
                "target_tokens": case.get("target_tokens"),
                "prompt_bucket": bucket_prompt_tokens(m["prompt_tokens"] or case.get("target_tokens") or 0),
                "accurate": acc,
                "compliant": cmp,
                **m,
            }
        )

    with args.out.open("w", encoding="utf-8") as f:
        for row in rows:
            f.write(json.dumps(row, ensure_ascii=False) + "\n")
    meta = {
        "base_url": args.base_url,
        "model": model,
        "n_rows": len(rows),
        "no_think": args.no_think,
    }
    (args.out.parent / "meta.json").write_text(json.dumps(meta, indent=2), encoding="utf-8")
    print(f"wrote {len(rows)} rows -> {args.out}")


if __name__ == "__main__":
    main()
