#!/usr/bin/env python3
"""Smoke POST /v1/systemone (noul + choice)."""

from __future__ import annotations

import json
import sys

import httpx

BASE = sys.argv[1] if len(sys.argv) > 1 else "http://127.0.0.1:8090"


def check_dist(dist: dict, options: list[str]) -> None:
    s = sum(dist.values())
    if abs(s - 1.0) > 1e-4:
        raise SystemExit(f"distribution does not sum to 1: {s} {dist}")
    for o in options:
        if o not in dist:
            raise SystemExit(f"missing option {o} in {dist}")
        if not (0.0 <= dist[o] <= 1.0):
            raise SystemExit(f"bad p for {o}: {dist[o]}")


def main() -> None:
    health = httpx.get(f"{BASE}/health", timeout=8.0)
    health.raise_for_status()
    info = health.json()
    print("health", json.dumps(info))
    if not info.get("llm_ok"):
        raise SystemExit("LLM is down")
    if info.get("n_slots", 0) < 2:
        print("WARN: n_slots < 2; start llama-server with --parallel 2")

    noul_body = {
        "type": "noul",
        "question": "Has the agent task fully completed so the loop should stop?",
        "options": ["Yes", "No"],
        "context": "pytest -q\n..... 12 passed in 3.1s\nexit_code=0\nAGENT: all acceptance tests green.",
    }
    r = httpx.post(f"{BASE}/v1/systemone", json=noul_body, timeout=120.0)
    print("noul HTTP", r.status_code, r.text[:800])
    r.raise_for_status()
    noul = r.json()
    check_dist(noul["distribution"], ["Yes", "No"])
    if noul["result"] not in ("Yes", "No"):
        raise SystemExit(f"bad noul result {noul['result']}")
    if noul["result"] != "Yes":
        print("WARN: expected Yes on a passing log, got", noul["result"], "p", noul["p"])

    choice_body = {
        "type": "choice",
        "question": "Which tool should run next?",
        "options": ["ruff_format", "mypy", "pytest"],
        "context": "Agent: lint failed on long lines in src/jev.py. Tests were not run yet.",
    }
    r = httpx.post(f"{BASE}/v1/systemone", json=choice_body, timeout=120.0)
    print("choice HTTP", r.status_code, r.text[:800])
    r.raise_for_status()
    choice = r.json()
    check_dist(choice["distribution"], ["ruff_format", "mypy", "pytest"])
    if choice["result"] not in choice_body["options"]:
        raise SystemExit(f"bad choice result {choice['result']}")
    print("OK noul", noul["result"], round(noul["p"], 4), "ms", round(noul["latency_ms"], 1))
    print("OK choice", choice["result"], round(choice["p"], 4), "ms", round(choice["latency_ms"], 1))


if __name__ == "__main__":
    main()
