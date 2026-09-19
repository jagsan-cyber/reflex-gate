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
