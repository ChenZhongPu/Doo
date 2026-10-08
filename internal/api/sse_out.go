package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type SSERunCallInfo struct {
	CallID   int64  `json:"call_id"`
	TargetID int64  `json:"target_id"`
	Label    string `json:"label"`
}

type SSERunEvent struct {
	Type   string           `json:"type"` // "run"
	RunID  int64            `json:"run_id"`
	Calls  []SSERunCallInfo `json:"calls"`
}

type SSEDeltaEvent struct {
	Type   string `json:"type"` // "delta"
	CallID int64  `json:"call_id"`
	Kind   string `json:"kind"` // "content" | "reasoning"
	Text   string `json:"text"`
}

type SSEDoneEvent struct {
	Type          string          `json:"type"` // "done"
	CallID        int64           `json:"call_id"`
	Status        string          `json:"status"` // "done" | "canceled"
	FinishReason  string          `json:"finish_reason,omitempty"`
	Usage         json.RawMessage `json:"usage,omitempty"`
	Metrics       any             `json:"metrics,omitempty"`
	ResponseModel string          `json:"response_model,omitempty"`
	HTTPStatus    int             `json:"http_status,omitempty"`
	Output        string          `json:"output,omitempty"`
}

type SSEErrorEvent struct {
	Type       string `json:"type"` // "error"
	CallID     int64  `json:"call_id"`
	HTTPStatus int    `json:"http_status,omitempty"`
	Message    string `json:"message"`
	Metrics    any    `json:"metrics,omitempty"`
	Output     string `json:"output,omitempty"`
}

type SSEEndEvent struct {
	Type string `json:"type"` // "end"
}

type SSEWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
}

func NewSSEWriter(w http.ResponseWriter) (*SSEWriter, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("response writer does not support flushing")
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	return &SSEWriter{w: w, flusher: flusher}, nil
}

func (s *SSEWriter) Send(event any) error {
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := fmt.Fprintf(s.w, "data: %s\n\n", data); err != nil {
		return err
	}
	s.flusher.Flush()
	return nil
}
