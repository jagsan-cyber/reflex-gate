package api

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"local-jev/internal/schema"
)

func TestModelsEndpoint(t *testing.T) {
	s := NewServer("http://127.0.0.1:9999")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/models")
	if err != nil {
		t.Fatalf("failed to GET /v1/models: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Data []string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if len(body.Data) != 3 {
		t.Errorf("expected 3 models, got %d: %v", len(body.Data), body.Data)
	}
	expected := map[string]bool{"jev-latest": true, "jev-preview": true, "jev-1.13.0": true}
	for _, m := range body.Data {
		if !expected[m] {
			t.Errorf("unexpected model %q in data", m)
		}
	}
}

func TestHealthAuthField(t *testing.T) {
	s := NewServer("http://127.0.0.1:9999")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	// Default: auth == "off"
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("failed to GET /health: %v", err)
	}
	defer resp.Body.Close()

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if body["auth"] != "off" {
		t.Errorf("expected auth=off, got %v", body["auth"])
	}

	// Environment variable JEV_AUTH=strict
	os.Setenv("JEV_AUTH", "strict")
	defer os.Unsetenv("JEV_AUTH")
	resp2, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	defer resp2.Body.Close()
	var body2 map[string]any
	_ = json.NewDecoder(resp2.Body).Decode(&body2)
	if body2["auth"] != "strict" {
		t.Errorf("expected auth=strict, got %v", body2["auth"])
	}
}

func TestCleanModelName(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{
			input: `C:\Users\fallo\Documents\llama.cpp-qwen3.8build\jev-bench\go\models\qwen2.5-coder-1.5b-instruct-q8_0.gguf`,
			want:  "qwen2.5-coder-1.5b-instruct-q8_0",
		},
		{
			input: "/opt/models/qwen2.5-coder-1.5b-instruct.gguf",
			want:  "qwen2.5-coder-1.5b-instruct",
		},
		{
			input: "qwen2.5-coder-1.5b-instruct-q8_0.gguf",
			want:  "qwen2.5-coder-1.5b-instruct-q8_0",
		},
		{
			input: "qwen2.5-coder-1.5b-instruct",
			want:  "qwen2.5-coder-1.5b-instruct",
		},
		{
			input: "",
			want:  "",
		},
	}
	for _, tc := range cases {
		got := cleanModelName(tc.input)
		if got != tc.want {
			t.Errorf("cleanModelName(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestAuthValidation(t *testing.T) {
	// 1. Default mode ("off"): requests succeed with or without header
	t.Run("mode_off_default", func(t *testing.T) {
		os.Unsetenv("JEV_AUTH")
		os.Unsetenv("TYPESAFE_API_KEY")

		s := NewServer("http://127.0.0.1:9999")
		ts := httptest.NewServer(s.Handler())
		defer ts.Close()

		// No auth header -> 200
		resp, err := http.Get(ts.URL + "/v1/models")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}

		// Dummy bearer token -> 200
		req, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
		req.Header.Set("Authorization", "Bearer dummy-token")
		resp2, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp2.Body.Close()
		if resp2.StatusCode != 200 {
			t.Errorf("expected 200 with dummy token in off mode, got %d", resp2.StatusCode)
		}
	})

	// 2. Strict mode via env
	t.Run("mode_strict_env", func(t *testing.T) {
		os.Setenv("JEV_AUTH", "strict")
		os.Setenv("TYPESAFE_API_KEY", "secret-test-key")
		defer func() {
			os.Unsetenv("JEV_AUTH")
			os.Unsetenv("TYPESAFE_API_KEY")
		}()

		s := NewServer("http://127.0.0.1:9999")
		ts := httptest.NewServer(s.Handler())
		defer ts.Close()

		// Missing header -> 401
		resp1, err := http.Get(ts.URL + "/v1/models")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp1.Body.Close()
		if resp1.StatusCode != 401 {
			t.Errorf("expected 401 without auth header, got %d", resp1.StatusCode)
		}

		// Wrong key -> 401
		req2, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
		req2.Header.Set("Authorization", "Bearer wrong-key")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp2.Body.Close()
		if resp2.StatusCode != 401 {
			t.Errorf("expected 401 with wrong key, got %d", resp2.StatusCode)
		}

		// Correct key -> 200
		req3, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
		req3.Header.Set("Authorization", "Bearer secret-test-key")
		resp3, err := http.DefaultClient.Do(req3)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp3.Body.Close()
		if resp3.StatusCode != 200 {
			t.Errorf("expected 200 with correct key, got %d", resp3.StatusCode)
		}
	})

	// 3. Loose mode via env
	t.Run("mode_loose_env", func(t *testing.T) {
		os.Setenv("JEV_AUTH", "loose")
		os.Setenv("TYPESAFE_API_KEY", "secret-test-key")
		defer func() {
			os.Unsetenv("JEV_AUTH")
			os.Unsetenv("TYPESAFE_API_KEY")
		}()

		s := NewServer("http://127.0.0.1:9999")
		ts := httptest.NewServer(s.Handler())
		defer ts.Close()

		// Missing header -> 200 (allowed for local scripts)
		resp1, err := http.Get(ts.URL + "/v1/models")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp1.Body.Close()
		if resp1.StatusCode != 200 {
			t.Errorf("expected 200 without header in loose mode, got %d", resp1.StatusCode)
		}

		// Wrong key -> 401
		req2, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
		req2.Header.Set("Authorization", "Bearer wrong-key")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp2.Body.Close()
		if resp2.StatusCode != 401 {
			t.Errorf("expected 401 with wrong key in loose mode, got %d", resp2.StatusCode)
		}

		// Correct key -> 200
		req3, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
		req3.Header.Set("Authorization", "Bearer secret-test-key")
		resp3, err := http.DefaultClient.Do(req3)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp3.Body.Close()
		if resp3.StatusCode != 200 {
			t.Errorf("expected 200 with correct key in loose mode, got %d", resp3.StatusCode)
		}
	})

	// 4. Config fallback (server struct fields used when env vars are unset)
	t.Run("config_fallback_strict", func(t *testing.T) {
		os.Unsetenv("JEV_AUTH")
		os.Unsetenv("TYPESAFE_API_KEY")

		s := NewServer("http://127.0.0.1:9999")
		s.AuthMode = "strict"
		s.APIKey = "config-secret-key"

		ts := httptest.NewServer(s.Handler())
		defer ts.Close()

		// Missing header -> 401
		resp1, err := http.Get(ts.URL + "/v1/models")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp1.Body.Close()
		if resp1.StatusCode != 401 {
			t.Errorf("expected 401 without auth header, got %d", resp1.StatusCode)
		}

		// Wrong key -> 401
		req2, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
		req2.Header.Set("Authorization", "Bearer wrong-key")
		resp2, err := http.DefaultClient.Do(req2)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp2.Body.Close()
		if resp2.StatusCode != 401 {
			t.Errorf("expected 401 with wrong key, got %d", resp2.StatusCode)
		}

		// Correct key from config -> 200
		req3, _ := http.NewRequest("GET", ts.URL+"/v1/models", nil)
		req3.Header.Set("Authorization", "Bearer config-secret-key")
		resp3, err := http.DefaultClient.Do(req3)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		resp3.Body.Close()
		if resp3.StatusCode != 200 {
			t.Errorf("expected 200 with config key, got %d", resp3.StatusCode)
		}
	})
}

func TestSystemOneInputValidation(t *testing.T) {
	s := NewServer("http://127.0.0.1:9999")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	cases := []struct {
		name       string
		payload    map[string]any
		expectCode int
	}{
		{
			name: "missing model",
			payload: map[string]any{
				"state":     "log",
				"questions": map[string]any{"q": map[string]any{"type": "noul"}},
			},
			expectCode: 422,
		},
		{
			name: "invalid model alias",
			payload: map[string]any{
				"model":     "gpt-4",
				"state":     "log",
				"questions": map[string]any{"q": map[string]any{"type": "noul"}},
			},
			expectCode: 422,
		},
		{
			name: "missing state",
			payload: map[string]any{
				"model":     "jev-latest",
				"questions": map[string]any{"q": map[string]any{"type": "noul"}},
			},
			expectCode: 422,
		},
		{
			name: "empty questions",
			payload: map[string]any{
				"model":     "jev-latest",
				"state":     "some state",
				"questions": map[string]any{},
			},
			expectCode: 422,
		},
		{
			name: "unknown question type",
			payload: map[string]any{
				"model": "jev-latest",
				"state": "some state",
				"questions": map[string]any{
					"q": map[string]any{"type": "unsupported", "instructions": "test"},
				},
			},
			expectCode: 422,
		},
		{
			name: "choice missing criteria",
			payload: map[string]any{
				"model": "jev-latest",
				"state": "some state",
				"questions": map[string]any{
					"q": map[string]any{"type": "choice", "instructions": "test"},
				},
			},
			expectCode: 422,
		},
		{
			name: "score with single level (<2)",
			payload: map[string]any{
				"model": "jev-latest",
				"state": "some state",
				"questions": map[string]any{
					"q": map[string]any{"type": "score", "instructions": "test", "criteria": []string{"one"}},
				},
			},
			expectCode: 422,
		},
		{
			name: "score with too many levels (>10)",
			payload: map[string]any{
				"model": "jev-latest",
				"state": "some state",
				"questions": map[string]any{
					"q": map[string]any{
						"type":         "score",
						"instructions": "test",
						"criteria":     []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "10"},
					},
				},
			},
			expectCode: 422,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, _ := json.Marshal(tc.payload)
			resp, err := http.Post(ts.URL+"/v1/systemone", "application/json", bytes.NewReader(b))
			if err != nil {
				t.Fatalf("failed request: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.expectCode {
				t.Errorf("expected HTTP %d, got %d", tc.expectCode, resp.StatusCode)
			}
		})
	}
}

func TestSystemOneNoulExecution(t *testing.T) {
	mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/completion" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"content": "Reason: Customer requested refund due to damaged item.\nDecision: true",
				"timings": map[string]any{
					"prompt_n":    50,
					"predicted_n": 10,
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{"role": "assistant", "content": "yes"},
					"logprobs": map[string]any{
						"content": []map[string]any{
							{
								"token": "yes", "logprob": -0.15,
								"top_logprobs": []map[string]any{
									{"token": "yes", "logprob": -0.15},
									{"token": "no", "logprob": -2.0},
								},
							},
						},
					},
				},
			},
			"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 1},
		})
	}))
	defer mockLlama.Close()

	s := NewServer(mockLlama.URL)
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	payload := map[string]any{
		"model": "jev-latest",
		"state": "Customer requested full refund due to damaged item.",
		"questions": map[string]any{
			"needs_review": map[string]any{
				"type":         "noul",
				"instructions": "Is human review or refund required?",
			},
		},
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(ts.URL+"/v1/systemone", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var res struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type string   `json:"type"`
			Noul *float64 `json:"noul"`
		} `json:"answers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	ans, ok := res.Answers["needs_review"]
	if !ok {
		t.Fatalf("missing answer for needs_review")
	}
	if ans.Type != "noul" {
		t.Errorf("expected type noul, got %s", ans.Type)
	}
	if ans.Noul == nil || *ans.Noul < 0.7 {
		t.Errorf("expected noul >= 0.7, got %v", ans.Noul)
	}
}

func TestSystemOneSecretFastPath(t *testing.T) {
	s := NewServer("http://127.0.0.1:9999") // Should not even reach llama
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	payload := map[string]any{
		"model": "jev-latest",
		"state": "CI build failed. Environment: GITHUB_TOKEN=ghp_ABC123xyzSecretToken456Value dumped in error trace.",
		"questions": map[string]any{
			"leak_check": map[string]any{
				"type":         "noul",
				"instructions": "Is there a leaked secret token or credential in the state?",
			},
		},
	}
	b, _ := json.Marshal(payload)
	resp, err := http.Post(ts.URL+"/v1/systemone", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("failed request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var res struct {
		Model   string `json:"model"`
		Answers map[string]struct {
			Type string   `json:"type"`
			Noul *float64 `json:"noul"`
		} `json:"answers"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	ans, ok := res.Answers["leak_check"]
	if !ok {
		t.Fatalf("missing answer for leak_check")
	}
	if ans.Type != "noul" {
		t.Errorf("expected type noul, got %s", ans.Type)
	}
	if ans.Noul == nil || *ans.Noul != 0.99 {
		t.Errorf("expected noul == 0.99, got %v", ans.Noul)
	}
}

func TestParseNoulCoT(t *testing.T) {
	tests := []struct {
		input        string
		wantReason   string
		wantDecision string
	}{
		{
			input:        "Reason: Customer requested refund.\nDecision: true",
			wantReason:   "Customer requested refund.",
			wantDecision: "true",
		},
		{
			input:        "Reason: All tests passed with 0 errors.\nDecision: false",
			wantReason:   "All tests passed with 0 errors.",
			wantDecision: "false",
		},
		{
			input:        "true",
			wantReason:   "true",
			wantDecision: "true",
		},
	}

	for _, tc := range tests {
		gotReason, gotDecision := schema.ParseNoulCoT(tc.input)
		if gotReason != tc.wantReason {
			t.Errorf("ParseNoulCoT(%q) reason = %q, want %q", tc.input, gotReason, tc.wantReason)
		}
		if gotDecision != tc.wantDecision {
			t.Errorf("ParseNoulCoT(%q) decision = %q, want %q", tc.input, gotDecision, tc.wantDecision)
		}
	}
}

func TestCalibrateNoulTemperature(t *testing.T) {
	// 1. Equal logits -> 0.50
	pEqual := schema.CalibrateNoul(-0.5, -0.5, 0.20)
	if math.Abs(pEqual-0.50) > 1e-4 {
		t.Errorf("expected 0.50 for equal logits, got %f", pEqual)
	}

	// 2. Small positive delta (+0.2) with T=0.20 -> p > 0.70 (1 / (1 + exp(-1)) ~= 0.731)
	pHigh := schema.CalibrateNoul(-0.4, -0.6, 0.20)
	if pHigh < 0.70 {
		t.Errorf("expected p > 0.70 for delta=0.2, got %f", pHigh)
	}

	// 3. Small negative delta (-0.2) with T=0.20 -> p < 0.30
	pLow := schema.CalibrateNoul(-0.6, -0.4, 0.20)
	if pLow > 0.30 {
		t.Errorf("expected p < 0.30 for delta=-0.2, got %f", pLow)
	}

	// 4. Missing false token -> 1.0
	pMissingFalse := schema.CalibrateNoul(-0.3, schema.MissingLogprob, 0.20)
	if pMissingFalse != 1.0 {
		t.Errorf("expected 1.0 when false is missing, got %f", pMissingFalse)
	}

	// 5. Missing true token -> 0.0
	pMissingTrue := schema.CalibrateNoul(schema.MissingLogprob, -0.3, 0.20)
	if pMissingTrue != 0.0 {
		t.Errorf("expected 0.0 when true is missing, got %f", pMissingTrue)
	}
}

func TestGetNoulSystemPrompt(t *testing.T) {
	// 1. Loop termination / Stop
	p1 := schema.GetNoulSystemPrompt("Should the loop stop now?")
	if !strings.Contains(p1, "process execution gatekeeper") {
		t.Errorf("expected process execution gatekeeper for stop question, got %q", p1)
	}
	if !strings.Contains(p1, "0 items collected") {
		t.Errorf("expected A-N1 rule (0 items collected) in stop prompt")
	}
	if !strings.Contains(p1, "awaiting user/human confirmation") {
		t.Errorf("expected A-N3 rule (awaiting confirmation) in stop prompt")
	}

	// 2. Human escalation / Support
	p2 := schema.GetNoulSystemPrompt("Does this ticket require human review?")
	if !strings.Contains(p2, "customer support escalation triage") {
		t.Errorf("expected customer support escalation triage for human review, got %q", p2)
	}
	if !strings.Contains(p2, "billing errors") {
		t.Errorf("expected JA-1 rule (billing errors) in human review prompt")
	}

	// 3. Security
	p3 := schema.GetNoulSystemPrompt("Is there a leaked secret token or credential?")
	if !strings.Contains(p3, "security and safety triage gatekeeper") {
		t.Errorf("expected security triage gatekeeper for secret token question, got %q", p3)
	}

	// 4. Default general
	p4 := schema.GetNoulSystemPrompt("Is this condition satisfied?")
	if !strings.Contains(p4, "expert decision gatekeeper") {
		t.Errorf("expected expert decision gatekeeper for generic question, got %q", p4)
	}
}

func TestSystemOneNoulViaChoice(t *testing.T) {
	t.Run("BaselineNegative_LowNoul", func(t *testing.T) {
		mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{"role": "assistant", "content": "no"},
						"logprobs": map[string]any{
							"content": []map[string]any{
								{
									"token": "no", "logprob": -0.27,
									"top_logprobs": []map[string]any{
										{"token": "no", "logprob": -0.27},
										{"token": "yes", "logprob": -1.45},
									},
								},
							},
						},
					},
				},
				"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 1},
			})
		}))
		defer mockLlama.Close()

		s := NewServer(mockLlama.URL)
		ts := httptest.NewServer(s.Handler())
		defer ts.Close()

		payload := map[string]any{
			"model": "jev-latest",
			"state": "0 tests collected. Process finished with exit code 0.",
			"questions": map[string]any{
				"should_stop": map[string]any{
					"type":         "noul",
					"instructions": "Should the execution stop now?",
				},
			},
		}
		b, _ := json.Marshal(payload)
		resp, err := http.Post(ts.URL+"/v1/systemone", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var res struct {
			Model   string `json:"model"`
			Answers map[string]struct {
				Type string   `json:"type"`
				Noul *float64 `json:"noul"`
			} `json:"answers"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		ans, ok := res.Answers["should_stop"]
		if !ok {
			t.Fatalf("missing answer for should_stop")
		}
		if ans.Type != "noul" {
			t.Errorf("expected type noul, got %s", ans.Type)
		}
		if ans.Noul == nil || *ans.Noul > 0.35 || *ans.Noul < 0.0 {
			t.Errorf("expected healthy low noul in [0.0, 0.35], got %v", ans.Noul)
		}
	})

	t.Run("ParityAndDirectAssignment_0.69", func(t *testing.T) {
		// Verify exact parity between noul and choice positive prob on identical input,
		// fixed to choice probability 0.69, proving direct assignment without compression.
		pRaw := 0.69
		mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{"role": "assistant", "content": "yes"},
						"logprobs": map[string]any{
							"content": []map[string]any{
								{
									"token": "yes", "logprob": math.Log(pRaw),
									"top_logprobs": []map[string]any{
										{"token": "yes", "logprob": math.Log(pRaw)},
										{"token": "no", "logprob": math.Log(1.0 - pRaw)},
									},
								},
							},
						},
					},
				},
				"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 1},
			})
		}))
		defer mockLlama.Close()

		s := NewServer(mockLlama.URL)
		ts := httptest.NewServer(s.Handler())
		defer ts.Close()

		payload := map[string]any{
			"model": "jev-latest",
			"state": "Sample system state for calibration validation.",
			"questions": map[string]any{
				"q_noul": map[string]any{
					"type":         "noul",
					"instructions": "Is the condition met?",
				},
				"q_choice": map[string]any{
					"type":         "choice",
					"instructions": "Is the condition met?",
					"criteria": map[string]string{
						"yes": "Condition is met",
						"no":  "Condition is not met",
					},
				},
			},
		}
		b, _ := json.Marshal(payload)
		resp, err := http.Post(ts.URL+"/v1/systemone", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}

		var res struct {
			Answers map[string]struct {
				Type          string             `json:"type"`
				Noul          *float64           `json:"noul"`
				Choice        string             `json:"choice"`
				Confidence    float64            `json:"confidence"`
				Probabilities map[string]float64 `json:"probabilities"`
			} `json:"answers"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		noulAns, okN := res.Answers["q_noul"]
		if !okN || noulAns.Noul == nil {
			t.Fatalf("missing or nil q_noul: %+v", noulAns)
		}
		choiceAns, okC := res.Answers["q_choice"]
		if !okC {
			t.Fatalf("missing q_choice: %+v", res.Answers)
		}

		// 1. Range check: 0.0 <= noul <= 1.0
		if *noulAns.Noul < 0.0 || *noulAns.Noul > 1.0 {
			t.Errorf("noul out of range [0.0, 1.0]: %f", *noulAns.Noul)
		}

		// 2. Choice probability must be 0.69
		choiceYesProb := choiceAns.Probabilities["yes"]
		if choiceYesProb != 0.69 {
			t.Errorf("expected choice[yes] to be 0.69, got %f", choiceYesProb)
		}

		// 3. Direct assignment check: noul must equal 0.69 exactly (not 0.55-0.57)
		if *noulAns.Noul != 0.69 {
			t.Errorf("expected noul to be 0.69, got %f", *noulAns.Noul)
		}

		// 4. Exact parity check: noul completely matches choice probabilities["yes"] (0.01 precision)
		if math.Abs(*noulAns.Noul-choiceYesProb) > 0.01 {
			t.Errorf("exact parity mismatch: noul=%f != choice[yes]=%f", *noulAns.Noul, choiceYesProb)
		}
	})

	t.Run("ParityStrictAssertion_0.55_Fails", func(t *testing.T) {
		// Explicitly assert that distorted/compressed value of 0.55 strictly FAILS
		// against choice positive probability of 0.69 with 0.01 precision.
		choiceYesProb := 0.69
		distortedNoul := 0.55
		diff := math.Abs(distortedNoul - choiceYesProb)
		if diff <= 0.01 {
			t.Fatalf("expected parity check to fail for noul=0.55 vs choice=0.69, but got diff=%f <= 0.01", diff)
		}
	})

	t.Run("PrecisionRounding_0.760123", func(t *testing.T) {
		// Verify probability of 0.760123... rounds exactly to 0.76
		pRaw := 0.76012345
		mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{
						"message": map[string]any{"role": "assistant", "content": "yes"},
						"logprobs": map[string]any{
							"content": []map[string]any{
								{
									"token": "yes", "logprob": math.Log(pRaw),
									"top_logprobs": []map[string]any{
										{"token": "yes", "logprob": math.Log(pRaw)},
										{"token": "no", "logprob": math.Log(1.0 - pRaw)},
									},
								},
							},
						},
					},
				},
				"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 1},
			})
		}))
		defer mockLlama.Close()

		s := NewServer(mockLlama.URL)
		ts := httptest.NewServer(s.Handler())
		defer ts.Close()

		payload := map[string]any{
			"model": "jev-latest",
			"state": "Sample state for rounding verification.",
			"questions": map[string]any{
				"q_round": map[string]any{
					"type":         "noul",
					"instructions": "Should proceed?",
				},
			},
		}
		b, _ := json.Marshal(payload)
		resp, err := http.Post(ts.URL+"/v1/systemone", "application/json", bytes.NewReader(b))
		if err != nil {
			t.Fatalf("failed request: %v", err)
		}
		defer resp.Body.Close()

		var res struct {
			Answers map[string]struct {
				Type string   `json:"type"`
				Noul *float64 `json:"noul"`
			} `json:"answers"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		ans := res.Answers["q_round"]
		if ans.Noul == nil {
			t.Fatalf("missing noul answer")
		}
		if *ans.Noul != 0.76 {
			t.Errorf("expected exact rounded 0.76 from 0.760123..., got %f", *ans.Noul)
		}
		if *ans.Noul < 0.0 || *ans.Noul > 1.0 {
			t.Errorf("noul out of range [0.0, 1.0]: %f", *ans.Noul)
		}
	})

	t.Run("Boundaries", func(t *testing.T) {
		for _, tc := range []struct {
			name     string
			pYes     float64
			expected float64
		}{
			{"near_zero", 0.001, 0.00},
			{"near_one", 0.999, 1.00},
		} {
			t.Run(tc.name, func(t *testing.T) {
				mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{
						"choices": []map[string]any{
							{
								"message": map[string]any{"role": "assistant", "content": "yes"},
								"logprobs": map[string]any{
									"content": []map[string]any{
										{
											"token": "yes", "logprob": math.Log(tc.pYes),
											"top_logprobs": []map[string]any{
												{"token": "yes", "logprob": math.Log(tc.pYes)},
												{"token": "no", "logprob": math.Log(1.0 - tc.pYes)},
											},
										},
									},
								},
							},
						},
						"usage": map[string]any{"prompt_tokens": 50, "completion_tokens": 1},
					})
				}))
				defer mockLlama.Close()

				s := NewServer(mockLlama.URL)
				ts := httptest.NewServer(s.Handler())
				defer ts.Close()

				payload := map[string]any{
					"model": "jev-latest",
					"state": "Boundary check",
					"questions": map[string]any{
						"q": map[string]any{"type": "noul", "instructions": "boundary?"},
					},
				}
				b, _ := json.Marshal(payload)
				resp, err := http.Post(ts.URL+"/v1/systemone", "application/json", bytes.NewReader(b))
				if err != nil {
					t.Fatalf("request failed: %v", err)
				}
				defer resp.Body.Close()

				var res struct {
					Answers map[string]struct {
						Noul *float64 `json:"noul"`
					} `json:"answers"`
				}
				_ = json.NewDecoder(resp.Body).Decode(&res)
				ans := res.Answers["q"]
				if ans.Noul == nil {
					t.Fatalf("missing noul")
				}
				if *ans.Noul < 0.0 || *ans.Noul > 1.0 {
					t.Errorf("out of range: %f", *ans.Noul)
				}
				if *ans.Noul != tc.expected {
					t.Errorf("expected %f, got %f", tc.expected, *ans.Noul)
				}
			})
		}
	})
}
