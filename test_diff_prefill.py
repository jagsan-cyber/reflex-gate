import json
import time
import urllib.request

URL = "http://127.0.0.1:8090/jev/stop"
SESSION_ID = "autonomous-agent-loop-test"

turns = [
    # Turn 1: Initial task
    """Step 1: Goal initialized: Refactor database connector.
Running: git status
On branch main, working tree clean.""",

    # Turn 2: Cumulative log with Step 2 appended
    """Step 1: Goal initialized: Refactor database connector.
Running: git status
On branch main, working tree clean.
Step 2: Applied patch to internal/db/conn.go.
Running: go build ./...
Build finished successfully.""",

    # Turn 3: Cumulative log with Step 3 appended
    """Step 1: Goal initialized: Refactor database connector.
Running: git status
On branch main, working tree clean.
Step 2: Applied patch to internal/db/conn.go.
Running: go build ./...
Build finished successfully.
Step 3: Running test suite: go test -v ./...
=== RUN TestConnectionPool
--- PASS: TestConnectionPool (0.05s)""",

    # Turn 4: Final step completed
    """Step 1: Goal initialized: Refactor database connector.
Running: git status
On branch main, working tree clean.
Step 2: Applied patch to internal/db/conn.go.
Running: go build ./...
Build finished successfully.
Step 3: Running test suite: go test -v ./...
=== RUN TestConnectionPool
--- PASS: TestConnectionPool (0.05s)
Step 4: All tests passed with 0 errors. All requirements fulfilled. Exiting."""
]

print("=" * 70)
print(f"DIFFERENTIAL PREFILL TEST (Target: {URL}, Session: {SESSION_ID})")
print("=" * 70)

for i, log_content in enumerate(turns, 1):
    payload = {
        "log": log_content,
        "session_id": SESSION_ID
    }
    data = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(URL, data=data, headers={"Content-Type": "application/json"})

    t0 = time.perf_counter()
    with urllib.request.urlopen(req) as resp:
        res = json.loads(resp.read().decode("utf-8"))
    wall_ms = (time.perf_counter() - t0) * 1000.0

    metrics = res.get("metrics", {})
    stop_val = res.get("stop")
    raw_val = res.get("raw")
    cached = metrics.get("cached")
    prompt_n = metrics.get("prompt_eval_count")
    prompt_ms = metrics.get("prompt_eval_ms")

    print(f"Turn {i}:")
    print(f"  Decision: stop={stop_val} (raw='{raw_val}')")
    print(f"  Latency : {wall_ms:.1f}ms (API reported: {metrics.get('latency_ms')}ms)")
    print(f"  Prefill : {prompt_n} tokens evaluated in {prompt_ms:.1f}ms")
    print(f"  Cached  : {cached}")
    print("-" * 70)
