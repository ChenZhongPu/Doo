package adapter

import (
	"bufio"
	"bytes"
	"io"
)

const (
	MaxRawResponseBytes = 4 * 1024 * 1024 // 4 MiB
	MaxErrResponseBytes = 64 * 1024       // 64 KiB
)

type RawRecorder struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func NewRawRecorder(limit int) *RawRecorder {
	return &RawRecorder{limit: limit}
}

func (r *RawRecorder) Write(p []byte) (n int, err error) {
	n = len(p)
	if r.buf.Len() < r.limit {
		available := r.limit - r.buf.Len()
		if len(p) <= available {
			r.buf.Write(p)
		} else {
			r.buf.Write(p[:available])
			r.truncated = true
		}
	} else {
		r.truncated = true
	}
	return n, nil
}

func (r *RawRecorder) Bytes() []byte {
	return r.buf.Bytes()
}

func (r *RawRecorder) Truncated() bool {
	return r.truncated
}

// ReadSSE reads SSE lines using bufio.Reader.ReadBytes('\n').
// It captures all bytes into recorder and emits data payload for each event.
func ReadSSE(reader io.Reader, recorder *RawRecorder, onData func(data []byte) error) error {
	br := bufio.NewReader(reader)
	var eventData []byte

	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if recorder != nil {
				_, _ = recorder.Write(line)
			}
			trimmed := bytes.TrimRight(line, "\r\n")
			if len(trimmed) == 0 {
				// Empty line: event boundary
				if len(eventData) > 0 {
					if errCallback := onData(eventData); errCallback != nil {
						return errCallback
					}
					eventData = nil
				}
			} else if bytes.HasPrefix(trimmed, []byte(":")) {
				// Comment line, ignore
			} else if bytes.HasPrefix(trimmed, []byte("data:")) {
				content := bytes.TrimPrefix(trimmed, []byte("data:"))
				content = bytes.TrimPrefix(content, []byte(" "))
				if len(eventData) > 0 {
					eventData = append(eventData, '\n')
				}
				eventData = append(eventData, content...)
			}
		}

		if err != nil {
			if err == io.EOF {
				if len(eventData) > 0 {
					_ = onData(eventData)
				}
				return nil
			}
			return err
		}
	}
}
