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

type OpenAIChatAdapter struct {
	Client *http.Client
	Clock  Clock
}

func (a *OpenAIChatAdapter) getClock() Clock {
	if a.Clock != nil {
		return a.Clock
	}
	return time.Now
}

func (a *OpenAIChatAdapter) getClient() *http.Client {
	if a.Client != nil {
		return a.Client
	}
	return &http.Client{}
}

func (a *OpenAIChatAdapter) Do(ctx context.Context, p Provider, apiKey string, body []byte, on func(Event)) *Result {
	clock := a.getClock()
	client := a.getClient()

	res := &Result{}
	t0 := clock()

	defer func() {
		tot := float64(clock().Sub(t0).Milliseconds())
		res.Metrics.TotalMs = &tot
	}()

	targetURL := strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(body))
	if err != nil {
		res.Status = "error"
		res.Err = fmt.Sprintf("create request: %v", err)
		return res
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
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
			Error any `json:"error"`
		}
		if jsonErr := json.Unmarshal(res.RespRaw, &errObj); jsonErr == nil && errObj.Error != nil {
			switch e := errObj.Error.(type) {
			case string:
				res.Err = e
			case map[string]any:
				if msg, ok := e["message"].(string); ok && msg != "" {
					res.Err = msg
				} else {
					res.Err = string(res.RespRaw)
				}
			default:
				res.Err = string(res.RespRaw)
			}
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

	if isSSE {
		readErr := ReadSSE(resp.Body, rawRec, func(data []byte) error {
			if string(data) == "[DONE]" {
				return nil
			}

			var chunk struct {
				ID      string `json:"id"`
				Model   string `json:"model"`
				Choices []struct {
					Delta struct {
						Content          *string `json:"content"`
						ReasoningContent *string `json:"reasoning_content"`
						Reasoning        *string `json:"reasoning"`
					} `json:"delta"`
					FinishReason *string `json:"finish_reason"`
				} `json:"choices"`
				Usage *struct {
					PromptTokens            int `json:"prompt_tokens"`
					CompletionTokens        int `json:"completion_tokens"`
					CompletionTokensDetails *struct {
						ReasoningTokens int `json:"reasoning_tokens"`
					} `json:"completion_tokens_details"`
				} `json:"usage"`
				Error any `json:"error"`
			}

			if err := json.Unmarshal(data, &chunk); err != nil {
				return nil
			}

			if chunk.Error != nil {
				streamErr = fmt.Sprintf("upstream error: %v", chunk.Error)
				return fmt.Errorf("upstream error: %v", chunk.Error)
			}

			if res.ResponseModel == "" && chunk.Model != "" {
				res.ResponseModel = chunk.Model
			}
			if res.ProviderReqID == "" && chunk.ID != "" {
				res.ProviderReqID = chunk.ID
			}

			for _, ch := range chunk.Choices {
				if ch.FinishReason != nil && *ch.FinishReason != "" {
					res.FinishReason = *ch.FinishReason
				}

				reasoning := ""
				if ch.Delta.ReasoningContent != nil {
					reasoning = *ch.Delta.ReasoningContent
				} else if ch.Delta.Reasoning != nil {
					reasoning = *ch.Delta.Reasoning
				}

				if reasoning != "" {
					diff := float64(clock().Sub(t0).Milliseconds())
					if res.Metrics.FirstTokenMs == nil {
						res.Metrics.FirstTokenMs = &diff
					}
					reasoningBuf.WriteString(reasoning)
					if on != nil {
						on(Event{Kind: "reasoning", Text: reasoning})
					}
				}

				if ch.Delta.Content != nil && *ch.Delta.Content != "" {
					content := *ch.Delta.Content
					diff := float64(clock().Sub(t0).Milliseconds())
					if res.Metrics.FirstTokenMs == nil {
						res.Metrics.FirstTokenMs = &diff
					}
					if res.Metrics.FirstContentMs == nil {
						res.Metrics.FirstContentMs = &diff
					}
					outputBuf.WriteString(content)
					if on != nil {
						on(Event{Kind: "content", Text: content})
					}
				}
			}

			if chunk.Usage != nil {
				uBytes, _ := json.Marshal(chunk.Usage)
				res.UsageJSON = uBytes
				res.PromptTokens = &chunk.Usage.PromptTokens
				res.CompletionTokens = &chunk.Usage.CompletionTokens
				if chunk.Usage.CompletionTokensDetails != nil {
					res.ReasoningTokens = &chunk.Usage.CompletionTokensDetails.ReasoningTokens
				}
			}

			return nil
		})

		res.RespRaw = rawRec.Bytes()
		res.RespTruncated = rawRec.Truncated()
		res.Output = outputBuf.String()
		res.Reasoning = reasoningBuf.String()

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

		var nonStream struct {
			ID      string `json:"id"`
			Model   string `json:"model"`
			Choices []struct {
				Message struct {
					Content          *string `json:"content"`
					ReasoningContent *string `json:"reasoning_content"`
					Reasoning        *string `json:"reasoning"`
				} `json:"message"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens            int `json:"prompt_tokens"`
				CompletionTokens        int `json:"completion_tokens"`
				CompletionTokensDetails *struct {
					ReasoningTokens int `json:"reasoning_tokens"`
				} `json:"completion_tokens_details"`
			} `json:"usage"`
			Error any `json:"error"`
		}

		if err := json.Unmarshal(res.RespRaw, &nonStream); err != nil {
			res.Status = "error"
			res.Err = fmt.Sprintf("unmarshal response: %v", err)
			return res
		}

		if nonStream.Error != nil {
			res.Status = "error"
			res.Err = fmt.Sprintf("upstream error: %v", nonStream.Error)
			return res
		}

		if nonStream.Model != "" {
			res.ResponseModel = nonStream.Model
		}
		if nonStream.ID != "" {
			res.ProviderReqID = nonStream.ID
		}

		diff := float64(clock().Sub(t0).Milliseconds())
		for _, ch := range nonStream.Choices {
			if ch.FinishReason != nil && *ch.FinishReason != "" {
				res.FinishReason = *ch.FinishReason
			}

			reasoning := ""
			if ch.Message.ReasoningContent != nil {
				reasoning = *ch.Message.ReasoningContent
			} else if ch.Message.Reasoning != nil {
				reasoning = *ch.Message.Reasoning
			}

			if reasoning != "" {
				if res.Metrics.FirstTokenMs == nil {
					res.Metrics.FirstTokenMs = &diff
				}
				reasoningBuf.WriteString(reasoning)
				if on != nil {
					on(Event{Kind: "reasoning", Text: reasoning})
				}
			}

			if ch.Message.Content != nil && *ch.Message.Content != "" {
				content := *ch.Message.Content
				if res.Metrics.FirstTokenMs == nil {
					res.Metrics.FirstTokenMs = &diff
				}
				if res.Metrics.FirstContentMs == nil {
					res.Metrics.FirstContentMs = &diff
				}
				outputBuf.WriteString(content)
				if on != nil {
					on(Event{Kind: "content", Text: content})
				}
			}
		}

		if nonStream.Usage != nil {
			uBytes, _ := json.Marshal(nonStream.Usage)
			res.UsageJSON = uBytes
			res.PromptTokens = &nonStream.Usage.PromptTokens
			res.CompletionTokens = &nonStream.Usage.CompletionTokens
			if nonStream.Usage.CompletionTokensDetails != nil {
				res.ReasoningTokens = &nonStream.Usage.CompletionTokensDetails.ReasoningTokens
			}
		}

		res.Output = outputBuf.String()
		res.Reasoning = reasoningBuf.String()
	}

	res.Status = "done"
	return res
}
