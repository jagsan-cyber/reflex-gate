package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"local-jev/internal/schema"
)

func TestSessionManager_SlotBinding(t *testing.T) {
	sm := NewSessionManager(4)

	slot1 := sm.GetSlot("agent-alpha", 0)
	slot2 := sm.GetSlot("agent-beta", 0)
	slot1Again := sm.GetSlot("agent-alpha", 0)

	if slot1 != slot1Again {
		t.Fatalf("expected stable slot for agent-alpha, got %d and %d", slot1, slot1Again)
	}

	if slot1 == slot2 {
		t.Fatalf("expected different slots for different sessions, both got %d", slot1)
	}

	// Empty session should get defaultSlot
	slotDefault := sm.GetSlot("", 2)
	if slotDefault != 2 {
		t.Fatalf("expected default slot 2 for empty session, got %d", slotDefault)
	}
}

func TestSessionManager_IncrementalTracking(t *testing.T) {
	sm := NewSessionManager(2)
	sess := "agent-loop-1"

	if sm.IsIncremental(sess, 100) {
		t.Fatalf("turn 0 should not be incremental")
	}

	sm.RecordTurn(sess, 0, 100)

	// Turn 1 with longer prompt should be incremental
	if !sm.IsIncremental(sess, 150) {
		t.Fatalf("turn 1 with longer prompt should be incremental")
	}

	// Shorter prompt indicates reset
	if sm.IsIncremental(sess, 50) {
		t.Fatalf("shorter prompt should not be incremental")
	}
}

func TestFormatStopPrompt_PrefixInvariance(t *testing.T) {
	logTurn1 := "Step 1: cd repo && git status"
	promptTurn1 := schema.FormatStopPrompt(logTurn1)

	logTurn2 := "Step 1: cd repo && git status\nStep 2: pytest tests/test_core.py -v\n3 passed"
	promptTurn2 := schema.FormatStopPrompt(logTurn2)

	// The prefix up to the end of logTurn1 must match exactly
	idx1 := strings.Index(promptTurn1, logTurn1)
	if idx1 == -1 {
		t.Fatalf("promptTurn1 missing logTurn1")
	}
	prefix1 := promptTurn1[:idx1+len(logTurn1)]

	if !strings.HasPrefix(promptTurn2, prefix1) {
		t.Fatalf("prefix invariance violated between turn 1 and turn 2!\nPrefix 1:\n%s\nPrompt 2:\n%s", prefix1, promptTurn2)
	}

	// Tail anchor should match
	if !strings.HasSuffix(promptTurn1, schema.StopTailAnchor) {
		t.Fatalf("promptTurn1 missing tail anchor")
	}
	if !strings.HasSuffix(promptTurn2, schema.StopTailAnchor) {
		t.Fatalf("promptTurn2 missing tail anchor")
	}
}

func TestStopHandler_MockLlamaIncremental(t *testing.T) {
	// Mock llama-server completion endpoint
	callCount := 0
	mockLlama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)

		tokensCached := 0
		tokensEval := 120
		promptMs := 45.0

		if callCount > 1 {
			// Second call reuses cached KV
			tokensCached = 120
			tokensEval = 25
			promptMs = 5.8
		}

		resp := map[string]any{
			"content":          " No",
			"tokens_cached":    tokensCached,
			"tokens_evaluated": tokensEval,
			"tokens_predicted": 1,
			"timings": map[string]any{
				"prompt_n":     tokensEval,
				"prompt_ms":    promptMs,
				"predicted_n":  1,
				"predicted_ms": 2.5,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer mockLlama.Close()

	srv := NewServer(mockLlama.URL)
	handler := srv.Handler()

	// Turn 1
	body1 := `{"log": "Agent started task", "session_id": "test-session"}`
	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("POST", "/jev/stop", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec1.Code, rec1.Body.String())
	}

	var res1 struct {
		Stop    bool           `json:"stop"`
		Raw     string         `json:"raw"`
		Metrics map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &res1); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if res1.Metrics["cached"] == true {
		t.Fatalf("turn 1 should not be cached")
	}

	// Turn 2 with appended log and same session_id
	body2 := `{"log": "Agent started task\nExecuting command...\nTests passed!", "session_id": "test-session"}`
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("POST", "/jev/stop", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rec2.Code, rec2.Body.String())
	}

	var res2 struct {
		Stop    bool           `json:"stop"`
		Raw     string         `json:"raw"`
		Metrics map[string]any `json:"metrics"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &res2); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if res2.Metrics["cached"] != true {
		t.Fatalf("turn 2 should be marked cached=true, got %v", res2.Metrics["cached"])
	}

	evalCount := res2.Metrics["prompt_eval_count"].(float64)
	if evalCount != 25 {
		t.Fatalf("expected prompt_eval_count=25, got %v", evalCount)
	}
	evalMs := res2.Metrics["prompt_eval_ms"].(float64)
	if evalMs != 5.8 {
		t.Fatalf("expected prompt_eval_ms=5.8, got %v", evalMs)
	}
}
