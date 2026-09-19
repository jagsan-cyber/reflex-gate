package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"embed"
	"log"
	"os"

	"local-jev/internal/schema"
)

var reSecretTokens = regexp.MustCompile(`(?i)(` +
	`ghp_[A-Za-z0-9_]{20,}|` + // GitHub Personal Access Token
	`github_pat_[A-Za-z0-9_]{22,}|` + // GitHub Fine-grained PAT
	`gho_[A-Za-z0-9_]{20,}|` + // GitHub OAuth Token
	`AKIA[0-9A-Z]{16}|` + // AWS Access Key ID
	`sk-[A-Za-z0-9_-]{20,}|` + // OpenAI / Generic API Secret Key
	`-----BEGIN (?:[A-Z0-9_-]+ )?PRIVATE KEY-----` + // SSH / TLS private key
`)`)

//go:embed embed/demo.html
var demoFS embed.FS

type RequestEvent struct {
	Task       string
	SlotID     int
	LatencyMs  int64
	TTFTMs     int64
	TokPerSec  float64
	PromptToks int
	OutToks    int
	Summary    string
	Timestamp  time.Time
}

type Server struct {
	Llama    *Llama
	http     *http.Server
	mu       sync.Mutex
	OnEvent  func(ev RequestEvent)
	AuthMode string // off | loose | strict
	APIKey   string
}

func (s *Server) emit(ev RequestEvent) {
	ev.Timestamp = time.Now()
	s.mu.Lock()
	cb := s.OnEvent
	s.mu.Unlock()
	if cb != nil {
		cb(ev)
	}
}

func NewServer(llamaRoot string) *Server {
	return &Server{
		Llama:    NewLlama(llamaRoot),
		AuthMode: "off",
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, "/demo", http.StatusFound)
	})
	mux.HandleFunc("/demo", func(w http.ResponseWriter, r *http.Request) {
		b, err := demoFS.ReadFile("embed/demo.html")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	mux.HandleFunc("/health", s.health)
	mux.HandleFunc("/jev/schema", s.schema)
	mux.HandleFunc("/jev/stop", s.stop)
	mux.HandleFunc("/jev/extract", s.extract)
	mux.HandleFunc("/jev/scan", s.scan)
	mux.HandleFunc("/v1/systemone", s.systemone)
	mux.HandleFunc("/v1/models", s.models)
	mux.HandleFunc("/jev/raw", s.raw)
	return cors(mux)
}

func (s *Server) Start(addr string) error {
	s.mu.Lock()
	s.http = &http.Server{Addr: addr, Handler: s.Handler()}
	srv := s.http
	s.mu.Unlock()
	return srv.ListenAndServe()
}

func (s *Server) Stop() error {
	s.mu.Lock()
	srv := s.http
	s.mu.Unlock()
	if srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readLog(r *http.Request) (string, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return "", err
	}
	var body struct {
		Log string `json:"log"`
	}
	if err := json.Unmarshal(b, &body); err != nil {
		return "", err
	}
	if strings.TrimSpace(body.Log) == "" {
		return "", errEmpty
	}
	return body.Log, nil
}

var errEmpty = errStr("log is required")

type errStr string

func (e errStr) Error() string { return string(e) }

func (s *Server) authMode() string {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("JEV_AUTH")))
	if mode == "" {
		mode = strings.ToLower(strings.TrimSpace(s.AuthMode))
	}
	if mode != "loose" && mode != "strict" {
		return "off"
	}
	return mode
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	id, n, err := s.Llama.ModelsOK()
	if err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "llm_ok": false, "llm": s.Llama.Root, "n_slots": n, "auth": s.authMode(), "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"ok": true, "llm_ok": true, "llm": s.Llama.Root, "model": id, "n_slots": n,
		"auth": s.authMode(),
		"slots": map[string]int{
			"decision": schema.SlotDecision,
			"extract":  schema.SlotExtract,
			"scan":     schema.SlotScan,
		},
		"warn_parallel": n < 3,
	})
}

func (s *Server) schema(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"stop":      map[string]any{"type": "cot", "grammar": "1-line reason + Yes/No", "slot": schema.SlotDecision},
		"extract":   map[string]any{"schema": schema.ExtractSchema(), "slot": schema.SlotExtract},
		"scan":      map[string]any{"type": "semantic", "backend": "llm", "slot": schema.SlotScan},
		"systemone": map[string]any{"path": "/v1/systemone", "types": []string{"noul", "choice"}, "slot": schema.SlotDecision},
	})
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	logText, err := readLog(r)
	if err != nil {
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}
	t0 := time.Now()
	verdict, reason, metrics, err := s.Llama.DecideCoT(logText)
	if err != nil {
		writeJSON(w, 502, map[string]string{"detail": err.Error()})
		return
	}
	latencyMs := time.Since(t0).Milliseconds()
	ttftMs := int64(asFloat(metrics["ttft_s"]) * 1000)
	toks := asFloat(metrics["tok_s"])
	pn := asInt(metrics["prompt_tokens"])
	cn := asInt(metrics["completion_tokens"])

	s.emit(RequestEvent{
		Task:       "Task A (Stop+CoT)",
		SlotID:     schema.SlotDecision,
		LatencyMs:  latencyMs,
		TTFTMs:     ttftMs,
		TokPerSec:  toks,
		PromptToks: pn,
		OutToks:    cn,
		Summary:    fmt.Sprintf("stop=%v (%s) reason=%s", strings.EqualFold(verdict, "Yes"), verdict, truncate(reason, 60)),
	})
	writeJSON(w, 200, map[string]any{
		"stop":    strings.EqualFold(verdict, "Yes"),
		"raw":     verdict,
		"reason":  reason,
		"metrics": metrics,
	})
}

func (s *Server) extract(w http.ResponseWriter, r *http.Request) {
	logText, err := readLog(r)
	if err != nil {
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}

	// Defensively protect against oversized logs exceeding slot context window
	if len(logText) > 12000 {
		logText = logText[len(logText)-12000:]
	}

	userMsg := buildExtractUserPrompt(logText)

	if os.Getenv("JEV_DEBUG") != "" {
		log.Printf("[extract] user len=%d", len(userMsg))
	}

	t0 := time.Now()
	req := map[string]any{
		"model":        "local",
		"temperature":  0.0,
		"max_tokens":   256,
		"messages": []map[string]string{
			{"role": "system", "content": schema.SysB},
			{"role": "user", "content": userMsg},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "jev_extract",
				"schema": schema.ExtractSchema(),
				"strict": true,
			},
		},
		"json_schema":          schema.ExtractSchema(),
		"id_slot":              schema.SlotExtract,
		"cache_prompt":         true,
		"chat_template_kwargs": map[string]any{"enable_thinking": false},
	}
	data, err := s.Llama.postJSON("/v1/chat/completions", req)
	if err != nil {
		writeJSON(w, 502, map[string]string{"detail": err.Error()})
		return
	}
	text := jsonPathString(data, "choices", 0, "message", "content")
	log.Printf("[DEBUG Extract Output] raw len=%d: %q", len(text), text)
	cleanJSON := strings.TrimSpace(text)
	if strings.HasPrefix(cleanJSON, "```") {
		if idx := strings.Index(cleanJSON, "\n"); idx != -1 {
			cleanJSON = cleanJSON[idx+1:]
		}
		if idx := strings.LastIndex(cleanJSON, "```"); idx != -1 {
			cleanJSON = cleanJSON[:idx]
		}
		cleanJSON = strings.TrimSpace(cleanJSON)
	}

	start, end := strings.Index(cleanJSON, "{"), strings.LastIndex(cleanJSON, "}")
	if start < 0 || end <= start {
		log.Printf("[WARN] Extract did not return JSON brackets | raw: %q | cleaned: %q", text, cleanJSON)
		writeJSON(w, 422, map[string]string{"detail": "JEV did not return JSON: " + text})
		return
	}
	raw := cleanJSON[start : end+1]
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		log.Printf("[ERROR] Extract JSON parse failed: %v | raw: %q | cleaned: %q", err, text, raw)
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}
	metrics := TimingsMetrics(data, time.Since(t0).Seconds())
	latMs := time.Since(t0).Milliseconds()
	ttftMs := int64(asFloat(metrics["ttft_s"]) * 1000)
	toks := asFloat(metrics["tok_s"])
	pn := asInt(metrics["prompt_tokens"])
	cn := asInt(metrics["completion_tokens"])
	summary := fmt.Sprintf("status=%v, tool=%v", parsed["status"], parsed["tool"])
	s.emit(RequestEvent{
		Task:       "Task B (Extract)",
		SlotID:     schema.SlotExtract,
		LatencyMs:  latMs,
		TTFTMs:     ttftMs,
		TokPerSec:  toks,
		PromptToks: pn,
		OutToks:    cn,
		Summary:    summary,
	})
	writeJSON(w, 200, map[string]any{
		"result":  parsed,
		"raw":     raw,
		"metrics": metrics,
	})
}

func buildExtractUserPrompt(logText string) string {
	if strings.Contains(logText, "Required keys:") || strings.Contains(logText, "Agent output:") {
		return logText
	}
	return schema.TaskBOneShot + "\nAgent output:\n```\n" + logText + "\n```\nRemember: no error means \"error_code\": null, never \"none\"."
}

func (s *Server) scan(w http.ResponseWriter, r *http.Request) {
	logText, err := readLog(r)
	if err != nil {
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}

	// 1. Fast-path secret shield
	if matched := reSecretTokens.FindString(logText); matched != "" {
		masked := matched
		if len(matched) > 12 {
			masked = matched[:8] + "..."
		}
		finding := fmt.Sprintf("Detected plaintext secret token in log (%s)", masked)
		s.emit(RequestEvent{
			Task:       "Task C (Scan)",
			SlotID:     schema.SlotScan,
			LatencyMs:  0,
			TTFTMs:     0,
			TokPerSec:  0,
			PromptToks: 0,
			OutToks:    0,
			Summary:    fmt.Sprintf("severity=Critical finding=%s [FAST-PATH]", truncate(finding, 40)),
		})
		writeJSON(w, 200, map[string]any{
			"severity": "Critical",
			"finding":  finding,
			"action":   "halt_loop",
			"metrics": map[string]any{
				"latency_ms": 0.1,
				"fast_path":  true,
			},
		})
		return
	}

	// 2. Fall-through to LLM semantic scan (Slot 2)
	t0 := time.Now()
	severity, finding, metrics, err := s.Llama.ScanLog(logText)
	if err != nil {
		writeJSON(w, 502, map[string]string{"detail": err.Error()})
		return
	}
	latencyMs := time.Since(t0).Milliseconds()
	ttftMs := int64(asFloat(metrics["ttft_s"]) * 1000)
	toks := asFloat(metrics["tok_s"])
	pn := asInt(metrics["prompt_tokens"])
	cn := asInt(metrics["completion_tokens"])

	action := "continue"
	if severity == "Critical" {
		action = "halt_loop"
	}

	s.emit(RequestEvent{
		Task:       "Task C (Scan)",
		SlotID:     schema.SlotScan,
		LatencyMs:  latencyMs,
		TTFTMs:     ttftMs,
		TokPerSec:  toks,
		PromptToks: pn,
		OutToks:    cn,
		Summary:    fmt.Sprintf("severity=%s finding=%s", severity, truncate(finding, 50)),
	})
	writeJSON(w, 200, map[string]any{
		"severity": severity,
		"finding":  finding,
		"action":   action,
		"metrics":  metrics,
	})
}

func (s *Server) raw(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Prompt    string `json:"prompt"`
		System    string `json:"system"`
		MaxTokens int    `json:"max_tokens"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}
	if body.System == "" {
		body.System = "Follow the user instruction exactly. No extra words."
	}
	if body.MaxTokens <= 0 {
		body.MaxTokens = 30
	}
	prompt := body.System + "\n\n" + body.Prompt
	t0 := time.Now()
	data, err := s.Llama.Completion(prompt, body.MaxTokens, schema.SlotDecision, 0, "", nil)
	if err != nil {
		writeJSON(w, 502, map[string]string{"detail": err.Error()})
		return
	}
	text, _ := data["content"].(string)
	metrics := TimingsMetrics(data, time.Since(t0).Seconds())
	s.emit(RequestEvent{
		Task:       "Raw Prompt",
		SlotID:     0,
		LatencyMs:  time.Since(t0).Milliseconds(),
		TTFTMs:     int64(asFloat(metrics["ttft_s"]) * 1000),
		TokPerSec:  asFloat(metrics["tok_s"]),
		Summary:    strings.ReplaceAll(strings.TrimSpace(text), "\n", " "),
	})
	writeJSON(w, 200, map[string]any{
		"text":    text,
		"metrics": metrics,
	})
}

func (s *Server) models(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(r) {
		writeJSON(w, 401, map[string]string{"detail": "Invalid API key"})
		return
	}
	writeJSON(w, 200, map[string]any{
		"data": []string{schema.ModelJevLatest, schema.ModelJevPreview, schema.ModelJev1130},
	})
}

func (s *Server) checkAuth(r *http.Request) bool {
	mode := s.authMode()
	if mode == "off" {
		// Complete bypass: 200 OK regardless of whether header is missing or dummy
		return true
	}

	// Key precedence: env TYPESAFE_API_KEY > s.APIKey
	expectedKey := strings.TrimSpace(os.Getenv("TYPESAFE_API_KEY"))
	if expectedKey == "" {
		expectedKey = strings.TrimSpace(s.APIKey)
	}

	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	var token string
	if auth != "" {
		parts := strings.SplitN(auth, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			token = strings.TrimSpace(parts[1])
		}
	}

	if mode == "loose" {
		// If key is not configured or no Authorization header sent, allow access.
		if expectedKey == "" || auth == "" {
			return true
		}
		return token != "" && token == expectedKey
	}

	if mode == "strict" {
		// Strict test mode: must provide Authorization: Bearer <key> matching expectedKey
		if expectedKey == "" {
			return true // if no key is configured, cannot validate
		}
		return token != "" && token == expectedKey
	}

	return true
}

type systemOneQuestion struct {
	Type         string          `json:"type"`
	Instructions string          `json:"instructions"`
	Criteria     json.RawMessage `json:"criteria"`
}

type systemOneRequest struct {
	Model     string                        `json:"model"`
	State     any                           `json:"state"`
	Questions map[string]systemOneQuestion `json:"questions"`
}

func (s *Server) systemone(w http.ResponseWriter, r *http.Request) {
	if !s.checkAuth(r) {
		writeJSON(w, 401, map[string]string{"detail": "Invalid API key"})
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, 422, map[string]string{"detail": "failed to read request body: " + err.Error()})
		return
	}

	// 1. Check if legacy single-question request: {"type": "...", "question": "...", ...}
	var legacyCheck struct {
		Type     string   `json:"type"`
		Question string   `json:"question"`
		Options  []string `json:"options"`
		Context  string   `json:"context"`
	}
	if err := json.Unmarshal(bodyBytes, &legacyCheck); err == nil && legacyCheck.Question != "" {
		opts := legacyCheck.Options
		if len(opts) == 0 {
			if legacyCheck.Type == "noul" {
				opts = []string{"true", "false"}
			} else {
				writeJSON(w, 422, map[string]string{"detail": "choice requires options"})
				return
			}
		}
		t0 := time.Now()
		result, dist, logps, _, _, _, err := s.Llama.ChatDecide(legacyCheck.Question, opts, legacyCheck.Context)
		if err != nil {
			writeJSON(w, 502, map[string]string{"detail": err.Error()})
			return
		}
		if legacyCheck.Type == "noul" {
			temp := schema.GetNoulTemperature()
			zTrue := schema.FindLogprob(logps, "true", "yes")
			zFalse := schema.FindLogprob(logps, "false", "no")
			pTrue := schema.CalibrateNoul(zTrue, zFalse, temp)
			dist["true"] = math.Round(pTrue*100) / 100
			dist["false"] = math.Round((1.0-pTrue)*100) / 100
			if _, ok := dist["Yes"]; ok {
				dist["Yes"] = dist["true"]
				dist["No"] = dist["false"]
			}
		}
		p := dist[result]
		latMs := time.Since(t0).Milliseconds()
		s.emit(RequestEvent{
			Task:      "SystemOne (" + legacyCheck.Type + ")",
			SlotID:    schema.SlotDecision,
			LatencyMs: latMs,
			TTFTMs:    latMs,
			Summary:   fmt.Sprintf("%s (p=%.2f)", result, p),
		})
		writeJSON(w, 200, map[string]any{
			"type":         legacyCheck.Type,
			"result":       result,
			"p":            p,
			"distribution": dist,
			"latency_ms":   float64(latMs),
		})
		return
	}

	// 2. TypeSafe official wire-compatible /v1/systemone request
	var req systemOneRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		writeJSON(w, 422, map[string]string{"detail": "invalid JSON: " + err.Error()})
		return
	}

	// Model validation
	if req.Model == "" {
		writeJSON(w, 422, map[string]string{"detail": "model is required"})
		return
	}
	switch req.Model {
	case schema.ModelJevLatest, schema.ModelJevPreview, schema.ModelJev1130:
	default:
		writeJSON(w, 422, map[string]string{"detail": "model must be jev-latest, jev-preview, or jev-1.13.0"})
		return
	}

	// State validation
	if req.State == nil {
		writeJSON(w, 422, map[string]string{"detail": "state is required"})
		return
	}
	var stateStr string
	switch sVal := req.State.(type) {
	case string:
		stateStr = sVal
	default:
		b, _ := json.Marshal(sVal)
		stateStr = string(b)
	}

	// Questions validation
	if len(req.Questions) == 0 {
		writeJSON(w, 422, map[string]string{"detail": "questions is required and must not be empty"})
		return
	}

	// Validate each question definition prior to execution
	type validatedQ struct {
		id       string
		qType    string
		instr    string
		options  []string
		legend   map[string]string
		rawCrit  string
	}
	vQuestions := make([]validatedQ, 0, len(req.Questions))

	for qID, q := range req.Questions {
		qType := strings.ToLower(strings.TrimSpace(q.Type))
		instr := strings.TrimSpace(q.Instructions)
		if qType == "" {
			writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: type is required", qID)})
			return
		}
		if instr == "" && qType != "noul" {
			writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: instructions is required", qID)})
			return
		}

		vq := validatedQ{id: qID, qType: qType, instr: instr}

		switch qType {
		case "noul":
			if instr == "" && len(q.Criteria) == 0 {
				writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: instructions is required when criteria is omitted", qID)})
				return
			}
			vq.options = []string{"true", "false"}
			if len(q.Criteria) > 0 {
				trueDesc := "The condition or statement is true / YES"
				falseDesc := "The condition or statement is false / NO"
				var critMap map[string]any
				if err := json.Unmarshal(q.Criteria, &critMap); err == nil && len(critMap) > 0 {
					for k, v := range critMap {
						lk := strings.ToLower(strings.TrimSpace(k))
						if lk == "true" || lk == "yes" {
							trueDesc = fmt.Sprintf("%v", v)
						} else if lk == "false" || lk == "no" {
							falseDesc = fmt.Sprintf("%v", v)
						}
					}
				} else {
					var s string
					if err := json.Unmarshal(q.Criteria, &s); err == nil && strings.TrimSpace(s) != "" {
						trueDesc = strings.TrimSpace(s)
					}
				}
				var sb strings.Builder
				sb.WriteString(fmt.Sprintf("- true: %s\n", trueDesc))
				sb.WriteString(fmt.Sprintf("- false: %s\n", falseDesc))
				vq.rawCrit = sb.String()
			}

		case "choice":
			if len(q.Criteria) == 0 {
				writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: choice question requires criteria", qID)})
				return
			}
			var critMap map[string]any
			if err := json.Unmarshal(q.Criteria, &critMap); err != nil || len(critMap) == 0 {
				writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: choice question requires criteria with at least one option", qID)})
				return
			}
			if len(critMap) > 255 {
				writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: choice question criteria exceeds maximum of 255 options", qID)})
				return
			}
			opts := make([]string, 0, len(critMap))
			var sb strings.Builder
			for opt, desc := range critMap {
				opts = append(opts, opt)
				sb.WriteString(fmt.Sprintf("- %s: %v\n", opt, desc))
			}
			vq.options = opts
			vq.rawCrit = sb.String()

		case "score":
			if len(q.Criteria) == 0 {
				writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: score question requires criteria with 2-10 levels", qID)})
				return
			}
			var levels []any
			if err := json.Unmarshal(q.Criteria, &levels); err != nil {
				writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: score question requires criteria as an ordered array", qID)})
				return
			}
			if len(levels) < 2 || len(levels) > 10 {
				writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: score question requires criteria with 2-10 levels", qID)})
				return
			}
			opts := make([]string, 0, len(levels))
			legend := make(map[string]string, len(levels))
			var sb strings.Builder
			for idx, lv := range levels {
				key := fmt.Sprintf("%d", idx)
				val := fmt.Sprintf("%v", lv)
				opts = append(opts, key)
				legend[key] = val
				sb.WriteString(fmt.Sprintf("%s: %s\n", key, val))
			}
			vq.options = opts
			vq.legend = legend
			vq.rawCrit = sb.String()

		default:
			writeJSON(w, 422, map[string]string{"detail": fmt.Sprintf("question %q: type must be noul, choice, or score", qID)})
			return
		}

		vQuestions = append(vQuestions, vq)
	}

	// Execute questions in parallel across slots
	type qResult struct {
		id       string
		answer   map[string]any
		inToks   int
		outToks  int
		err      error
	}

	resChan := make(chan qResult, len(vQuestions))
	var wg sync.WaitGroup

	for i, q := range vQuestions {
		wg.Add(1)
		// Distribute across slot 0, 1, 2
		slotID := i % 3
		go func(vq validatedQ, slot int) {
			defer wg.Done()
			t0 := time.Now()

			// 1. Handle noul questions
			if vq.qType == "noul" {
				lowerInstr := strings.ToLower(vq.instr)
				isSecretQuery := strings.Contains(lowerInstr, "secret") ||
					strings.Contains(lowerInstr, "token") ||
					strings.Contains(lowerInstr, "key") ||
					strings.Contains(lowerInstr, "leak") ||
					strings.Contains(lowerInstr, "credential")

				if isSecretQuery && reSecretTokens.MatchString(stateStr) {
					matched := reSecretTokens.FindString(stateStr)
					masked := matched
					if len(matched) > 8 {
						masked = matched[:8] + "..."
					}
					s.emit(RequestEvent{
						Task:       "SystemOne (noul)",
						SlotID:     slot,
						LatencyMs:  0,
						TTFTMs:     0,
						TokPerSec:  0,
						PromptToks: 0,
						OutToks:    0,
						Summary:    fmt.Sprintf("%s: noul=0.99 (secret shield: %s) [FAST-PATH]", vq.id, masked),
					})
					resChan <- qResult{
						id: vq.id,
						answer: map[string]any{
							"type": "noul",
							"noul": 0.99,
						},
						inToks:  0,
						outToks: 0,
					}
					return
				}

				decision, reason, inN, outN, latMs, err := s.Llama.DecideNoulCoT(vq.instr, vq.rawCrit, stateStr, slot)
				if err != nil {
					resChan <- qResult{id: vq.id, err: err}
					return
				}

				noulVal := 0.05
				if strings.EqualFold(decision, "true") || strings.EqualFold(decision, "yes") {
					noulVal = 0.95
				}

				tokPerSec := 0.0
				if latMs > 0 && outN > 0 {
					tokPerSec = float64(outN) / (float64(latMs) / 1000.0)
				}
				s.emit(RequestEvent{
					Task:       "SystemOne (noul)",
					SlotID:     slot,
					LatencyMs:  latMs,
					TTFTMs:     latMs,
					TokPerSec:  tokPerSec,
					PromptToks: inN,
					OutToks:    outN,
					Summary:    fmt.Sprintf("%s: noul=%.2f reason=%s", vq.id, noulVal, truncate(reason, 40)),
				})

				resChan <- qResult{
					id: vq.id,
					answer: map[string]any{
						"type": "noul",
						"noul": noulVal,
					},
					inToks:  inN,
					outToks: outN,
				}
				return
			}

			// 2. Handle choice and score questions
			promptText := vq.instr
			if vq.rawCrit != "" {
				promptText = promptText + "\nCriteria:\n" + vq.rawCrit
			}

			choice, dist, _, _, inN, outN, err := s.Llama.ChatDecideSlot(promptText, vq.options, stateStr, slot)
			if err != nil {
				resChan <- qResult{id: vq.id, err: err}
				return
			}

			ans := map[string]any{"type": vq.qType}

			switch vq.qType {
			case "choice":
				conf := dist[choice]
				conf = math.Round(conf*100) / 100
				probs := make(map[string]float64, len(vq.options))
				for _, o := range vq.options {
					probs[o] = math.Round(dist[o]*100) / 100
				}
				ans["choice"] = choice
				ans["confidence"] = conf
				ans["probabilities"] = probs

			case "score":
				probs := make(map[string]float64, len(vq.options))
				var expectedScore float64
				var maxP float64
				for idx := range vq.options {
					key := fmt.Sprintf("%d", idx)
					p := dist[key]
					probs[key] = math.Round(p*100) / 100
					expectedScore += float64(idx) * p
					if p > maxP {
						maxP = p
					}
				}
				ans["score"] = math.Round(expectedScore*10) / 10
				ans["confidence"] = math.Round(maxP*100) / 100
				ans["legend"] = vq.legend
				ans["probabilities"] = probs
			}

			latMs := time.Since(t0).Milliseconds()
			var summaryStr string
			switch vq.qType {
			case "choice":
				summaryStr = fmt.Sprintf("choice=%v (%.2f)", ans["choice"], ans["confidence"])
			case "score":
				summaryStr = fmt.Sprintf("score=%.1f", ans["score"])
			}
			tokPerSec := 0.0
			if latMs > 0 && outN > 0 {
				tokPerSec = float64(outN) / (float64(latMs) / 1000.0)
			}
			s.emit(RequestEvent{
				Task:       "SystemOne (" + vq.qType + ")",
				SlotID:     slot,
				LatencyMs:  latMs,
				TTFTMs:     latMs,
				TokPerSec:  tokPerSec,
				PromptToks: inN,
				OutToks:    outN,
				Summary:    fmt.Sprintf("%s: %s", vq.id, summaryStr),
			})

			resChan <- qResult{
				id:      vq.id,
				answer:  ans,
				inToks:  inN,
				outToks: outN,
			}
		}(q, slotID)
	}

	wg.Wait()
	close(resChan)

	answers := make(map[string]any, len(vQuestions))
	var totalInToks, totalOutToks int
	for res := range resChan {
		if res.err != nil {
			writeJSON(w, 529, map[string]string{"detail": "Service overloaded: " + res.err.Error()})
			return
		}
		answers[res.id] = res.answer
		totalInToks += res.inToks
		totalOutToks += res.outToks
	}

	writeJSON(w, 200, map[string]any{
		"model":   schema.ResolvedModel,
		"answers": answers,
		"usage": map[string]any{
			"input_tokens":  totalInToks,
			"output_tokens": totalOutToks,
		},
	})
}
