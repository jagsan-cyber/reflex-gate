package api

import (
	"encoding/json"
	"testing"
)

func TestResolveLegacyContext(t *testing.T) {
	t.Run("prefers_context_when_both_set", func(t *testing.T) {
		state, _ := json.Marshal("from-state")
		got := resolveLegacyContext("from-context", state)
		if got != "from-context" {
			t.Fatalf("got %q, want from-context", got)
		}
	})

	t.Run("uses_state_string_when_context_empty", func(t *testing.T) {
		state, _ := json.Marshal("KEY=sk-proj-abc")
		got := resolveLegacyContext("", state)
		if got != "KEY=sk-proj-abc" {
			t.Fatalf("got %q, want KEY=sk-proj-abc", got)
		}
	})

	t.Run("uses_state_object_when_context_empty", func(t *testing.T) {
		state, _ := json.Marshal(map[string]string{"log": "secret"})
		got := resolveLegacyContext("  ", state)
		if got == "" || got == "null" {
			t.Fatalf("expected JSON state, got %q", got)
		}
		var m map[string]string
		if err := json.Unmarshal([]byte(got), &m); err != nil {
			t.Fatalf("state should round-trip as JSON object: %v (%q)", err, got)
		}
		if m["log"] != "secret" {
			t.Fatalf("got %#v", m)
		}
	})

	t.Run("empty_when_both_missing", func(t *testing.T) {
		got := resolveLegacyContext("", nil)
		if got != "" {
			t.Fatalf("got %q, want empty", got)
		}
		got = resolveLegacyContext("", json.RawMessage("null"))
		if got != "" {
			t.Fatalf("null state should be empty, got %q", got)
		}
	})
}
