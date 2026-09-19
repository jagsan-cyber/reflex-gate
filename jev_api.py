#!/usr/bin/env python3
"""Thin JEV HTTP API. Schema is server-side; callers send a log only."""

from __future__ import annotations

import json
import os
import re
import time
import uuid
from pathlib import Path
from typing import Any

from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, RedirectResponse
from openai import OpenAI
from pydantic import BaseModel

STATIC_DIR = Path(__file__).resolve().parent / "static"

import httpx

from jev_schema import (
    ERROR_LINE,
    EXTRACT_JSON_SCHEMA,
    RESPONSE_FORMAT_EXTRACT,
    STOP_TAIL,
    SYS_A,
    SYS_B,
    YES_NO_GRAMMAR,
    TASK_B_ONESHOT,
    ExtractResult,
    JevMetrics,
    LogIn,
    RawIn,
)

YES_NO = re.compile(r"\b(Yes|No)\b")

LLM_BASE_URL = os.environ.get("JEV_LLM_BASE_URL", "http://127.0.0.1:8080/v1")
LLM_API_KEY = os.environ.get("JEV_LLM_API_KEY", "local")
LLM_MODEL = os.environ.get("JEV_LLM_MODEL", "")
_last_mode: str | None = None

app = FastAPI(title="JEV API", version="1.0")
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"],
)


def client(timeout: float = 600.0) -> OpenAI:
    return OpenAI(base_url=LLM_BASE_URL, api_key=LLM_API_KEY, timeout=timeout)


def llm_root() -> str:
    return LLM_BASE_URL.rstrip("/").removesuffix("/v1")


def erase_slots() -> None:
    root = llm_root()
    try:
        listing = httpx.get(f"{root}/slots", timeout=3.0)
        ids = [0]
        if listing.status_code == 200 and isinstance(listing.json(), list):
            ids = [int(s.get("id", i)) for i, s in enumerate(listing.json())] or [0]
        for i in ids:
            httpx.post(f"{root}/slots/{i}", params={"action": "erase"}, timeout=3.0)
    except Exception:
        return


def maybe_erase(next_mode: str) -> None:
    global _last_mode
    if _last_mode is not None and _last_mode != next_mode:
        erase_slots()
    _last_mode = next_mode


def resolve_model(c: OpenAI) -> str:
    if LLM_MODEL:
        return LLM_MODEL
    ids = [m.id for m in c.models.list().data]
    if not ids:
        raise HTTPException(503, "llama-server advertised no models")
    return ids[0]


def complete(
    system: str,
    prompt: str,
    max_tokens: int,
    response_format: dict[str, Any] | None = None,
    grammar: str | None = None,
) -> tuple[str, JevMetrics]:
    c = client()
    model = resolve_model(c)
    prompt = f"# Session: {uuid.uuid4()}\n" + prompt
    extra_body: dict[str, Any] = {
        "cache_prompt": False,
        "chat_template_kwargs": {"enable_thinking": False},
    }
    if grammar:
        extra_body["grammar"] = grammar
    elif response_format is None:
        extra_body["grammar"] = ""
        extra_body["response_format"] = {"type": "text"}
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
        extra = dict(extra_body)
        extra["response_format"] = response_format
        extra.pop("grammar", None)
        extra.pop("json_schema", None)
        kwargs["extra_body"] = extra

    t0 = time.perf_counter()
    ttft = None
    pieces: list[str] = []
    prompt_tokens = 0
    completion_tokens = 0
    try:
        stream = c.chat.completions.create(**kwargs)
        for chunk in stream:
            if chunk.usage:
                prompt_tokens = chunk.usage.prompt_tokens or prompt_tokens
                completion_tokens = chunk.usage.completion_tokens or completion_tokens
            if not chunk.choices:
                continue
            content = chunk.choices[0].delta.content or ""
            if content:
                now = time.perf_counter()
                if ttft is None:
                    ttft = now - t0
                pieces.append(content)
    except Exception as exc:
        raise HTTPException(502, f"llm error: {exc}") from exc

    t_end = time.perf_counter()
    text = "".join(pieces)
    if ttft is None:
        ttft = t_end - t0
    decode_s = max(0.0, t_end - t0 - ttft)
    n_out = completion_tokens if completion_tokens else len(text.split())
    tok_s = (n_out / decode_s) if decode_s > 0 and n_out else 0.0
    metrics = JevMetrics(
        ttft_s=ttft,
        decode_s=decode_s,
        total_s=t_end - t0,
        prompt_tokens=prompt_tokens,
        completion_tokens=n_out,
        tok_s=tok_s,
    )
    return text, metrics


class StopOut(BaseModel):
    stop: bool
    raw: str
    metrics: JevMetrics


class ExtractOut(BaseModel):
    result: ExtractResult
    raw: str
    metrics: JevMetrics


class ScanOut(BaseModel):
    line: str
    metrics: JevMetrics


class RawOut(BaseModel):
    text: str
    metrics: JevMetrics


@app.get("/")
def root() -> RedirectResponse:
    return RedirectResponse("/demo")


@app.get("/demo")
def demo() -> FileResponse:
    page = STATIC_DIR / "demo.html"
    if not page.is_file():
        raise HTTPException(404, "demo.html missing")
    return FileResponse(page)


@app.get("/health")
def health() -> dict[str, Any]:
    llm_ok = False
    model = LLM_MODEL or None
    try:
        c = client(timeout=5.0)
        model = resolve_model(c)
        llm_ok = True
    except Exception as exc:
        return {"ok": False, "llm_ok": False, "llm": LLM_BASE_URL, "error": str(exc)}
    return {"ok": True, "llm_ok": llm_ok, "llm": LLM_BASE_URL, "model": model}


@app.get("/jev/schema")
def schema() -> dict[str, Any]:
    return {
        "stop": {"type": "boolean", "from": "Yes/No"},
        "extract": EXTRACT_JSON_SCHEMA,
        "scan": {"type": "string", "description": "exact [ERROR] or [FATAL] line"},
    }


@app.post("/jev/stop", response_model=StopOut)
def jev_stop(body: LogIn) -> StopOut:
    maybe_erase("stop")
    prompt = f"Log:\n```\n{body.log}\n```\n{STOP_TAIL}"
    text, metrics = complete(SYS_A, prompt, 4, grammar=YES_NO_GRAMMAR)
    m = YES_NO.search(text)
    if not m:
        raise HTTPException(422, f"JEV did not return Yes/No: {text!r}")
    return StopOut(stop=m.group(1).lower() == "yes", raw=m.group(1), metrics=metrics)


@app.post("/jev/extract", response_model=ExtractOut)
def jev_extract(body: LogIn) -> ExtractOut:
    maybe_erase("extract")
    prompt = (
        TASK_B_ONESHOT
        + f"\nAgent output:\n```\n{body.log}\n```\n"
        + 'Remember: no error means "error_code": null, never "none".'
    )
    text, metrics = complete(SYS_B, prompt, 80, RESPONSE_FORMAT_EXTRACT)
    start, end = text.find("{"), text.rfind("}")
    if start < 0 or end <= start:
        raise HTTPException(422, f"JEV did not return JSON: {text!r}")
    try:
        parsed = ExtractResult.model_validate(json.loads(text[start : end + 1]))
    except Exception as exc:
        raise HTTPException(422, f"JEV JSON failed schema: {exc}; raw={text!r}") from exc
    return ExtractOut(result=parsed, raw=text[start : end + 1], metrics=metrics)


@app.post("/jev/scan", response_model=ScanOut)
def jev_scan(body: LogIn) -> ScanOut:
    maybe_erase("scan")
    t0 = time.perf_counter()
    found = ERROR_LINE.findall(body.log)
    line = found[-1].strip() if found else ""
    dt = time.perf_counter() - t0
    return ScanOut(
        line=line,
        metrics=JevMetrics(
            ttft_s=dt,
            decode_s=0.0,
            total_s=dt,
            prompt_tokens=0,
            completion_tokens=0,
            tok_s=0.0,
        ),
    )


@app.post("/jev/raw", response_model=RawOut)
def jev_raw(body: RawIn) -> RawOut:
    maybe_erase("raw")
    text, metrics = complete(body.system, body.prompt, body.max_tokens)
    return RawOut(text=text, metrics=metrics)


def main() -> None:
    import uvicorn

    host = os.environ.get("JEV_HOST", "0.0.0.0")
    port = int(os.environ.get("JEV_PORT", "8090"))
    uvicorn.run("jev_api:app", host=host, port=port, reload=False)


if __name__ == "__main__":
    main()
