package schema

import (
	"encoding/json"
	"math"
	"regexp"
	"strings"
)

const (
	ContextTrim    = 1500
	MissingLogprob = -99.0
	SlotDecision   = 0
	SlotExtract    = 1
	DecisionSystem = "You are a deterministic decision engine.\nAnswer with exactly one of the allowed labels. No other text.\nTreat passing tests, exit_code 0, and remaining_todos=0 as complete/success."
	SysB           = "You are the JEV format extractor. Extract the requested fields from the agent output. Reply with a single JSON object and nothing else. Use exactly the keys listed. If there is no error, error_code MUST be JSON null. Never emit the strings none, n/a, or an empty string for error_code."
	TaskBOneShot   = `Required keys: status, error_code, files_changed, tool.
Types: status string, error_code string or null, files_changed integer, tool string.

If there is no error, you MUST write "error_code": null.
Forbidden: "error_code": "none"  and  "error_code": ""

Example (success, no error):
{"status":"passed","error_code":null,"files_changed":2,"tool":"pytest"}

Example (failure):
{"status":"failed","error_code":"E101","files_changed":1,"tool":"mypy"}
`)


var ErrorLine = regexp.MustCompile(`(?m)^\[(?:ERROR|FATAL)\][^\n]*`)

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
			if tok == opt || strings.TrimSpace(tok) == opt {
				if lp > found {
					found = lp
				}
				if len(opt) > bestLen {
					bestLen = len(opt)
				}
				continue
			}
			if strings.HasPrefix(opt, tok) && len(tok) >= bestLen && len(tok) >= 2 {
				bestLen = len(tok)
				found = lp
			}
		}
		out[opt] = found
	}
	return out
}

func ExtractSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","enum":["passed","failed","timeout","running"]},"error_code":{"type":["string","null"],"pattern":"^E[0-9]+$","not":{"enum":["none",""]}},"files_changed":{"type":"integer"},"tool":{"type":"string"}},"required":["status","error_code","files_changed","tool"],"additionalProperties":false}`)
}
