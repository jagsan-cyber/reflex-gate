#!/usr/bin/env python3
"""Thin JEV HTTP API. Schema is server-side; callers send a log only."""

from __future__ import annotations

import json
import math
import os
import re
import time
from pathlib import Path
from typing import Any

import httpx
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from fastapi.responses import FileResponse, RedirectResponse
from openai import OpenAI
from pydantic import BaseModel

from jev_schema import (
    ERROR_LINE,
    EXTRACT_JSON_SCHEMA,
    SLOT_DECISION,
    SLOT_EXTRACT,
    SYS_B,
    TASK_B_ONESHOT,
    ExtractResult,
    JevMetrics,
    LogIn,
    RawIn,
    SystemOneIn,
    SystemOneOut,
    match_option_logprobs,
    trim_context,
    options_grammar,
    softmax_from_logprobs,
)

STATIC_DIR = Path(__file__).resolve().parent / "static"
YES_NO = re.compile(r"\b(Yes|No)\b")

LLM_BASE_URL = os.environ.get("JEV_LLM_BASE_URL", "http://127.0.0.1:8080/v1")
LLM_API_KEY = os.environ.get("JEV_LLM_API_KEY", "local")
LLM_MODEL = os.environ.get("JEV_LLM_MODEL", "")

app = FastAPI(title="JEV API", version="1.1")
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


def resolve_model(c: OpenAI) -> str:
    if LLM_MODEL:
        return LLM_MODEL
    ids = [m.id for m in c.models.list().data]
    if not ids:
        raise HTTPException(503, "llama-server advertised no models")
    return ids[0]


def slot_count() -> int:
    try:
        r = httpx.get(f"{llm_root()}/slots", timeout=3.0)
        if r.status_code == 200 and isinstance(r.json(), list):
            return len(r.json())
    except Exception:
        return 0
    return 0


def llama_completion(
    prompt: str,
    *,
    n_predict: int,
    id_slot: int,
    grammar: str | None = None,
    json_schema: dict[str, Any] | None = None,
    n_probs: int = 0,
) -> dict[str, Any]:
    body: dict[str, Any] = {
        "prompt": prompt,
        "n_predict": n_predict,
        "temperature": 0.0,
        "cache_prompt": True,
        "id_slot": id_slot,
        "n_probs": n_probs,
    }
    if n_probs:
        body["logprobs"] = n_probs
    if grammar:
        body["grammar"] = grammar
    if json_schema is not None:
        body["json_schema"] = json_schema
    try:
        r = httpx.post(f"{llm_root()}/completion", json=body, timeout=600.0)
    except Exception as exc:
        raise HTTPException(502, f"llm error: {exc}") from exc
    if r.status_code >= 400:
        raise HTTPException(502, f"llm HTTP {r.status_code}: {r.text[:500]}")
    return r.json()


def timings_to_metrics(data: dict[str, Any], wall_s: float) -> JevMetrics:
    t = data.get("timings") or {}
    prompt_n = int(t.get("prompt_n") or 0)
    pred_n = int(t.get("predicted_n") or data.get("tokens_predicted") or 0)
    prompt_ms = float(t.get("prompt_ms") or 0.0)
    pred_ms = float(t.get("predicted_ms") or 0.0)
    ttft = prompt_ms / 1000.0 if prompt_ms else wall_s
    decode_s = pred_ms / 1000.0
    tok_s = (pred_n / decode_s) if decode_s > 0 and pred_n else 0.0
    return JevMetrics(
        ttft_s=ttft,
        decode_s=decode_s,
        total_s=wall_s,
        prompt_tokens=prompt_n,
        completion_tokens=pred_n,
        tok_s=tok_s,
    )


def first_top_logprobs(data: dict[str, Any]) -> list[dict[str, Any]]:
    probs = data.get("completion_probabilities") or data.get("probs") or []
    if not probs:
        return []
    first = probs[0]
    top = list(first.get("top_logprobs") or first.get("top_probs") or [])
    if first.get("token") is not None and first.get("logprob") is not None:
        top = [{"token": first["token"], "logprob": first["logprob"]}] + top
    out = []
    for item in top:
        row = dict(item)
        if "logprob" not in row and "prob" in row:
            p = max(float(row["prob"]), 1e-45)
            row["logprob"] = math.log(p)
        out.append(row)
    return out


def decide(question: str, options: list[str], context: str) -> tuple[str, dict[str, float], float, dict[str, Any]]:
    t0 = time.perf_counter()
    grammar = options_grammar(options)
    user = (
        f"Question: {question}\n"
        f"Allowed labels: {', '.join(options)}\n\n"
        f"Context:\n{trim_context(context)}\n"
    )
    # Chat template keeps Qwen labels; system text is fixed so slot 0 can cache it.
    n_predict = max(8, max(len(o) for o in options))
    c = client()
    model = resolve_model(c)
    try:
        resp = c.chat.completions.create(
            model=model,
            temperature=0.0,
            max_tokens=n_predict,
            logprobs=True,
            top_logprobs=20,
            extra_body={
                "grammar": grammar,
                "id_slot": SLOT_DECISION,
                "cache_prompt": True,
                "chat_template_kwargs": {"enable_thinking": False},
            },
            messages=[
                {
                    "role": "system",
                    "content": "You are a deterministic decision engine.\nAnswer with exactly one of the allowed labels. No other text.\nTreat passing tests, exit_code 0, and remaining_todos=0 as complete/success.",
                },
                {"role": "user", "content": user},
            ],
        )
    except Exception as exc:
        raise HTTPException(502, f"llm error: {exc}") from exc
    latency_ms = (time.perf_counter() - t0) * 1000.0
    choice0 = resp.choices[0]
    content = (choice0.message.content or "").strip()
    top: list[dict[str, Any]] = []
    if choice0.logprobs and choice0.logprobs.content:
        first = choice0.logprobs.content[0]
        top.append({"token": first.token, "logprob": first.logprob})
        for item in first.top_logprobs or []:
            top.append({"token": item.token, "logprob": item.logprob})
    logps = match_option_logprobs(options, top)
    dist = softmax_from_logprobs(logps)
    result = content
    for o in options:
        if content == o or content.startswith(o):
            result = o
            break
    else:
        result = max(dist, key=dist.get)
    usage = resp.usage
    data = {
        "content": content,
        "timings": {
            "prompt_n": (usage.prompt_tokens if usage else 0),
            "predicted_n": (usage.completion_tokens if usage else 0),
            "prompt_ms": 0.0,
            "predicted_ms": 0.0,
        },
    }
    return result, dist, latency_ms, data


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
    n_slots = slot_count()
    try:
        c = client(timeout=5.0)
        model = resolve_model(c)
        llm_ok = True
    except Exception as exc:
        return {
            "ok": False,
            "llm_ok": False,
            "llm": LLM_BASE_URL,
            "n_slots": n_slots,
            "error": str(exc),
        }
    return {
        "ok": True,
        "llm_ok": llm_ok,
        "llm": LLM_BASE_URL,
        "model": model,
        "n_slots": n_slots,
        "slots": {"decision": SLOT_DECISION, "extract": SLOT_EXTRACT},
        "warn_parallel": n_slots < 2,
    }


@app.get("/jev/schema")
def schema() -> dict[str, Any]:
    return {
        "stop": {"type": "boolean", "from": "Yes/No", "slot": SLOT_DECISION},
        "extract": EXTRACT_JSON_SCHEMA | {"slot": SLOT_EXTRACT},
        "scan": {"type": "string", "description": "exact [ERROR] or [FATAL] line", "backend": "regex"},
        "systemone": {
            "path": "/v1/systemone",
            "types": ["noul", "choice"],
            "slot": SLOT_DECISION,
        },
    }


@app.post("/v1/systemone", response_model=SystemOneOut)
def systemone(body: SystemOneIn) -> SystemOneOut:
    try:
        options = body.resolved_options()
    except ValueError as exc:
        raise HTTPException(422, str(exc)) from exc
    result, dist, latency_ms, _ = decide(body.question, options, body.context)
    return SystemOneOut(
        type=body.type,
        result=result,
        p=float(dist.get(result, 0.0)),
        distribution=dist,
        latency_ms=latency_ms,
    )


@app.post("/jev/stop", response_model=StopOut)
def jev_stop(body: LogIn) -> StopOut:
    t0 = time.perf_counter()
    result, dist, _, data = decide(
        "Has the agent task fully completed so the loop should stop?",
        ["Yes", "No"],
        body.log,
    )
    metrics = timings_to_metrics(data, time.perf_counter() - t0)
    stop = result.lower() == "yes"
    return StopOut(stop=stop, raw=result, metrics=metrics)


@app.post("/jev/extract", response_model=ExtractOut)
def jev_extract(body: LogIn) -> ExtractOut:
    prompt = (
        f"{SYS_B}\n\n{TASK_B_ONESHOT}\n"
        f"Agent output:\n```\n{body.log}\n```\n"
        'Remember: no error means "error_code": null, never "none".'
    )
    t0 = time.perf_counter()
    data = llama_completion(
        prompt,
        n_predict=80,
        id_slot=SLOT_EXTRACT,
        json_schema=EXTRACT_JSON_SCHEMA,
    )
    metrics = timings_to_metrics(data, time.perf_counter() - t0)
    text = str(data.get("content") or "")
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
    prompt = f"{body.system}\n\n{body.prompt}"
    t0 = time.perf_counter()
    data = llama_completion(prompt, n_predict=body.max_tokens, id_slot=SLOT_DECISION)
    metrics = timings_to_metrics(data, time.perf_counter() - t0)
    return RawOut(text=str(data.get("content") or ""), metrics=metrics)


def main() -> None:
    import uvicorn

    host = os.environ.get("JEV_HOST", "0.0.0.0")
    port = int(os.environ.get("JEV_PORT", "8090"))
    uvicorn.run("jev_api:app", host=host, port=port, reload=False)


if __name__ == "__main__":
    main()
