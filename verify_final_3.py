import httpx
import json
import time
import sys

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(encoding="utf-8")

BASE_URL = "http://127.0.0.1:8090"

def main():
    print("=" * 70)
    print(" Verifying 3 Remaining Edge Cases (C-T2, C-S3, B-09) + Regression")
    print("=" * 70)

    with httpx.Client(base_url=BASE_URL, timeout=30.0) as client:
        # Case 1: C-T2 - Prompt Contamination & Hallucination Check
        print("\n[TEST 1] C-T2: Benign log with 'github' mention (No hallucinated tokens)")
        log_c_t2 = (
            "Checking repository status...\n"
            "git status: On branch main, nothing to commit, working tree clean.\n"
            "Reminder: sync with github upstream later today.\n"
            "Build succeeded with 0 errors."
        )
        r1 = client.post("/jev/scan", json={"log": log_c_t2}).json()
        print("  - Severity:", r1.get("severity"))
        print("  - Finding :", r1.get("finding"))
        assert r1.get("severity") in ["Safe", "Warning"], f"Expected Safe or Warning, got {r1.get('severity')}"
        assert r1.get("severity") != "Critical", f"Expected non-Critical, got {r1.get('severity')}"
        assert "ghp" not in r1.get("finding", "").lower(), f"Hallucination detected: {r1.get('finding')}"
        print("  -> PASS: Correctly classified without hallucinating credentials or false Critical!")

        # Case 2: C-S3 - Low-risk Audit (0 critical, 2 low) -> Safe
        print("\n[TEST 2] C-S3: Informational audit with 0 critical issues (npm audit)")
        log_c_s3 = (
            "Running security audit...\n"
            "$ npm audit\n"
            "found 2 low severity vulnerabilities in 854 scanned packages\n"
            "0 critical, 0 high, 0 moderate, 2 low\n"
            "run `npm audit fix` to update packages"
        )
        r2 = client.post("/jev/scan", json={"log": log_c_s3}).json()
        print("  - Severity:", r2.get("severity"))
        print("  - Finding :", r2.get("finding"))
        print("  - Action  :", r2.get("action"))
        assert r2.get("severity") == "Safe", f"Expected Safe, got {r2.get('severity')}"
        print("  -> PASS: 0 critical audit correctly classified as Safe!")

        # Case 3: Real secret leak must still be Critical (Regression check for C-T2 fix)
        print("\n[TEST 3] Real secret leak (ghp_ token) regression check")
        log_real_secret = "git push origin main --token=ghp_Secr3tTokenValue999"
        r3 = client.post("/jev/scan", json={"log": log_real_secret}).json()
        print("  - Severity:", r3.get("severity"))
        print("  - Finding :", r3.get("finding"))
        print("  - Action  :", r3.get("action"))
        assert r3.get("severity") == "Critical", f"Expected Critical, got {r3.get('severity')}"
        assert r3.get("action") == "halt_loop"
        print("  -> PASS: Real token leak correctly flagged as Critical!")

        # Case 4: B-09 - Long log (120+ lines)
        print("\n[TEST 4] B-09: 130-line log extraction with error_code")
        lines = [f"test_file_{i:03d}.py::test_fn_{i:03d} ... PASSED" for i in range(130)]
        lines.append("test_file_999.py::test_fail ... FAILED (code: ERR_IO_TIMEOUT)")
        lines.append("=== 1 failed, 130 passed in 4.52s ===")
        log_b_09 = "\n".join(lines)
        r4 = client.post("/jev/extract", json={"log": log_b_09}).json()
        print("  - Response:", json.dumps(r4.get("result", {}), ensure_ascii=False))
        assert "result" in r4 and r4["result"] is not None, f"Result is null: {r4}"
        assert r4["result"].get("status") == "failed", f"Expected 'failed', got {r4['result'].get('status')}"
        assert r4["result"].get("error_code") == "ERR_IO_TIMEOUT", f"Expected ERR_IO_TIMEOUT, got {r4['result'].get('error_code')}"
        print("  -> PASS: 130-line log extracted successfully without null!")

    print("\n" + "=" * 70)
    print(" ALL EDGE CASE TESTS PASSED 100%!")
    print("=" * 70)

if __name__ == "__main__":
    main()
