package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"embed"
	"log"
	"os"

	"local-jev/internal/schema"
)

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
	Llama   *Llama
	http    *http.Server
	mu      sync.Mutex
	OnEvent func(ev RequestEvent)
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
	return &Server{Llama: NewLlama(llamaRoot)}
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

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	id, n, err := s.Llama.ModelsOK()
	if err != nil {
		writeJSON(w, 200, map[string]any{"ok": false, "llm_ok": false, "llm": s.Llama.Root, "n_slots": n, "error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"ok": true, "llm_ok": true, "llm": s.Llama.Root, "model": id, "n_slots": n,
		"slots": map[string]int{"decision": schema.SlotDecision, "extract": schema.SlotExtract},
		"warn_parallel": n < 2,
	})
}

func (s *Server) schema(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"stop": map[string]any{"type": "boolean", "from": "Yes/No", "slot": schema.SlotDecision},
		"extract": map[string]any{"schema": schema.ExtractSchema(), "slot": schema.SlotExtract},
		"scan": map[string]any{"type": "string", "backend": "regex"},
		"systemone": map[string]any{"path": "/v1/systemone", "types": []string{"noul", "choice"}, "slot": schema.SlotDecision},
	})
}

func (s *Server) stop(w http.ResponseWriter, r *http.Request) {
	log, err := readLog(r)
	if err != nil {
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}
	t0 := time.Now()
	result, _, _, pn, cn, err := s.Llama.ChatDecide(
		"Has the agent task fully completed so the loop should stop?",
		[]string{"Yes", "No"},
		log,
	)
	if err != nil {
		writeJSON(w, 502, map[string]string{"detail": err.Error()})
		return
	}
	latencyMs := time.Since(t0).Milliseconds()
	ttftMs := latencyMs
	toks := 0.0
	if cn > 0 && latencyMs > 0 {
		toks = float64(cn) / (float64(latencyMs) / 1000.0)
	}
	s.emit(RequestEvent{
		Task:       "Task A (Stop)",
		SlotID:     0,
		LatencyMs:  latencyMs,
		TTFTMs:     ttftMs,
		TokPerSec:  toks,
		PromptToks: pn,
		OutToks:    cn,
		Summary:    fmt.Sprintf("stop=%v (%s)", strings.EqualFold(result, "Yes"), result),
	})
	writeJSON(w, 200, map[string]any{
		"stop": strings.EqualFold(result, "Yes"),
		"raw":  result,
		"metrics": map[string]any{
			"ttft_s": time.Since(t0).Seconds(), "decode_s": 0.0, "total_s": time.Since(t0).Seconds(),
			"prompt_tokens": pn, "completion_tokens": cn, "tok_s": 0.0,
		},
	})
}

func (s *Server) extract(w http.ResponseWriter, r *http.Request) {
	logText, err := readLog(r)
	if err != nil {
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}

	userMsg := buildExtractUserPrompt(logText)

	if os.Getenv("JEV_DEBUG") != "" {
		log.Printf("[extract] user len=%d", len(userMsg))
	}

	t0 := time.Now()
	req := map[string]any{
		"model":        "local",
		"temperature":  0.0,
		"max_tokens":   80,
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
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end <= start {
		writeJSON(w, 422, map[string]string{"detail": "JEV did not return JSON: " + text})
		return
	}
	raw := text[start : end+1]
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
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
		SlotID:     1,
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
	log, err := readLog(r)
	if err != nil {
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}
	t0 := time.Now()
	found := schema.ErrorLine.FindAllString(log, -1)
	line := ""
	if len(found) > 0 {
		line = strings.TrimSpace(found[len(found)-1])
	}
	dt := time.Since(t0).Seconds()
	s.emit(RequestEvent{
		Task:       "Task C (Scan)",
		SlotID:     -1,
		LatencyMs:  time.Since(t0).Milliseconds(),
		TTFTMs:     0,
		TokPerSec:  0,
		Summary:    fmt.Sprintf("needle=%s", line),
	})
	writeJSON(w, 200, map[string]any{
		"line": line,
		"metrics": map[string]any{
			"ttft_s": dt, "decode_s": 0, "total_s": dt,
			"prompt_tokens": 0, "completion_tokens": 0, "tok_s": 0,
		},
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

func (s *Server) systemone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type     string   `json:"type"`
		Question string   `json:"question"`
		Options  []string `json:"options"`
		Context  string   `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, 422, map[string]string{"detail": err.Error()})
		return
	}
	opts := body.Options
	if len(opts) == 0 {
		if body.Type == "noul" {
			opts = []string{"true", "false"}
		} else {
			writeJSON(w, 422, map[string]string{"detail": "choice requires options"})
			return
		}
	}
	t0 := time.Now()
	result, dist, _, _, _, err := s.Llama.ChatDecide(body.Question, opts, body.Context)
	if err != nil {
		writeJSON(w, 502, map[string]string{"detail": err.Error()})
		return
	}
	p := dist[result]
	latMs := time.Since(t0).Milliseconds()
	s.emit(RequestEvent{
		Task:      "SystemOne (" + body.Type + ")",
		SlotID:    0,
		LatencyMs: latMs,
		TTFTMs:    latMs,
		Summary:   fmt.Sprintf("%s (p=%.2f)", result, p),
	})
	writeJSON(w, 200, map[string]any{
		"type":         body.Type,
		"result":       result,
		"p":            p,
		"distribution": dist,
		"latency_ms":   float64(latMs),
	})
}
