import httpx
import json
import time
import sys

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

BASE_URL = "http://127.0.0.1:8090"

def test_edge_cases():
    print("=" * 70)
    print(" ReflexGate 精度強化エッジケース単体検証 (Targeted Edge Case Verification)")
    print("=" * 70)
    
    with httpx.Client(base_url=BASE_URL, timeout=30.0) as client:
        # 1. B-05: KEXEC-1024 prefix preservation
        print("[TEST 1] B-05: Non-E error code prefix (KEXEC-1024)")
        log_b05 = "Agent turn 5.\nExecution failed with kernel code KEXEC-1024.\nChanged files: 1\nTool: bash"
        r = client.post("/jev/extract", json={"log": log_b05}).json()
        print("  - Response:", json.dumps(r.get("result", {}), ensure_ascii=False))
        code = r.get("result", {}).get("error_code")
        assert code == "KEXEC-1024", f"Expected 'KEXEC-1024', got '{code}'"
        print("  -> PASS: error_code was strictly preserved as 'KEXEC-1024'!\n")

        # 2. B-04: Status priority (24 passed, 3 failed -> status=failed)
        print("[TEST 2] B-04: Status Priority (24 passed, 3 failed)")
        log_b04 = "Test summary: 24 passed, 3 failed in 4.12s.\nModified files: 4\nTool: pytest\nError code: E401"
        r = client.post("/jev/extract", json={"log": log_b04}).json()
        print("  - Response:", json.dumps(r.get("result", {}), ensure_ascii=False))
        st = r.get("result", {}).get("status")
        assert st == "failed", f"Expected 'failed', got '{st}'"
        print("  -> PASS: status priority prioritized 'failed' over passed!\n")

        # 3. B-09: 120+ lines long log
        print("[TEST 3] B-09: 120+ lines log parsing without truncation")
        lines = [f"test_mod_{i:03d}.py::test_case_{i:03d} PASSED" for i in range(125)]
        lines.append("test_mod_core.py::test_eval FAILED (error_code: E999)")
        lines.append("=== 1 failed, 125 passed in 3.14s ===")
        log_b09 = "\n".join(lines)
        r = client.post("/jev/extract", json={"log": log_b09}).json()
        print("  - Response:", json.dumps(r.get("result", {}), ensure_ascii=False))
        assert "result" in r and r["result"] is not None, "Expected valid result object"
        assert r["result"].get("error_code") == "E999", f"Expected 'E999', got {r['result'].get('error_code')}"
        print("  -> PASS: 127-line log extracted successfully without null/truncation!\n")

        # 4. C-C4: GitHub PAT secret leak (ghp_...)
        print("[TEST 4] C-C4: Modern token leak detection (ghp_...)")
        log_cc4 = "Deploying to GitHub release...\nUsing auth token: GH_TOKEN=ghp_ABC123xyzSecretToken456\nUploading assets..."
        r = client.post("/jev/scan", json={"log": log_cc4}).json()
        print("  - Severity:", r.get("severity"))
        print("  - Finding :", r.get("finding"))
        print("  - Action  :", r.get("action"))
        assert r.get("severity") == "Critical", f"Expected 'Critical', got '{r.get('severity')}'"
        assert r.get("action") == "halt_loop", f"Expected 'halt_loop', got '{r.get('action')}'"
        print("  -> PASS: ghp_ token leak detected as Critical with halt_loop!\n")

    print("=" * 70)
    print(" ALL 4 TARGETED EDGE CASES PASSED PERFECTLY!")
    print("=" * 70)

if __name__ == "__main__":
    test_edge_cases()
