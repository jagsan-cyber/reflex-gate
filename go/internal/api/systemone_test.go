package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{"role": "assistant", "content": "true"},
					"logprobs": map[string]any{
						"content": []map[string]any{
							{
								"token": "true", "logprob": -0.15,
								"top_logprobs": []map[string]any{
									{"token": "true", "logprob": -0.15},
									{"token": "false", "logprob": -2.0},
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
