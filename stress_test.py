#!/usr/bin/env python3
"""local-jev High-Concurrency Indiscriminate Load & Stress Tester

Generates random payloads across /jev/stop, /jev/extract, /jev/scan,
and /v1/chat/completions to stress-test dual slots, FA, and latency meters.

Usage:
  python stress_test.py                     # Infinite random spam with 4 concurrent workers
  python stress_test.py -c 8 -n 100         # 8 workers, 100 total requests
  python stress_test.py --rps 20            # Rate-controlled stress testing
  python stress_test.py --port 8090         # Target custom JEV port
"""

from __future__ import annotations

import argparse
import json
import random
import sys
import time
import urllib.error
import urllib.request
from concurrent.futures import ThreadPoolExecutor, as_completed
from dataclasses import dataclass
from typing import Any

# ==============================================================================
# Diverse Randomized Sample Payloads
# ==============================================================================
SAMPLE_LOGS = [
    # Clean completion logs (Task A -> stop=true)
    "All unit tests passed. 42 passed in 1.2s. Output verified, shutting down pipeline.",
    "Build succeeded. 0 errors, 0 warnings. Artifacts packaged to /dist/release.zip. Done.",
    "Task completed successfully. Everything looks solid.",
    "Finished execution without errors. Code review passed.",

    # Work in progress / Partial logs (Task A -> stop=false)
    "Step 3/10: Downloading model weights... 45% [=====>      ] ETA 30s",
    "Running integration suite... Test 4 failed: timeout waiting for port 8080. Retrying in 5s...",
    "Editing src/llama-kv-cache.cpp: line 412. Need to verify Walsh-Hadamard rotation dimension.",
    "Compilation failed with 3 errors: undefined reference to ggml_cuda_op_tq4_1s.",

    # Diff / modification logs (Task B -> extract)
    """git diff --stat
 internal/api/server.go   | 34 ++++++++++++++++++++++------------
 internal/proc/proc.go    | 12 +++++++++---
 schema/schema.go         |  8 ++++----
 3 files changed, 35 insertions(+), 19 deletions(-)
 commit 8a9f23c: Fix KV slot allocation race condition in parallel decode.""",

    """Modified files:
- /app/src/main.rs: added SIMD kernel for ARM NEON
- /app/Cargo.toml: bumped version to 0.4.2
Status: Unit tests running on target device.""",

    # Error / stack trace logs (Task C -> scan)
    """Traceback (most recent call last):
  File "engine/runner.py", line 84, in execute
    conn = socket.create_connection(('127.0.0.1', 8080), timeout=2.0)
ConnectionRefusedError: [Errno 111] Connection refused
CRITICAL: Process terminated with exit code 1.""",

    """RuntimeError: CUDA out of memory. Tried to allocate 2.40 GiB (GPU 0; 24.00 GiB total capacity; 22.80 GiB already allocated).
Traceback: llama.cpp/src/llama-kv-cache.cpp:892 ggml_cuda_op_flash_attn_ext""",

    # Long combined text
    """[INFO] 2026-09-19 19:10:01 - Started JEV micro-agent worker slot #0
[DEBUG] Model loaded: Qwen3.5-0.8B-Q8_0 (8192 context, NGL=99, Vulkan compute)
[INFO] Processing request id=9941a8
Warning: High memory pressure detected in page table cache.
All sub-tasks finished. Success rate 100%."""
]

CHAT_PROMPTS = [
    "Say 'READY' in one word.",
    "What is 15 * 18?",
    "Classify as Bug or Feature: 'The latency meter is glowing cyan.'",
    "Is 8192 greater than 4096? Answer Yes or No.",
    "Summarize: Edge AI is fast and private."
]


@dataclass
class Result:
    task: str
    status_code: int
    latency_ms: float
    summary: str
    error: str = ""


# ==============================================================================
# Worker Request Dispatcher
# ==============================================================================
def send_random_request(base_url: str, timeout: float = 30.0) -> Result:
    endpoints = ["stop", "extract", "scan", "chat", "systemone"]
    weights = [35, 30, 20, 10, 5]
    chosen = random.choices(endpoints, weights=weights, k=1)[0]

    t0 = time.perf_counter()
    target_url = ""
    req_body = None
    headers = {"Content-Type": "application/json", "User-Agent": "jev-stress-test"}

    if chosen == "stop":
        target_url = f"{base_url}/jev/stop"
        req_body = json.dumps({"log": random.choice(SAMPLE_LOGS)}).encode("utf-8")
    elif chosen == "extract":
        target_url = f"{base_url}/jev/extract"
        req_body = json.dumps({"log": random.choice(SAMPLE_LOGS)}).encode("utf-8")
    elif chosen == "scan":
        target_url = f"{base_url}/jev/scan"
        req_body = json.dumps({"log": random.choice(SAMPLE_LOGS)}).encode("utf-8")
    elif chosen == "chat":
        target_url = f"{base_url}/v1/chat/completions"
        req_body = json.dumps({
            "model": "qwen3.5-0.8b",
            "messages": [{"role": "user", "content": random.choice(CHAT_PROMPTS)}],
            "max_tokens": 16,
            "temperature": 0.0
        }).encode("utf-8")
    elif chosen == "systemone":
        target_url = f"{base_url}/v1/systemone"
        req_body = json.dumps({
            "type": "choice",
            "question": "Is the task finished?",
            "choices": ["Yes", "No"],
            "context": random.choice(SAMPLE_LOGS)
        }).encode("utf-8")

    req = urllib.request.Request(target_url, data=req_body, headers=headers, method="POST")

    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            data = resp.read()
            lat = (time.perf_counter() - t0) * 1000.0
            summary = ""
            try:
                parsed = json.loads(data)
                if "stop" in parsed:
                    summary = f"stop={parsed['stop']}"
                elif "files" in parsed or "changed_files" in parsed:
                    summary = f"diff={len(parsed.get('files') or parsed.get('changed_files') or [])} files"
                elif "errors" in parsed or "has_error" in parsed:
                    summary = f"errs={len(parsed.get('errors', []))}"
                elif "choices" in parsed:
                    txt = parsed["choices"][0].get("message", {}).get("content", "").strip()
                    summary = f"out: {txt[:25]}"
                else:
                    summary = f"{len(data)}B"
            except Exception:
                summary = f"{len(data)}B"

            return Result(task=chosen, status_code=resp.status, latency_ms=lat, summary=summary)

    except urllib.error.HTTPError as e:
        lat = (time.perf_counter() - t0) * 1000.0
        return Result(task=chosen, status_code=e.code, latency_ms=lat, summary="", error=f"HTTP {e.code}")
    except Exception as e:
        lat = (time.perf_counter() - t0) * 1000.0
        return Result(task=chosen, status_code=0, latency_ms=lat, summary="", error=str(e)[:40])


# ==============================================================================
# Main Runner & CLI
# ==============================================================================
def main() -> None:
    parser = argparse.ArgumentParser(
        description="local-jev Indiscriminate High-Concurrency Load Tester",
        formatter_class=argparse.RawTextHelpFormatter
    )
    parser.add_argument("--host", default="127.0.0.1", help="Target host (default: 127.0.0.1)")
    parser.add_argument("-p", "--port", type=int, default=8090, help="JEV API port (default: 8090)")
    parser.add_argument("-c", "--concurrency", type=int, default=4, help="Number of concurrent client workers (default: 4)")
    parser.add_argument("-n", "--requests", type=int, default=0, help="Total requests to send (0 = run forever until Ctrl+C)")
    parser.add_argument("-d", "--delay", type=float, default=0.0, help="Delay in seconds between requests per worker (default: 0.0)")
    parser.add_argument("--timeout", type=float, default=15.0, help="Request timeout in seconds (default: 15.0)")

    args = parser.parse_args()
    base_url = f"http://{args.host}:{args.port}"

    print("=" * 70)
    print("  \033[1;36mLOCAL-JEV INDISCRIMINATE STRESS & LOAD TESTER\033[0m")
    print("=" * 70)
    print(f"  Target Endpoint : \033[1;32m{base_url}\033[0m")
    print(f"  Concurrency     : \033[1;33m{args.concurrency} parallel workers\033[0m")
    print(f"  Total Requests  : \033[1;33m{'Infinite (Press Ctrl+C to stop)' if args.requests == 0 else args.requests}\033[0m")
    print(f"  Inter-req Delay : {args.delay}s")
    print("-" * 70)

    # Health check
    try:
        health_req = urllib.request.Request(f"{base_url}/health", headers={"User-Agent": "jev-stress-test"})
        with urllib.request.urlopen(health_req, timeout=3.0) as resp:
            h = json.loads(resp.read())
            print(f"  [+] Connected! LLM Engine: \033[1;32m{h.get('model', 'OK')}\033[0m (Slots: {h.get('n_slots', 'auto')})")
    except Exception as e:
        print(f"  \033[1;31m[!] Warning: Cannot reach {base_url}/health: {e}\033[0m")
        print("      Make sure local-jev server is started and running!")
        sys.exit(1)

    print("=" * 70)
    print("  Starting stress fire! Watch the GUI's live waveform & slot LEDs...")
    print("=" * 70)

    latencies: list[float] = []
    task_counts: dict[str, int] = {"stop": 0, "extract": 0, "scan": 0, "chat": 0, "systemone": 0}
    status_counts: dict[int, int] = {}
    completed = 0
    errors = 0
    t_start = time.perf_counter()

    def worker_loop():
        nonlocal completed, errors
        while True:
            if args.requests > 0 and completed >= args.requests:
                break
            res = send_random_request(base_url, timeout=args.timeout)
            completed += 1
            latencies.append(res.latency_ms)
            task_counts[res.task] = task_counts.get(res.task, 0) + 1
            status_counts[res.status_code] = status_counts.get(res.status_code, 0) + 1

            if res.status_code != 200:
                errors += 1
                color = "\033[1;31m"
            else:
                color = "\033[1;32m"

            elapsed = time.perf_counter() - t_start
            rps = completed / max(0.001, elapsed)

            # Compact 1-line telemetry stream
            task_tag = f"[{res.task.upper():<7}]"
            print(
                f"\r{color}#{completed:04d}\033[0m {task_tag} "
                f"lat=\033[1;36m{res.latency_ms:5.1f}ms\033[0m "
                f"rps=\033[1;33m{rps:4.1f}\033[0m "
                f"errs={errors} "
                f"-> {res.summary or res.error:<20}",
                end="",
                flush=True
            )

            if args.delay > 0:
                time.sleep(args.delay)

    try:
        with ThreadPoolExecutor(max_workers=args.concurrency) as executor:
            futures = [executor.submit(worker_loop) for _ in range(args.concurrency)]
            for f in as_completed(futures):
                f.result()
    except KeyboardInterrupt:
        print("\n\n  \033[1;33m[!] Stress test interrupted by user.\033[0m")

    # ==============================================================================
    # Final Benchmark Statistics
    # ==============================================================================
    total_time = time.perf_counter() - t_start
    if not latencies:
        print("\nNo requests completed.")
        return

    latencies.sort()
    n = len(latencies)
    avg_lat = sum(latencies) / n
    p50 = latencies[int(n * 0.50)]
    p95 = latencies[int(n * 0.95)]
    p99 = latencies[int(n * 0.99)] if n >= 100 else latencies[-1]

    print("\n" + "=" * 70)
    print("  \033[1;36mSTRESS TEST FINAL PERFORMANCE SUMMARY\033[0m")
    print("=" * 70)
    print(f"  Total Requests    : {n:,}")
    print(f"  Successful (200)  : \033[1;32m{status_counts.get(200, 0):,}\033[0m")
    print(f"  Failed / Timeouts : \033[1;31m{errors:,}\033[0m")
    print(f"  Elapsed Wall Time : {total_time:.2f} s")
    print(f"  Throughput (RPS)  : \033[1;32m{n / total_time:.2f} requests/sec\033[0m")
    print("-" * 70)
    print("  Latency Distribution:")
    print(f"    Min             : {latencies[0]:.2f} ms")
    print(f"    Avg (Mean)      : {avg_lat:.2f} ms")
    print(f"    P50 (Median)    : {p50:.2f} ms")
    print(f"    P95             : \033[1;33m{p95:.2f} ms\033[0m")
    print(f"    P99             : {p99:.2f} ms")
    print(f"    Max             : {latencies[-1]:.2f} ms")
    print("-" * 70)
    print("  Task Breakdown:")
    for task, count in sorted(task_counts.items(), key=lambda x: x[1], reverse=True):
        pct = (count / n) * 100.0 if n > 0 else 0
        print(f"    {task:<12}: {count:5d} requests ({pct:4.1f}%)")
    print("=" * 70)


if __name__ == "__main__":
    main()
