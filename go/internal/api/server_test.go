package api

import (
	"strings"
	"testing"

	"local-jev/internal/schema"
)

func TestBuildExtractUserPrompt(t *testing.T) {
	// Case 1: Raw log (e.g. from demo.html or direct caller)
	rawLog := "Agent turn 1.\nI ran pytest and it took 296ms.\nstatus=passed\nerror_code=none\nChanged files: 9"
	prompt1 := buildExtractUserPrompt(rawLog)

	if !strings.Contains(prompt1, schema.TaskBOneShot) {
		t.Errorf("expected prompt to contain TaskBOneShot")
	}
	if !strings.Contains(prompt1, "Agent output:\n```\n"+rawLog+"\n```") {
		t.Errorf("expected prompt to contain wrapped raw log")
	}
	if !strings.HasSuffix(prompt1, `Remember: no error means "error_code": null, never "none".`) {
		t.Errorf("expected prompt to end with anchor without extra newline")
	}

	// Case 2: Pre-formatted prompt (e.g. from run_bench.py / jev_dataset.jsonl)
	datasetPrompt := schema.TaskBOneShot + "\nAgent output:\n```\n" + rawLog + "\n```\nRemember: no error means \"error_code\": null, never \"none\"."
	prompt2 := buildExtractUserPrompt(datasetPrompt)

	// Ensure no double wrapping occurred
	if strings.Count(prompt2, "Required keys:") != 1 {
		t.Errorf("expected exactly 1 occurrence of 'Required keys:', got %d", strings.Count(prompt2, "Required keys:"))
	}
	if strings.Count(prompt2, "Agent output:") != 1 {
		t.Errorf("expected exactly 1 occurrence of 'Agent output:', got %d", strings.Count(prompt2, "Agent output:"))
	}
	if prompt1 != prompt2 {
		t.Errorf("prompt1 and prompt2 should be identical!\nPrompt1:\n%s\n\nPrompt2:\n%s", prompt1, prompt2)
	}
}

func TestSecretTokensShield(t *testing.T) {
	positives := []string{
		"GH_TOKEN=ghp_ABC123xyzSecretToken456Value",
		"github_pat_11ABCD1234567890abcdefghijklmnopqrstuvwxyz",
		"gho_1234567890abcdefghijklmnopqrstuvwxyz",
		"AWS_KEY=AKIAIOSFODNN7EXAMPLE",
		"sk-proj-1234567890abcdef1234567890",
		"-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----",
		"-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9w0BAQEFAASC...\n-----END PRIVATE KEY-----",
	}
	for _, p := range positives {
		if !reSecretTokens.MatchString(p) {
			t.Errorf("expected secret token regex to match %q", p)
		}
	}

	negatives := []string{
		"TODO: update token tomorrow",
		"Reminder: sync with github upstream later today.",
		"npm audit: 0 critical, 2 low vulnerabilities",
		"grep -i 'ERROR' /var/log/app.log: 0 matches",
		"DeprecationWarning: pkg_resources is deprecated",
		"status=passed, error_code=none",
	}
	for _, n := range negatives {
		if reSecretTokens.MatchString(n) {
			t.Errorf("expected secret token regex NOT to match %q", n)
		}
	}
}
