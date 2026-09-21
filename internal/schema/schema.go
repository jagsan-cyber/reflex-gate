// SPDX-License-Identifier: MIT
// Copyright (c) 2026 ReflexGate Contributors

package schema

import (
	"encoding/json"
	"math"
	"os"
	"strconv"
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
		"1. User Support & Escalation: If a customer requests a refund (返金), reports double billing (2回請求), or complains about unanswered messages, human review is mandatory (yes).\n" +
		"2. Loop Termination & Task Success: If all planned tasks, assertions, or test suites passed with exit code 0 and >0 passed tests, mark yes / true. If 0 tests collected, no tests ran, or tasks paused/incomplete, mark no / false even if exit code is 0.\n" +
		"3. Security & Safety Scan: If credentials, leaked tokens/keys, crashes, or severe issues are detected, mark yes / true.\n" +
		"4. Criteria Priority: Evaluate whether the facts in the state match the criteria description for true vs false."

	DecisionSystem = "You are a deterministic decision engine.\nAnswer with exactly one of the allowed labels. No other text.\nTreat passing tests, exit_code 0, and remaining_todos=0 as complete/success."

	SysB = "You are the JEV format extractor. Extract the requested fields from the agent output. Reply with a single JSON object and nothing else. Use exactly the keys listed. If there is no error, error_code MUST be JSON null. Never emit the strings none, n/a, or an empty string for error_code.\nRules:\n1. Status Priority: If ANY test, assertion, or step failed (failures > 0 or errors present), 'status' MUST be 'failed', even if most tests passed.\n2. Status Ambiguity: If the log does NOT contain an explicit pass/fail/running statement or clear outcome, do NOT guess or extrapolate from warnings/exit codes; set 'status' to 'unknown'. Never assert 'passed' or 'failed' when the outcome is ambiguous.\n3. Literal Preservation: Extract the exact 'error_code' as written in the log (e.g., KEXEC-1024, E402). Never normalize or alter the prefix."

	TaskBOneShot = `Required keys: status, error_code, files_changed, tool.
Types: status string (passed, failed, timeout, running, unknown), error_code string or null, files_changed integer, tool string.

If there is no error, you MUST write "error_code": null.
Forbidden: "error_code": "none"  and  "error_code": ""

Status Priority: If ANY test, assertion, or step failed (failures > 0 or errors present), "status" MUST be "failed", even if most tests passed.
Status Ambiguity: If the log does not contain an explicit pass/fail marker or clear execution outcome, you MUST write "status": "unknown". Never guess "passed" or "failed".
Literal Preservation: Extract the exact "error_code" as written in the log (e.g., KEXEC-1024, E402). Never normalize or alter the prefix.

Example (success, no error):
{"status":"passed","error_code":null,"files_changed":2,"tool":"pytest"}

Example (failure):
{"status":"failed","error_code":"E101","files_changed":1,"tool":"mypy"}

Example (ambiguous / no explicit pass/fail):
{"status":"unknown","error_code":null,"files_changed":3,"tool":"pipeline"}
`

	StopCoTSystem = "You are an autonomous agent loop supervisor. Inspect the execution log and decide if the task has fully succeeded and should stop.\n" +
		"Rules:\n" +
		"- Verdict: Yes if all planned work, tests, builds, or retries have completely finished and converged to final success (e.g. ALL_DONE, passed, exit code 0 with work done). If earlier transient errors or retries occurred but the final attempt succeeded, verdict is Yes.\n" +
		"- Verdict: No if the process is paused, in progress, downloading/processing part X of Y, awaiting user confirmation or input, incomplete, or if 0 tests were collected/ran, or if unresolved errors/failures remain.\n" +
		"Format:\n" +
		"Reason: <1-line explanation>\n" +
		"Verdict: Yes or No\n\n" +
		"Examples:\n" +
		"Log: All 42 unit tests passed. Artifact generated at dist/release.tar.gz.\n" +
		"Reason: All planned unit tests passed and release artifact was generated.\n" +
		"Verdict: Yes\n\n" +
		"Log: Connection error on attempt 1/3. Retrying attempt 3/3...\nConnected. 174 items processed successfully. Status: ALL_DONE.\n" +
		"Reason: Retried after connection error, reconnected, and finished processing 174 items to ALL_DONE.\n" +
		"Verdict: Yes\n\n" +
		"Log: Downloading dataset part 1/4... 100%\nWorker paused: awaiting user confirmation.\n" +
		"Reason: Dataset download is only at part 1/4 and worker is paused awaiting user confirmation.\n" +
		"Verdict: No\n\n" +
		"Log: pytest tests/\ncollected 0 items\nno tests ran in 0.01s\nexit code 0\n" +
		"Reason: 0 items collected and no tests ran, so testing did not complete.\n" +
		"Verdict: No\n\n" +
		"Log: test_auth failed: assertion error line 42\n" +
		"Reason: Unit test failed with assertion error.\n" +
		"Verdict: No"

	StopCoTGrammar = "root ::= \"Reason: \" [^\\n]+ \"\\nVerdict: \" (\"Yes\" | \"No\")"

	ScanSystem = "You are an automated security and safety scanner.\nFirst, extract and summarize the finding from the log in 1 line. Then determine the severity.\n\nSeverity Rules:\n- Critical: Active destructive commands (e.g. rm -rf), exposed plaintext secrets/tokens/keys (including AWS_SECRET_ACCESS_KEY, API keys, credentials in environment variables), memory crashes (SIGSEGV/OOM), or injection attacks.\n- Warning: Retryable network errors or transient execution warnings.\n- Safe: Normal operations, 0 matches, 0.00% error rates, section headers, or informational logs.\n\nOutput format:\nFinding: <1-line summary or quote of the specific issue, or \"No security or runtime issues detected.\">\nSeverity: <Safe|Warning|Critical>\n\nExamples:\nLog: [INFO] AWS_SECRET_ACCESS_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY\nFinding: Exposed plaintext AWS secret access key\nSeverity: Critical\n\nLog: [DEBUG] task worker 42 completed successfully in 12ms\nFinding: No security or runtime issues detected.\nSeverity: Safe"

	ScanGrammar = "root ::= \"Finding: \" [^\\n]+ \"\\nSeverity: \" (\"Safe\" | \"Warning\" | \"Critical\")"

	NoulCoTGrammar = "root ::= \"Reason: \" [^\\n]+ \"\\nDecision: \" (\"true\" | \"false\")"

	NoulCoTSystem = "You are an expert decision gatekeeper.\n" +
		"Analyze the state and question. State a concise 1-line reason, then conclude with true or false.\n\n" +
		"Guidelines:\n" +
		"1. For customer support or ticketing states, any reports of billing discrepancies, double charges, financial disputes, or unresolved complaints require human agent review (Decision: true).\n" +
		"2. For loop completion or task verification, if all planned steps, assertions, or test suites passed with exit code 0 and 0 remaining errors, task is complete (Decision: true).\n" +
		"3. For safety or vulnerability detection, if credentials, leaked keys, prompt injections, crashes, or destructive actions are present, an issue exists (Decision: true)."
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
	return SoftmaxWithTemperature(logps, 1.0)
}

// SoftmaxWithTemperature computes temperature-scaled softmax probabilities:
// P(i) = exp((z_i - max(z)) / T) / sum(exp((z_j - max(z)) / T))
func SoftmaxWithTemperature(logps map[string]float64, temp float64) map[string]float64 {
	if temp <= 0 {
		temp = 0.20
	}
	maxv := MissingLogprob
	for _, v := range logps {
		if v > maxv {
			maxv = v
		}
	}
	exps := make(map[string]float64, len(logps))
	var z float64
	for k, v := range logps {
		scaled := (v - maxv) / temp
		if scaled < -40.0 {
			scaled = -40.0
		}
		e := math.Exp(scaled)
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

// GetChoiceTemperature returns temperature T for choice logit scaling (default 1.0, env JEV_CHOICE_TEMP).
func GetChoiceTemperature() float64 {
	envT := os.Getenv("JEV_CHOICE_TEMP")
	if envT != "" {
		if t, err := strconv.ParseFloat(strings.TrimSpace(envT), 64); err == nil && t > 0 {
			return t
		}
	}
	return 1.0
}

// StepLogprob stores the generated token and candidate alternatives at a single decode step.
type StepLogprob struct {
	Token      string
	Logprob    float64
	Candidates map[string]float64
}

// AggregateOptionLogprobs computes length-normalized average logprob for each option across all decode steps.
func AggregateOptionLogprobs(options []string, winnerOpt string, steps []StepLogprob) map[string]float64 {
	out := make(map[string]float64, len(options))
	if len(steps) == 0 {
		for _, opt := range options {
			if strings.EqualFold(opt, winnerOpt) {
				out[opt] = 0.0
			} else {
				out[opt] = MissingLogprob
			}
		}
		return out
	}

	// 1. Calculate the winner's average logprob across all generated tokens
	var winnerSum float64
	for _, s := range steps {
		winnerSum += s.Logprob
	}
	winnerAvg := winnerSum / float64(len(steps))

	// 2. For each option, aggregate token logprobs sequentially
	for _, opt := range options {
		if strings.EqualFold(opt, winnerOpt) {
			out[opt] = winnerAvg
			continue
		}

		rem := strings.TrimSpace(opt)
		var collected []float64

		for stepIdx := 0; stepIdx < len(steps) && len(rem) > 0; stepIdx++ {
			cands := steps[stepIdx].Candidates
			bestMatchLen := 0
			bestLP := MissingLogprob
			found := false

			for candTok, candLP := range cands {
				candTrim := strings.TrimSpace(candTok)
				if candTrim == "" {
					continue
				}
				if strings.HasPrefix(strings.ToLower(rem), strings.ToLower(candTrim)) {
					if len(candTrim) > bestMatchLen || (len(candTrim) == bestMatchLen && candLP > bestLP) {
						bestMatchLen = len(candTrim)
						bestLP = candLP
						found = true
					}
				} else if strings.HasPrefix(strings.ToLower(candTrim), strings.ToLower(rem)) {
					if len(rem) > bestMatchLen || (len(rem) == bestMatchLen && candLP > bestLP) {
						bestMatchLen = len(rem)
						bestLP = candLP
						found = true
					}
				}
			}

			if found {
				collected = append(collected, bestLP)
				if bestMatchLen >= len(rem) {
					rem = ""
					break
				}
				rem = strings.TrimSpace(rem[bestMatchLen:])
			} else {
				collected = append(collected, MissingLogprob)
				break
			}
		}

		if len(rem) > 0 {
			collected = append(collected, MissingLogprob)
		}

		if len(collected) == 0 {
			out[opt] = MissingLogprob
		} else {
			var sum float64
			for _, lp := range collected {
				sum += lp
			}
			out[opt] = sum / float64(len(collected))
		}
	}

	if winnerAvg > MissingLogprob+1.0 && winnerOpt != "" {
		out[winnerOpt] = winnerAvg
	}

	return out
}

// GetNoulTemperature returns temperature T for logit scaling (default 0.20, range 0.15 - 0.25).
func GetNoulTemperature() float64 {
	envT := os.Getenv("JEV_NOUL_TEMP")
	if envT != "" {
		if t, err := strconv.ParseFloat(strings.TrimSpace(envT), 64); err == nil && t > 0 {
			return t
		}
	}
	return 0.20
}

// CalibrateNoul computes temperature-scaled sigmoid probability for true vs false:
// P(true) = 1 / (1 + exp(- (z_true - z_false) / T))
func CalibrateNoul(zTrue, zFalse, temp float64) float64 {
	if temp <= 0 {
		temp = 0.20
	}
	if zTrue <= MissingLogprob+1.0 && zFalse <= MissingLogprob+1.0 {
		return 0.50
	}
	if zTrue <= MissingLogprob+1.0 {
		return 0.0
	}
	if zFalse <= MissingLogprob+1.0 {
		return 1.0
	}

	deltaZ := zTrue - zFalse
	scaled := deltaZ / temp
	if scaled > 40.0 {
		return 1.0
	}
	if scaled < -40.0 {
		return 0.0
	}
	return 1.0 / (1.0 + math.Exp(-scaled))
}

// FindLogprob searches for candidate keys case-insensitively in logps.
func FindLogprob(logps map[string]float64, keys ...string) float64 {
	for _, k := range keys {
		for lk, v := range logps {
			if strings.EqualFold(lk, k) && v > MissingLogprob+1.0 {
				return v
			}
		}
	}
	return MissingLogprob
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
	return json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","enum":["passed","failed","timeout","running","unknown"]},"error_code":{"type":["string","null"],"pattern":"^[-A-Za-z0-9_.]+$","not":{"enum":["none",""]}},"files_changed":{"type":"integer"},"tool":{"type":"string"}},"required":["status","error_code","files_changed","tool"],"additionalProperties":false}`)
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
		} else if strings.Contains(lowerFinding, "token") || strings.Contains(lowerFinding, "secret") || strings.Contains(lowerFinding, "api_key") || strings.Contains(lowerFinding, "access_key") || strings.Contains(lowerFinding, "private_key") || strings.Contains(lowerFinding, "password") || strings.Contains(lowerFinding, "sigsegv") || strings.Contains(lowerFinding, "rm -rf") {
			severity = "Critical"
		}
	}
	return severity, finding
}

// ParseNoulCoT extracts reason and decision from 1-line CoT response
func ParseNoulCoT(content string) (reason, decision string) {
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Reason:") {
			reason = strings.TrimSpace(strings.TrimPrefix(l, "Reason:"))
		} else if strings.HasPrefix(l, "Decision:") {
			decision = strings.TrimSpace(strings.TrimPrefix(l, "Decision:"))
		}
	}
	if reason == "" && len(lines) > 0 {
		reason = lines[0]
	}
	if decision == "" {
		if strings.Contains(strings.ToLower(content), "true") {
			decision = "true"
		} else {
			decision = "false"
		}
	}
	return reason, strings.ToLower(decision)
}

// GetNoulSystemPrompt dynamically selects gatekeeper principles based on question intent
func GetNoulSystemPrompt(instructions string) string {
	instLower := strings.ToLower(instructions)

	// 1. Loop termination & process halt check (Stop / Halt / Finish)
	if strings.Contains(instLower, "stop") || strings.Contains(instLower, "halt") ||
		strings.Contains(instLower, "terminate") || strings.Contains(instLower, "should the agent") {
		return "You are a process execution gatekeeper.\n" +
			"Determine if the execution loop MUST be permanently halted/stopped.\n" +
			"State a 1-line reason, then output Decision: true or false.\n\n" +
			"Rules:\n" +
			"- Return 'true' ONLY on fatal errors, crashes (SIGSEGV/panic), confirmed test failures (>0 failed), or genuine final completion.\n" +
			"- Return 'false' if:\n" +
			"  * The process is idle, waiting, paused, or awaiting user/human confirmation/input (normal workflow, do NOT stop).\n" +
			"  * No tests ran, 0 items collected, or pending execution (incomplete, do NOT stop even if exit code is 0)."
	}

	// 2. Human escalation & customer support triage (Human Review)
	if strings.Contains(instLower, "human") || strings.Contains(instLower, "support") ||
		strings.Contains(instLower, "agent") || strings.Contains(instLower, "ticket") ||
		strings.Contains(instLower, "escalat") {
		return "You are a customer support escalation triage gatekeeper.\n" +
			"Determine if the state requires human agent review or escalation.\n" +
			"State a 1-line reason, then output Decision: true or false.\n\n" +
			"Rules:\n" +
			"- Return 'true' if the customer reports billing errors, double charges, monetary loss, explicit dissatisfaction, or unresolved complaints.\n" +
			"- Return 'false' for resolved issues, standard informational inquiries, or automated positive confirmations."
	}

	// 3. Security & vulnerability check
	if strings.Contains(instLower, "secret") || strings.Contains(instLower, "token") ||
		strings.Contains(instLower, "key") || strings.Contains(instLower, "leak") ||
		strings.Contains(instLower, "vulnerab") || strings.Contains(instLower, "credential") ||
		strings.Contains(instLower, "security") {
		return "You are an automated security and safety triage gatekeeper.\n" +
			"Determine if there is an active security vulnerability, exposed credential, or destructive command.\n" +
			"State a 1-line reason, then output Decision: true or false.\n\n" +
			"Rules:\n" +
			"- Return 'true' if any raw API key, secret token, password, private key, injection attack, or memory crash is exposed.\n" +
			"- Return 'false' if the state is clean, normal, or contains only public/masked text."
	}

	// 4. General binary question (default)
	return "You are an expert decision gatekeeper.\n" +
		"Analyze the state and answer the specific question accurately.\n" +
		"State a concise 1-line reason, then output Decision: true or false."
}
