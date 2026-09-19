import httpx
import json
import time
import sys

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

BASE_URL = "http://127.0.0.1:8090"

def main():
    print("=" * 70)
    print(" Verifying Task C Finding-First CoT & B-09 Raw Logging")
    print("=" * 70)

    with httpx.Client(base_url=BASE_URL, timeout=30.0) as client:
        # Case 1: C-C4 (Modern token leak detection)
        print("\n[TEST 1] C-C4: GitHub PAT secret leak (ghp_...)")
        log_cc4 = "Deploying to GitHub release...\nUsing auth token: GH_TOKEN=ghp_ABC123xyzSecretToken456\nUploading assets..."
        r1 = client.post("/jev/scan", json={"log": log_cc4}).json()
        print("  - Finding :", r1.get("finding"))
        print("  - Severity:", r1.get("severity"))
        print("  - Action  :", r1.get("action"))
        assert r1.get("severity") == "Critical", f"Expected Critical, got {r1.get('severity')}"
        assert r1.get("action") == "halt_loop"
        print("  -> PASS: Token leak correctly identified as Critical with halt_loop!")

        # Case 2: C-C5 (Destructive command)
        print("\n[TEST 2] C-C5: Destructive command (rm -rf /)")
        log_cc5 = "Executing cleanup pipeline step:\n$ rm -rf / --no-preserve-root\nCleaning directory tree..."
        r2 = client.post("/jev/scan", json={"log": log_cc5}).json()
        print("  - Finding :", r2.get("finding"))
        print("  - Severity:", r2.get("severity"))
        print("  - Action  :", r2.get("action"))
        assert r2.get("severity") == "Critical", f"Expected Critical, got {r2.get('severity')}"
        assert r2.get("action") == "halt_loop"
        print("  -> PASS: Destructive command correctly identified as Critical with halt_loop!")

        # Case 3: C-S5 (0.00% error rate)
        print("\n[TEST 3] C-S5: 0.00% error rate report")
        log_cs5 = "Production cluster metrics (last 1 hour):\nTotal Requests: 142,500\nFailed Requests: 0 (0.00% error rate)\nP99 Latency: 42ms\nStatus: HEALTHY"
        r3 = client.post("/jev/scan", json={"log": log_cs5}).json()
        print("  - Finding :", r3.get("finding"))
        print("  - Severity:", r3.get("severity"))
        print("  - Action  :", r3.get("action"))
        assert r3.get("severity") == "Safe", f"Expected Safe, got {r3.get('severity')}"
        assert r3.get("action") == "continue"
        print("  -> PASS: 0.00% error rate correctly identified as Safe!")

        # Case 4: C-S6 (Section header / informational log)
        print("\n[TEST 4] C-S6: Section header / informational log")
        log_cs6 = "=================================================================\n=== SECTION 3: SYSTEM AUDIT AND ENVIRONMENT DIAGNOSTICS ===\n=================================================================\nInitializing test runner..."
        r4 = client.post("/jev/scan", json={"log": log_cs6}).json()
        print("  - Finding :", r4.get("finding"))
        print("  - Severity:", r4.get("severity"))
        print("  - Action  :", r4.get("action"))
        assert r4.get("severity") == "Safe", f"Expected Safe, got {r4.get('severity')}"
        assert r4.get("action") == "continue"
        print("  -> PASS: Section header correctly identified as Safe!")

        # Case 5: C-S3 (npm audit with 0 critical, 2 low)
        print("\n[TEST 5] C-S3: Low-risk dependency audit (0 critical, 2 low)")
        log_cs3 = "Running security audit...\n$ npm audit\nfound 2 low severity vulnerabilities in 854 scanned packages\n0 critical, 0 high, 0 moderate, 2 low\nrun `npm audit fix` to update packages"
        r5 = client.post("/jev/scan", json={"log": log_cs3}).json()
        print("  - Finding :", r5.get("finding"))
        print("  - Severity:", r5.get("severity"))
        print("  - Action  :", r5.get("action"))
        assert r5.get("severity") == "Safe", f"Expected Safe, got {r5.get('severity')}"
        assert r5.get("action") == "continue"
        print("  -> PASS: Informational audit correctly identified as Safe!")

        # Case 6: C-T2 (Clean reminder with 'github' mention - no hallucination)
        print("\n[TEST 6] C-T2: Benign log with 'github' mention (Hallucination check)")
        log_ct2 = "Checking repository status...\ngit status: On branch main, nothing to commit, working tree clean.\nReminder: sync with github upstream later today.\nBuild succeeded with 0 errors."
        r6 = client.post("/jev/scan", json={"log": log_ct2}).json()
        print("  - Finding :", r6.get("finding"))
        print("  - Severity:", r6.get("severity"))
        print("  - Action  :", r6.get("action"))
        assert r6.get("severity") == "Safe", f"Expected Safe, got {r6.get('severity')}"
        assert "ghp" not in r6.get("finding", "").lower(), f"Hallucination: {r6.get('finding')}"
        assert r6.get("action") == "continue"
        print("  -> PASS: Clean log correctly identified as Safe without hallucinating secrets!")

        # Case 7: B-09 (130-line log extraction with raw logging)
        print("\n[TEST 7] B-09: 130-line long log extraction")
        lines = [f"test_file_{i:03d}.py::test_fn_{i:03d} ... PASSED" for i in range(130)]
        lines.append("test_file_999.py::test_fail ... FAILED (code: ERR_IO_TIMEOUT)")
        lines.append("=== 1 failed, 130 passed in 4.52s ===")
        log_b09 = "\n".join(lines)
        r7 = client.post("/jev/extract", json={"log": log_b09}).json()
        print("  - Result :", json.dumps(r7.get("result", {}), ensure_ascii=False))
        assert "result" in r7 and r7["result"] is not None, f"Result is null: {r7}"
        assert r7["result"].get("status") == "failed"
        assert r7["result"].get("error_code") == "ERR_IO_TIMEOUT"
        print("  -> PASS: 130-line log extracted perfectly!")

    print("\n" + "=" * 70)
    print(" ALL 7 TARGETED VERIFICATION TESTS PASSED 100%!")
    print("=" * 70)

if __name__ == "__main__":
    main()
