package selftest

import (
	"testing"
)

func TestGenerateCases(t *testing.T) {
	quickCases := generateCases("quick")
	if len(quickCases) < 10 {
		t.Fatalf("expected at least 10 quick cases, got %d", len(quickCases))
	}

	thoroughCases := generateCases("thorough")
	if len(thoroughCases) < 80 {
		t.Fatalf("expected at least 80 thorough cases, got %d", len(thoroughCases))
	}

	// Verify evaluation functions with mock positive responses
	for _, tc := range quickCases {
		if tc.Task == "" || tc.Generator == "" || tc.Log == "" {
			t.Errorf("incomplete test case: %+v", tc)
		}
	}
}
