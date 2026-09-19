"""Fixed JEV output contract. Callers do not send the schema."""

from __future__ import annotations

import re
from typing import Any, Literal

from pydantic import BaseModel, Field, field_validator

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
