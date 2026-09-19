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

func TestAuthValidation(t *testing.T) {
	os.Setenv("TYPESAFE_API_KEY", "secret-test-key")
	defer os.Unsetenv("TYPESAFE_API_KEY")

	s := NewServer("http://127.0.0.1:9999")
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	// Missing header -> 401
	resp1, err := http.Get(ts.URL + "/v1/models")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	defer resp1.Body.Close()
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
	defer resp2.Body.Close()
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
	defer resp3.Body.Close()
	if resp3.StatusCode != 200 {
		t.Errorf("expected 200 with correct key, got %d", resp3.StatusCode)
	}
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
