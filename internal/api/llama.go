package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"local-jev/internal/schema"
)

type Llama struct {
	Root   string
	Client *http.Client
}

func NewLlama(root string) *Llama {
	if root == "" {
		root = "http://127.0.0.1:8080"
	}
	root = strings.TrimRight(root, "/")
	root = strings.TrimSuffix(root, "/v1")
	return &Llama{Root: root, Client: &http.Client{Timeout: 10 * time.Minute}}
}

func cleanModelName(raw string) string {
	if raw == "" {
		return ""
	}
	raw = strings.ReplaceAll(raw, "\\", "/")
	if idx := strings.LastIndex(raw, "/"); idx >= 0 {
		raw = raw[idx+1:]
	}
	raw = strings.TrimSuffix(raw, ".gguf")
	raw = strings.TrimSuffix(raw, ".bin")
	return raw
}

func (l *Llama) ModelsOK() (string, int, error) {
	resp, err := l.Client.Get(l.Root + "/v1/models")
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("models HTTP %d: %s", resp.StatusCode, b)
	}
	var body struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	id := ""
	if len(body.Data) > 0 {
		id = cleanModelName(body.Data[0].ID)
	}
	n := 0
	if sr, err := l.Client.Get(l.Root + "/slots"); err == nil {
		defer sr.Body.Close()
		var slots []any
		if json.NewDecoder(sr.Body).Decode(&slots) == nil {
			n = len(slots)
		}
	}
	return id, n, nil
}

func (l *Llama) Completion(prompt string, nPredict, idSlot, nProbs int, grammar string, jsonSchema any) (map[string]any, error) {
	req := map[string]any{
		"prompt":       prompt,
		"n_predict":    nPredict,
		"temperature":  0.0,
		"cache_prompt": true,
		"id_slot":      idSlot,
		"n_probs":      nProbs,
	}
	if grammar != "" {
		req["grammar"] = grammar
	}
	if jsonSchema != nil {
		req["json_schema"] = jsonSchema
	}
	return l.postJSON("/completion", req)
}

func (l *Llama) ChatDecide(question string, options []string, context string) (result string, dist map[string]float64, logps map[string]float64, content string, promptN, predN int, err error) {
	return l.ChatDecideSlot(question, options, context, schema.SlotDecision)
}

func (l *Llama) ChatDecideSlot(question string, options []string, context string, slotID int) (result string, dist map[string]float64, logps map[string]float64, content string, promptN, predN int, err error) {
	user := fmt.Sprintf("Instructions:\n%s\nAllowed options:\n%s\n\nState:\n%s\n",
		question, strings.Join(options, ", "), schema.TrimContext(context))
	nPredict := 8
	for _, o := range options {
		if len(o) > nPredict {
			nPredict = len(o)
		}
	}
	req := map[string]any{
		"model":         "local",
		"temperature":   0.0,
		"max_tokens":    nPredict,
		"logprobs":      true,
		"top_logprobs":  20,
		"messages": []map[string]string{
			{"role": "system", "content": schema.SystemOneSystem},
			{"role": "user", "content": user},
		},
		"grammar":              schema.OptionsGrammar(options),
		"id_slot":              slotID,
		"cache_prompt":         false,
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	}
	raw, err := l.postJSON("/v1/chat/completions", req)
	if err != nil {
		return "", nil, nil, "", 0, 0, err
	}
	content = strings.TrimSpace(jsonPathString(raw, "choices", 0, "message", "content"))
	var top []struct {
		Token   string
		Logprob float64
	}
	if ch, ok := jsonIndex(raw["choices"], 0); ok {
		if lp, ok := ch["logprobs"].(map[string]any); ok {
			if arr, ok := lp["content"].([]any); ok && len(arr) > 0 {
				if first, ok := arr[0].(map[string]any); ok {
					tok, _ := first["token"].(string)
					lg, _ := first["logprob"].(float64)
					top = append(top, struct {
						Token   string
						Logprob float64
					}{tok, lg})
					if tl, ok := first["top_logprobs"].([]any); ok {
						for _, x := range tl {
							m, _ := x.(map[string]any)
							if m == nil {
								continue
							}
							t, _ := m["token"].(string)
							p, _ := m["logprob"].(float64)
							top = append(top, struct {
								Token   string
								Logprob float64
							}{t, p})
						}
					}
				}
			}
		}
	}
	logps = schema.MatchOptionLogprobs(options, top)
	dist = schema.Softmax(logps)
	result = content
	matched := false
	for _, o := range options {
		if content == o || strings.HasPrefix(content, o) {
			result = o
			matched = true
			break
		}
	}
	if !matched {
		best, bestP := "", -1.0
		for k, v := range dist {
			if v > bestP {
				best, bestP = k, v
			}
		}
		result = best
	}
	if u, ok := raw["usage"].(map[string]any); ok {
		promptN = asInt(u["prompt_tokens"])
		predN = asInt(u["completion_tokens"])
	}
	return result, dist, logps, content, promptN, predN, nil
}

// DecideCoT uses 1-line Chain-of-Thought reasoning with GBNF grammar
func (l *Llama) DecideCoT(logText string) (verdict, reason string, metrics map[string]any, err error) {
	userContent := "[Execution Log]\n" + schema.TrimContext(logText)
	prompt := schema.ChatPrompt(schema.StopCoTSystem, userContent)

	t0 := time.Now()
	data, err := l.Completion(prompt, 80, schema.SlotDecision, 0, schema.StopCoTGrammar, nil)
	if err != nil {
		return "", "", nil, err
	}
	wallS := time.Since(t0).Seconds()

	content, _ := data["content"].(string)
	reason, verdict = schema.ParseCoT(content)
	metrics = TimingsMetrics(data, wallS)
	metrics["raw"] = strings.TrimSpace(content)
	return verdict, reason, metrics, nil
}

// DecideNoulCoT evaluates noul queries using 1-line CoT reasoning with GBNF grammar
func (l *Llama) DecideNoulCoT(instructions, criteria, stateText string, slotID int) (decision, reason string, promptN, predN int, latMs int64, err error) {
	userContent := "[State]\n" + schema.TrimContext(stateText) + "\n\n[Question]\n" + instructions
	if criteria != "" {
		userContent += "\n\n[Criteria]\n" + criteria
	}
	systemPrompt := schema.GetNoulSystemPrompt(instructions)
	prompt := schema.ChatPrompt(systemPrompt, userContent)

	t0 := time.Now()
	data, err := l.Completion(prompt, 80, slotID, 0, schema.NoulCoTGrammar, nil)
	if err != nil {
		return "", "", 0, 0, 0, err
	}
	latMs = time.Since(t0).Milliseconds()

	content, _ := data["content"].(string)
	reason, decision = schema.ParseNoulCoT(content)

	if t, ok := data["timings"].(map[string]any); ok {
		promptN = asInt(t["prompt_n"])
		predN = asInt(t["predicted_n"])
	}
	if predN == 0 {
		predN = asInt(data["tokens_predicted"])
	}
	if u, ok := data["usage"].(map[string]any); ok {
		if promptN == 0 {
			promptN = asInt(u["prompt_tokens"])
		}
		if predN == 0 {
			predN = asInt(u["completion_tokens"])
		}
	}
	return decision, reason, promptN, predN, latMs, nil
}

// ScanLog performs semantic security/runtime scanning with GBNF grammar
func (l *Llama) ScanLog(logText string) (severity, finding string, metrics map[string]any, err error) {
	userContent := "[Log to scan]\n" + schema.TrimContext(logText)
	prompt := schema.ChatPrompt(schema.ScanSystem, userContent)

	t0 := time.Now()
	data, err := l.Completion(prompt, 100, schema.SlotScan, 0, schema.ScanGrammar, nil)
	if err != nil {
		return "", "", nil, err
	}
	wallS := time.Since(t0).Seconds()

	content, _ := data["content"].(string)
	severity, finding = schema.ParseScan(content)
	if severity == "Warning" {
		lowerLog := strings.ToLower(logText)
		if strings.Contains(lowerLog, "0 critical") && (strings.Contains(lowerLog, "audit") || strings.Contains(lowerLog, "scanned packages") || strings.Contains(lowerLog, "vulnerabilities")) {
			severity = "Safe"
		}
	}
	metrics = TimingsMetrics(data, wallS)
	metrics["raw"] = strings.TrimSpace(content)
	return severity, finding, metrics, nil
}

func (l *Llama) postJSON(path string, req map[string]any) (map[string]any, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	resp, err := l.Client.Post(l.Root+path, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("llm HTTP %d: %s", resp.StatusCode, truncate(string(body), 500))
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func jsonIndex(v any, i int) (map[string]any, bool) {
	arr, ok := v.([]any)
	if !ok || i < 0 || i >= len(arr) {
		return nil, false
	}
	m, ok := arr[i].(map[string]any)
	return m, ok
}

func jsonPathString(root map[string]any, keys ...any) string {
	var cur any = root
	for _, k := range keys {
		switch t := k.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return ""
			}
			cur = m[t]
		case int:
			m, ok := jsonIndex(cur, t)
			if !ok {
				return ""
			}
			cur = m
		}
	}
	s, _ := cur.(string)
	return s
}

func asInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	default:
		return 0
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func TimingsMetrics(data map[string]any, wallS float64) map[string]any {
	t, _ := data["timings"].(map[string]any)
	if t == nil {
		t = map[string]any{}
	}
	promptN := asInt(t["prompt_n"])
	predN := asInt(t["predicted_n"])
	if predN == 0 {
		predN = asInt(data["tokens_predicted"])
	}
	promptMS := asFloat(t["prompt_ms"])
	predMS := asFloat(t["predicted_ms"])
	ttft := wallS
	if promptMS > 0 {
		ttft = promptMS / 1000
	}
	decodeS := predMS / 1000
	toks := 0.0
	if decodeS > 0 && predN > 0 {
		toks = float64(predN) / decodeS
	}
	return map[string]any{
		"ttft_s":            ttft,
		"decode_s":          decodeS,
		"total_s":           wallS,
		"prompt_tokens":     promptN,
		"completion_tokens": predN,
		"tok_s":             toks,
	}
}

func asFloat(v any) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int:
		return float64(t)
	default:
		return 0
	}
}
