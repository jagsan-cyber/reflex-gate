// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"local-jev/internal/schema"
)

func TestParseCoT(t *testing.T) {
	tests := []struct {
		input       string
		wantReason  string
		wantVerdict string
	}{
		{
			input:       "Reason: Pytest finished with 0 failures and all 12 tests passed.\nVerdict: Yes",
			wantReason:  "Pytest finished with 0 failures and all 12 tests passed.",
			wantVerdict: "Yes",
		},
		{
			input:       "Reason: Compilation failed with 3 syntax errors.\nVerdict: No",
			wantReason:  "Compilation failed with 3 syntax errors.",
			wantVerdict: "No",
		},
		{
			input:       "Reason: All tests passed.\nVerdict: Yes\nExtra lines ignored",
			wantReason:  "All tests passed.",
			wantVerdict: "Yes",
		},
		{
			input:       "Reason: Retried after connection error, reconnected, and finished processing 174 items to ALL_DONE.\nVerdict: Yes",
			wantReason:  "Retried after connection error, reconnected, and finished processing 174 items to ALL_DONE.",
			wantVerdict: "Yes",
		},
		{
			input:       "Yes",
			wantReason:  "Yes",
			wantVerdict: "Yes",
		},
	}

	for _, tc := range tests {
		gotReason, gotVerdict := schema.ParseCoT(tc.input)
		if gotReason != tc.wantReason {
			t.Errorf("ParseCoT(%q) reason = %q, want %q", tc.input, gotReason, tc.wantReason)
		}
		if gotVerdict != tc.wantVerdict {
			t.Errorf("ParseCoT(%q) verdict = %q, want %q", tc.input, gotVerdict, tc.wantVerdict)
		}
	}
}

func TestParseScan(t *testing.T) {
	tests := []struct {
		input        string
		wantSeverity string
		wantFinding  string
	}{
		{
			input:        "Severity: Critical\nFinding: SIGSEGV 11 invalid memory reference in thread 4.",
			wantSeverity: "Critical",
			wantFinding:  "SIGSEGV 11 invalid memory reference in thread 4.",
		},
		{
			input:        "Severity: Warning\nFinding: Deprecated API call will be removed in next release.",
			wantSeverity: "Warning",
			wantFinding:  "Deprecated API call will be removed in next release.",
		},
		{
			input:        "Severity: Safe\nFinding: Normal build output with exit code 0.",
			wantSeverity: "Safe",
			wantFinding:  "Normal build output with exit code 0.",
		},
		{
			input:        "Severity: Unknown\nFinding: Something strange.",
			wantSeverity: "Safe",
			wantFinding:  "Something strange.",
		},
	}

	for _, tc := range tests {
		gotSeverity, gotFinding := schema.ParseScan(tc.input)
		if gotSeverity != tc.wantSeverity {
			t.Errorf("ParseScan(%q) severity = %q, want %q", tc.input, gotSeverity, tc.wantSeverity)
		}
		if gotFinding != tc.wantFinding {
			t.Errorf("ParseScan(%q) finding = %q, want %q", tc.input, gotFinding, tc.wantFinding)
		}
	}
}

func TestChatPrompt(t *testing.T) {
	prompt := schema.ChatPrompt("system instructions", "user log")
	expected := "<|im_start|>system\nsystem instructions<|im_end|>\n<|im_start|>user\nuser log<|im_end|>\n<|im_start|>assistant\n"
	if prompt != expected {
		t.Fatalf("ChatPrompt format mismatch:\ngot:\n%s\nwant:\n%s", prompt, expected)
	}
}

func TestHandlersWithMockLlama(t *testing.T) {
	// Mock llama-server returning completion
	mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		prompt, _ := req["prompt"].(string)
		w.Header().Set("Content-Type", "application/json")

		if bytes.Contains([]byte(prompt), []byte("safety scanner")) {
			// Scan response
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content": "Severity: Critical\nFinding: Secret key exposed in logs.",
				"timings": map[string]any{
					"prompt_n": 100, "predicted_n": 15, "prompt_ms": 20.0, "predicted_ms": 10.0,
				},
			})
			return
		}

		// CoT Stop response
		_ = json.NewEncoder(w).Encode(map[string]any{
			"content": "Reason: All 12 test assertions passed successfully.\nVerdict: Yes",
			"timings": map[string]any{
				"prompt_n": 80, "predicted_n": 18, "prompt_ms": 15.0, "predicted_ms": 12.0,
			},
		})
	}))
	defer mockLlama.Close()

	srv := NewServer(mockLlama.URL)
	handler := srv.Handler()

	// 1. Test /jev/stop CoT response
	{
		reqBody := []byte(`{"log": "Test run finished with 0 errors."}`)
		req := httptest.NewRequest("POST", "/jev/stop", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("/jev/stop HTTP %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}
		if resp["stop"] != true {
			t.Errorf("expected stop=true, got %v", resp["stop"])
		}
		if resp["raw"] != "Yes" {
			t.Errorf("expected raw='Yes', got %v", resp["raw"])
		}
		if resp["reason"] != "All 12 test assertions passed successfully." {
			t.Errorf("unexpected reason: %v", resp["reason"])
		}
	}

	// 2. Test /jev/scan semantic response
	{
		reqBody := []byte(`{"log": "API_SECRET=sk-1234567890abcdef"}`)
		req := httptest.NewRequest("POST", "/jev/scan", bytes.NewReader(reqBody))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("/jev/scan HTTP %d: %s", w.Code, w.Body.String())
		}
		var resp map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode JSON: %v", err)
		}
		if resp["severity"] != "Critical" {
			t.Errorf("expected severity=Critical, got %v", resp["severity"])
		}
		if resp["action"] != "halt_loop" {
			t.Errorf("expected action=halt_loop for Critical, got %v", resp["action"])
		}
		if resp["finding"] != "Secret key exposed in logs." {
			t.Errorf("unexpected finding: %v", resp["finding"])
		}
	}
}

func TestExtractSchemaPattern(t *testing.T) {
	var schemaMap struct {
		Properties struct {
			ErrorCode struct {
				Pattern string `json:"pattern"`
			} `json:"error_code"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schema.ExtractSchema(), &schemaMap); err != nil {
		t.Fatalf("failed to unmarshal ExtractSchema: %v", err)
	}

	pat := schemaMap.Properties.ErrorCode.Pattern
	if pat == "" {
		t.Fatal("expected non-empty error_code pattern")
	}

	validCodes := []string{
		"KEXEC-1024",
		"E101",
		"ERR_TIMEOUT",
		"FAIL-9.1",
		"SIGSEGV_11",
		"ghp_token123",
	}
	invalidCodes := []string{
		"has spaces",
		"error!",
		"code#1",
		"a+b",
	}

	for _, code := range validCodes {
		matched, err := regexp.MatchString(pat, code)
		if err != nil {
			t.Fatalf("invalid pattern %q: %v", pat, err)
		}
		if !matched {
			t.Errorf("expected pattern %q to match %q", pat, code)
		}
	}

	for _, code := range invalidCodes {
		matched, _ := regexp.MatchString(pat, code)
		if matched {
			t.Errorf("expected pattern %q to reject %q", pat, code)
		}
	}
}

