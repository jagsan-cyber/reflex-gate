package schema

import (
	"encoding/json"
	"math"
	"strings"
)

const (
	ContextTrim    = 1500
	MissingLogprob = -99.0
	SlotDecision   = 0
	SlotExtract    = 1
	SlotScan       = 2

	ModelJevLatest  = "jev-latest"
	ModelJevPreview = "jev-preview"
	ModelJev1130    = "jev-1.13.0"
	ResolvedModel   = "jev-1.13.0"

	SystemOneSystem = "You are a deterministic decision engine.\n" +
		"Read the state, the instructions, and the criteria, then answer with exactly one of the allowed labels. Output only the selected label and no other text.\n\n" +
		"Evaluation Rules:\n" +
		"1. User Support & Escalation: If a customer reports dissatisfaction, complaints, monetary disputes, or requests refunds, human verification is required (true / needs_review).\n" +
		"2. Loop Termination & Task Success: If all planned tasks, assertions, or test suites passed with exit code 0 and 0 remaining errors, mark true / Yes.\n" +
		"3. Security & Safety Scan: If credentials, leaked tokens/keys, crashes, or severe issues are detected, mark true for vulnerability detection.\n" +
		"4. Criteria Priority: Evaluate whether the facts in the state match the criteria description for true vs false."

	DecisionSystem = "You are a deterministic decision engine.\nAnswer with exactly one of the allowed labels. No other text.\nTreat passing tests, exit_code 0, and remaining_todos=0 as complete/success."

	SysB = "You are the JEV format extractor. Extract the requested fields from the agent output. Reply with a single JSON object and nothing else. Use exactly the keys listed. If there is no error, error_code MUST be JSON null. Never emit the strings none, n/a, or an empty string for error_code.\nRules:\n1. Status Priority: If ANY test, assertion, or step failed (failures > 0 or errors present), 'status' MUST be 'failed', even if most tests passed.\n2. Literal Preservation: Extract the exact 'error_code' as written in the log (e.g., KEXEC-1024, E402). Never normalize or alter the prefix."

	TaskBOneShot = `Required keys: status, error_code, files_changed, tool.
Types: status string, error_code string or null, files_changed integer, tool string.

If there is no error, you MUST write "error_code": null.
Forbidden: "error_code": "none"  and  "error_code": ""

Status Priority: If ANY test, assertion, or step failed (failures > 0 or errors present), "status" MUST be "failed", even if most tests passed.
Literal Preservation: Extract the exact "error_code" as written in the log (e.g., KEXEC-1024, E402). Never normalize or alter the prefix.

Example (success, no error):
{"status":"passed","error_code":null,"files_changed":2,"tool":"pytest"}

Example (failure):
{"status":"failed","error_code":"E101","files_changed":1,"tool":"mypy"}
`

	StopCoTSystem = "You are an autonomous agent loop supervisor. Inspect the execution log and decide if the task has fully succeeded and should stop.\n- Verdict: Yes if all planned work, tests, or retries finished successfully.\n- Verdict: No if 0 tests collected, work paused, in progress, or errors remain.\nFormat:\nReason: <1-line explanation>\nVerdict: Yes or No"

	StopCoTGrammar = "root ::= \"Reason: \" [^\\n]+ \"\\nVerdict: \" (\"Yes\" | \"No\")"

	ScanSystem = "You are an automated security and safety scanner.\nFirst, extract and summarize the finding from the log in 1 line. Then determine the severity.\n\nSeverity Rules:\n- Critical: Active destructive commands (e.g. rm -rf), exposed plaintext secrets/tokens/keys, memory crashes (SIGSEGV/OOM), or injection attacks.\n- Warning: Retryable network errors or transient execution warnings.\n- Safe: Normal operations, 0 matches, 0.00% error rates, section headers, or informational logs.\n\nOutput format:\nFinding: <1-line summary or quote of the specific issue, or \"No security or runtime issues detected.\">\nSeverity: <Safe|Warning|Critical>"

	ScanGrammar = "root ::= \"Finding: \" [^\\n]+ \"\\nSeverity: \" (\"Safe\" | \"Warning\" | \"Critical\")"
)

func OptionsGrammar(options []string) string {
	parts := make([]string, 0, len(options))
	for _, o := range options {
		esc := strings.ReplaceAll(o, `\`, `\\`)
		esc = strings.ReplaceAll(esc, `"`, `\"`)
		parts = append(parts, `"`+esc+`"`)
	}
	return "root ::= " + strings.Join(parts, " | ")
}

func TrimContext(text string) string {
	if len(text) <= ContextTrim {
		return text
	}
	return text[len(text)-ContextTrim:]
}

func Softmax(logps map[string]float64) map[string]float64 {
	maxv := MissingLogprob
	for _, v := range logps {
		if v > maxv {
			maxv = v
		}
	}
	exps := make(map[string]float64, len(logps))
	var z float64
	for k, v := range logps {
		e := math.Exp(v - maxv)
		exps[k] = e
		z += e
	}
	if z == 0 {
		z = 1
	}
	out := make(map[string]float64, len(logps))
	for k, e := range exps {
		out[k] = e / z
	}
	return out
}

func MatchOptionLogprobs(options []string, top []struct {
	Token   string
	Logprob float64
}) map[string]float64 {
	byTok := map[string]float64{}
	for _, item := range top {
		tok := item.Token
		lp := item.Logprob
		for _, key := range []string{tok, strings.TrimSpace(tok)} {
			if key == "" {
				continue
			}
			if prev, ok := byTok[key]; !ok || lp > prev {
				byTok[key] = lp
			}
		}
	}
	out := map[string]float64{}
	for _, opt := range options {
		found, ok := byTok[opt]
		bestLen := 0
		if ok {
			bestLen = len(opt)
		} else {
			found = MissingLogprob
		}
		for tok, lp := range byTok {
			if tok == "" {
				continue
			}
			trimmedTok := strings.TrimSpace(tok)
			if strings.EqualFold(tok, opt) || strings.EqualFold(trimmedTok, opt) {
				if lp > found {
					found = lp
				}
				if len(opt) > bestLen {
					bestLen = len(opt)
				}
				continue
			}
			if strings.HasPrefix(strings.ToLower(opt), strings.ToLower(trimmedTok)) && len(trimmedTok) >= bestLen && len(trimmedTok) >= 2 {
				bestLen = len(trimmedTok)
				if lp > found {
					found = lp
				}
			}
		}
		out[opt] = found
	}
	return out
}

func ExtractSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","enum":["passed","failed","timeout","running"]},"error_code":{"type":["string","null"],"pattern":"^[-A-Za-z0-9_.]+$","not":{"enum":["none",""]}},"files_changed":{"type":"integer"},"tool":{"type":"string"}},"required":["status","error_code","files_changed","tool"],"additionalProperties":false}`)
}

// ChatPrompt builds an im_start/im_end prompt for raw /completion
func ChatPrompt(system, user string) string {
	return "<|im_start|>system\n" + system + "<|im_end|>\n<|im_start|>user\n" + user + "<|im_end|>\n<|im_start|>assistant\n"
}

// ParseCoT extracts reason and verdict from "Reason: ...\nVerdict: Yes|No"
func ParseCoT(content string) (reason, verdict string) {
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Reason:") {
			reason = strings.TrimSpace(strings.TrimPrefix(l, "Reason:"))
		} else if strings.HasPrefix(l, "Verdict:") {
			verdict = strings.TrimSpace(strings.TrimPrefix(l, "Verdict:"))
		}
	}
	if reason == "" && len(lines) > 0 {
		reason = lines[0]
	}
	if verdict == "" {
		if strings.Contains(content, "Yes") {
			verdict = "Yes"
		} else {
			verdict = "No"
		}
	}
	return reason, verdict
}

// ParseScan extracts severity and finding from "Severity: ...\nFinding: ..."
func ParseScan(content string) (severity, finding string) {
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Finding:") {
			finding = strings.TrimSpace(strings.TrimPrefix(l, "Finding:"))
		} else if strings.HasPrefix(l, "Severity:") {
			severity = strings.TrimSpace(strings.TrimPrefix(l, "Severity:"))
		}
	}
	switch severity {
	case "Safe", "Warning", "Critical":
		// valid
	default:
		severity = "Safe"
	}
	if finding == "" {
		finding = "No security or runtime issues detected."
	}
	lowerFinding := strings.ToLower(finding)
	if severity == "Warning" {
		if strings.Contains(lowerFinding, "0 critical") || (strings.Contains(lowerFinding, "audit") && strings.Contains(lowerFinding, "low")) {
			severity = "Safe"
		} else if strings.Contains(lowerFinding, "token") || strings.Contains(lowerFinding, "secret") || strings.Contains(lowerFinding, "api_key") || strings.Contains(lowerFinding, "password") || strings.Contains(lowerFinding, "sigsegv") || strings.Contains(lowerFinding, "rm -rf") {
			severity = "Critical"
		}
	}
	return severity, finding
}
