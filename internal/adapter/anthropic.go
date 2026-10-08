package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type AnthropicAdapter struct {
	Client *http.Client
	Clock  Clock
}

func (a *AnthropicAdapter) getClock() Clock {
	if a.Clock != nil {
		return a.Clock
	}
	return time.Now
}

func (a *AnthropicAdapter) getClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return &http.Client{}
}

func (a *AnthropicAdapter) Do(ctx context.Context, p Provider, apiKey string, body []byte, on func(Event)) *Result {
	clock := a.getClock()
	client := a.getClient()

	res := &Result{}
	t0 := clock()

	defer func() {
		tot := float64(clock().Sub(t0).Milliseconds())
		res.Metrics.TotalMs = &tot
	}()

	targetURL := strings.TrimRight(p.BaseURL, "/") + "/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		res.Status = "error"
		res.Err = fmt.Sprintf("create request: %v", err)
		return res
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("anthropic-version", "2023-06-01")
	if apiKey != "" {
		req.Header.Set("x-api-key", apiKey)
	}
	for k, v := range p.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			res.Status = "canceled"
			return res
		}
		res.Status = "error"
		res.Err = fmt.Sprintf("http request: %v", err)
		return res
	}
	defer resp.Body.Close()

	ttfb := float64(clock().Sub(t0).Milliseconds())
	res.Metrics.TTFBms = &ttfb
	res.HTTPStatus = resp.StatusCode

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		res.Status = "error"
		errRec := NewRawRecorder(MaxErrResponseBytes)
		_, _ = io.Copy(errRec, resp.Body)
		res.RespRaw = errRec.Bytes()
		res.RespTruncated = errRec.Truncated()

		var errObj struct {
			Error struct {
				Message string `json:"message"`
				Type    string `json:"type"`
			} `json:"error"`
		}
		if jsonErr := json.Unmarshal(res.RespRaw, &errObj); jsonErr == nil && errObj.Error.Message != "" {
			res.Err = errObj.Error.Message
		} else {
			res.Err = string(res.RespRaw)
		}
		return res
	}

	contentType := resp.Header.Get("Content-Type")
	isSSE := strings.Contains(contentType, "text/event-stream")

	rawRec := NewRawRecorder(MaxRawResponseBytes)
	var outputBuf, reasoningBuf strings.Builder
	var streamErr string
	mergedUsage := make(map[string]any)

	if isSSE {
		readErr := ReadSSE(resp.Body, rawRec, func(data []byte) error {
			var generic struct {
				Type string `json:"type"`
			}
			if err := json.Unmarshal(data, &generic); err != nil {
				return nil
			}

			switch generic.Type {
			case "message_start":
				var start struct {
					Message struct {
						ID    string `json:"id"`
						Model string `json:"model"`
						Usage struct {
							InputTokens int `json:"input_tokens"`
						} `json:"usage"`
					} `json:"message"`
				}
				if err := json.Unmarshal(data, &start); err == nil {
					if res.ProviderReqID == "" && start.Message.ID != "" {
						res.ProviderReqID = start.Message.ID
					}
					if res.ResponseModel == "" && start.Message.Model != "" {
						res.ResponseModel = start.Message.Model
					}
					res.PromptTokens = &start.Message.Usage.InputTokens

					var rawUsageMap struct {
						Message struct {
							Usage map[string]any `json:"usage"`
						} `json:"message"`
					}
					if json.Unmarshal(data, &rawUsageMap) == nil && rawUsageMap.Message.Usage != nil {
						for k, v := range rawUsageMap.Message.Usage {
							mergedUsage[k] = v
						}
					}
				}

			case "content_block_delta":
				var block struct {
					Delta struct {
						Type     string `json:"type"`
						Text     string `json:"text"`
						Thinking string `json:"thinking"`
					} `json:"delta"`
				}
				if err := json.Unmarshal(data, &block); err == nil {
					if block.Delta.Type == "text_delta" && block.Delta.Text != "" {
						diff := float64(clock().Sub(t0).Milliseconds())
						if res.Metrics.FirstTokenMs == nil {
							res.Metrics.FirstTokenMs = &diff
						}
						if res.Metrics.FirstContentMs == nil {
							res.Metrics.FirstContentMs = &diff
						}
						outputBuf.WriteString(block.Delta.Text)
						if on != nil {
							on(Event{Kind: "content", Text: block.Delta.Text})
						}
					} else if block.Delta.Type == "thinking_delta" && block.Delta.Thinking != "" {
						diff := float64(clock().Sub(t0).Milliseconds())
						if res.Metrics.FirstTokenMs == nil {
							res.Metrics.FirstTokenMs = &diff
						}
						reasoningBuf.WriteString(block.Delta.Thinking)
						if on != nil {
							on(Event{Kind: "reasoning", Text: block.Delta.Thinking})
						}
					}
				}

			case "message_delta":
				var msgDelta struct {
					Delta struct {
						StopReason string `json:"stop_reason"`
					} `json:"delta"`
					Usage struct {
						OutputTokens int `json:"output_tokens"`
					} `json:"usage"`
				}
				if err := json.Unmarshal(data, &msgDelta); err == nil {
					if msgDelta.Delta.StopReason != "" {
						res.FinishReason = msgDelta.Delta.StopReason
					}
					res.CompletionTokens = &msgDelta.Usage.OutputTokens

					var rawDeltaUsage struct {
						Usage map[string]any `json:"usage"`
					}
					if json.Unmarshal(data, &rawDeltaUsage) == nil && rawDeltaUsage.Usage != nil {
						for k, v := range rawDeltaUsage.Usage {
							mergedUsage[k] = v
						}
					}
				}

			case "error":
				var errEvent struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				if json.Unmarshal(data, &errEvent) == nil && errEvent.Error.Message != "" {
					streamErr = errEvent.Error.Message
				} else {
					streamErr = string(data)
				}
				return fmt.Errorf("anthropic error: %s", streamErr)

			default:
				// ping, content_block_start, content_block_stop, message_stop, etc. are ignored
			}

			return nil
		})

		res.RespRaw = rawRec.Bytes()
		res.RespTruncated = rawRec.Truncated()
		res.Output = outputBuf.String()
		res.Reasoning = reasoningBuf.String()

		if len(mergedUsage) > 0 {
			uBytes, _ := json.Marshal(mergedUsage)
			res.UsageJSON = uBytes
		}

		if ctx.Err() != nil {
			res.Status = "canceled"
			return res
		}

		if streamErr != "" {
			res.Status = "error"
			res.Err = streamErr
			return res
		}

		if readErr != nil {
			res.Status = "error"
			res.Err = fmt.Sprintf("stream interrupted: %v", readErr)
			return res
		}
	} else {
		// Non-streaming response handling
		_, _ = io.Copy(rawRec, resp.Body)
		res.RespRaw = rawRec.Bytes()
		res.RespTruncated = rawRec.Truncated()

		if ctx.Err() != nil {
			res.Status = "canceled"
			return res
		}

		var msg struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				Thinking string `json:"thinking"`
			} `json:"content"`
			StopReason *string `json:"stop_reason"`
			Usage      *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
			Error any `json:"error"`
		}

		if err := json.Unmarshal(res.RespRaw, &msg); err != nil {
			res.Status = "error"
			res.Err = fmt.Sprintf("unmarshal response: %v", err)
			return res
		}

		if msg.Error != nil {
			res.Status = "error"
			res.Err = fmt.Sprintf("anthropic error: %v", msg.Error)
			return res
		}

		if msg.Model != "" {
			res.ResponseModel = msg.Model
		}
		if msg.ID != "" {
			res.ProviderReqID = msg.ID
		}
		if msg.StopReason != nil {
			res.FinishReason = *msg.StopReason
		}

		diff := float64(clock().Sub(t0).Milliseconds())
		for _, item := range msg.Content {
			if item.Type == "thinking" && item.Thinking != "" {
				if res.Metrics.FirstTokenMs == nil {
					res.Metrics.FirstTokenMs = &diff
				}
				reasoningBuf.WriteString(item.Thinking)
				if on != nil {
					on(Event{Kind: "reasoning", Text: item.Thinking})
				}
			} else if (item.Type == "text" || item.Type == "") && item.Text != "" {
				if res.Metrics.FirstTokenMs == nil {
					res.Metrics.FirstTokenMs = &diff
				}
				if res.Metrics.FirstContentMs == nil {
					res.Metrics.FirstContentMs = &diff
				}
				outputBuf.WriteString(item.Text)
				if on != nil {
					on(Event{Kind: "content", Text: item.Text})
				}
			}
		}

		if msg.Usage != nil {
			res.PromptTokens = &msg.Usage.InputTokens
			res.CompletionTokens = &msg.Usage.OutputTokens
			uBytes, _ := json.Marshal(msg.Usage)
			res.UsageJSON = uBytes
		}

		res.Output = outputBuf.String()
		res.Reasoning = reasoningBuf.String()
	}

	res.Status = "done"
	return res
}
