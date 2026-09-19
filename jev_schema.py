"""Fixed JEV output contract. Callers do not send the schema."""

from __future__ import annotations

import math
import re
from typing import Any, Literal

from pydantic import BaseModel, Field, field_validator

CONTEXT_TRIM = 1500
MISSING_LOGPROB = -99.0
SLOT_DECISION = 0
SLOT_EXTRACT = 1

DECISION_PREFIX = (
    "You are a deterministic decision engine.\n"
    "Answer with exactly one of the allowed labels. No other text.\n"
    "Treat passing tests, exit_code 0, and remaining_todos=0 as complete/success.\n"
)


def gbnf_string(s: str) -> str:
    return '"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"'


def options_grammar(options: list[str]) -> str:
    if len(options) < 2:
        raise ValueError("need at least two options")
    return "root ::= " + " | ".join(gbnf_string(o) for o in options)


def softmax_from_logprobs(logps: dict[str, float]) -> dict[str, float]:
    mx = max(logps.values())
    exps = {k: math.exp(v - mx) for k, v in logps.items()}
    z = sum(exps.values()) or 1.0
    return {k: exps[k] / z for k in logps}


def trim_context(text: str, n: int = CONTEXT_TRIM) -> str:
    if len(text) <= n:
        return text
    return text[-n:]


def build_decision_prompt(question: str, options: list[str], context: str) -> str:
    labels = ", ".join(options)
    ctx = trim_context(context)
    return (
        f"{DECISION_PREFIX}\n"
        f"Question: {question}\n"
        f"Allowed labels: {labels}\n\n"
        f"Context:\n{ctx}\n"
    )


def match_option_logprobs(options: list[str], top_logprobs: list[dict[str, Any]]) -> dict[str, float]:
    by_token: dict[str, float] = {}
    for item in top_logprobs:
        tok = str(item.get("token") or "")
        lp = item.get("logprob")
        if lp is None:
            continue
        lp = float(lp)
        for key in {tok, tok.strip()}:
            if not key:
                continue
            prev = by_token.get(key)
            by_token[key] = lp if prev is None else max(prev, lp)
    out: dict[str, float] = {}
    for opt in options:
        found = by_token.get(opt, MISSING_LOGPROB)
        best_len = len(opt) if opt in by_token else 0
        for tok, lp in by_token.items():
            if not tok:
                continue
            if tok == opt or tok.strip() == opt:
                if lp > found:
                    found = lp
                best_len = max(best_len, len(opt))
                continue
            if opt.startswith(tok) and len(tok) >= best_len and len(tok) >= 2:
                best_len = len(tok)
                found = lp
        out[opt] = found
    return out

SYS_A = (
    "You are the JEV loop-exit judge for an agent pipeline. "
    "Read the log. Answer Yes if the task is fully complete and the loop should stop. "
    "Answer No if errors remain unresolved or the agent must continue. "
    "Reply with the single word Yes or No. No punctuation, no explanation."
)

YES_NO_GRAMMAR = 'root ::= "Yes" | "No"'
STOP_TAIL = "Based on the log, if the task is complete output Yes, otherwise No. Answer (Yes/No):"

ERROR_LINE = re.compile(r"(?m)^\[(?:ERROR|FATAL)\][^\n]*")

SYS_B = (
    "You are the JEV format extractor. "
    "Extract the requested fields from the agent output. "
    "Reply with a single JSON object and nothing else. "
    "Use exactly the keys listed. "
    "If there is no error, error_code MUST be JSON null. "
    "Never emit the strings none, n/a, or an empty string for error_code."
)

SYS_C = (
    "You are the JEV log scanner. "
    "Copy the single log line that starts with [ERROR] or [FATAL]. "
    "Ignore access logs, CPU/memory stats, INFO, DEBUG, and any retry/timeout wording. "
    "Reply with that exact line only. No quotes, no prefix, no explanation."
)

TASK_B_ONESHOT = """Required keys: status, error_code, files_changed, tool.
Types: status string, error_code string or null, files_changed integer, tool string.

If there is no error, you MUST write "error_code": null.
Forbidden: "error_code": "none"  and  "error_code": ""

Example (success, no error):
{"status":"passed","error_code":null,"files_changed":2,"tool":"pytest"}

Example (failure):
{"status":"failed","error_code":"E101","files_changed":1,"tool":"mypy"}
"""

TASK_C_HEAD = (
    "Scan the log below. Ignore ordinary access and stats lines. "
    "There is exactly one [ERROR] or [FATAL] line."
)

TASK_C_TAIL = (
    "Important: ignore WARN, retryable, timeout, and HTTP 200 access lines. "
    "Copy the explicit fatal line that starts with [ERROR] or [FATAL], exact match, one line only. "
    "No preface, no commentary."
)

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

RESPONSE_FORMAT_EXTRACT: dict[str, Any] = {
    "type": "json_schema",
    "json_schema": {
        "name": "jev_extract",
        "schema": EXTRACT_JSON_SCHEMA,
        "strict": True,
    },
}


class ExtractResult(BaseModel):
    status: Literal["passed", "failed", "timeout", "running"]
    error_code: str | None
    files_changed: int
    tool: str

    @field_validator("error_code")
    @classmethod
    def error_code_ok(cls, v: str | None) -> str | None:
        if v is None:
            return None
        if v.strip() in {"", "none", "n/a", "null"}:
            raise ValueError("none/empty is not a valid error_code")
        return v


class JevMetrics(BaseModel):
    ttft_s: float
    decode_s: float
    total_s: float
    prompt_tokens: int
    completion_tokens: int
    tok_s: float


class LogIn(BaseModel):
    log: str = Field(min_length=1)


class RawIn(BaseModel):
    prompt: str = Field(min_length=1)
    system: str = "Follow the user instruction exactly. No extra words."
    max_tokens: int = Field(default=30, ge=1, le=512)


class SystemOneIn(BaseModel):
    type: Literal["noul", "choice"]
    question: str = Field(min_length=1)
    options: list[str] | None = None
    context: str = ""

    def resolved_options(self) -> list[str]:
        if self.options:
            return [str(o) for o in self.options]
        if self.type == "noul":
            return ["true", "false"]
        raise ValueError("choice requires options")


class SystemOneOut(BaseModel):
    type: Literal["noul", "choice"]
    result: str
    p: float
    distribution: dict[str, float]
    latency_ms: float
