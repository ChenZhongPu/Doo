package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// DetermineMaxTokensField decides whether to send "max_tokens" or "max_completion_tokens".
// OpenAI reasoning models (o1, o3 series) require max_completion_tokens, while standard models use max_tokens.
func DetermineMaxTokensField(modelName string, explicitField string) string {
	if explicitField == "max_completion_tokens" {
		return "max_completion_tokens"
	}
	lower := strings.ToLower(strings.TrimSpace(modelName))
	if strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3") {
		return "max_completion_tokens"
	}
	if explicitField != "" {
		return explicitField
	}
	return "max_tokens"
}

// BuildBody constructs the exact JSON request body to be sent to the provider.
// It is the single source of truth for request body generation.
func BuildBody(p Provider, t Target, in Input) ([]byte, error) {
	base := make(map[string]any)
	base["model"] = t.ModelName

	// Only send stream if explicitly specified in Core
	if t.Core.Stream != nil {
		base["stream"] = *t.Core.Stream
	}

	// Step 2: messages mapping
	switch p.Protocol {
	case "openai_chat":
		var msgs []map[string]any
		if in.System != "" {
			msgs = append(msgs, map[string]any{
				"role":    "system",
				"content": in.System,
			})
		}
		for _, m := range in.Messages {
			content := m.Content
			if len(content) == 0 {
				content = json.RawMessage(`""`)
			}
			msgs = append(msgs, map[string]any{
				"role":    m.Role,
				"content": content,
			})
		}
		base["messages"] = msgs

		// Step 3: core params
		if t.Core.Temperature != nil {
			base["temperature"] = *t.Core.Temperature
		}
		if t.Core.TopP != nil {
			base["top_p"] = *t.Core.TopP
		}
		if t.Core.Seed != nil {
			base["seed"] = *t.Core.Seed
		}
		if len(t.Core.Stop) > 0 {
			base["stop"] = t.Core.Stop
		}
		if t.Core.MaxTokens != nil {
			field := DetermineMaxTokensField(t.ModelName, t.MaxTokensField)
			base[field] = *t.Core.MaxTokens
		}


	case "anthropic":
		if in.System != "" {
			base["system"] = in.System
		}
		var msgs []map[string]any
		for _, m := range in.Messages {
			content := m.Content
			if len(content) == 0 {
				content = json.RawMessage(`""`)
			}
			msgs = append(msgs, map[string]any{
				"role":    m.Role,
				"content": content,
			})
		}
		base["messages"] = msgs

		// Step 3: core params
		if t.Core.Temperature != nil {
			base["temperature"] = *t.Core.Temperature
		}
		if t.Core.TopP != nil {
			base["top_p"] = *t.Core.TopP
		}
		if len(t.Core.Stop) > 0 {
			base["stop_sequences"] = t.Core.Stop
		}
		if t.Core.MaxTokens == nil {
			return nil, errors.New("max_tokens is required for anthropic")
		}
		base["max_tokens"] = *t.Core.MaxTokens
		if t.Core.Seed != nil {
			return nil, errors.New("seed is not supported for anthropic")
		}

	default:
		return nil, fmt.Errorf("unknown protocol: %s", p.Protocol)
	}

	// Step 4: deepMerge extra
	if len(t.Extra) > 0 {
		deepMerge(base, t.Extra)
	}


	// Step 5: serialization
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(base); err != nil {
		return nil, fmt.Errorf("encode request body: %w", err)
	}

	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// deepMerge recursively merges src into dst.
// Objects are recursively merged; other types (including arrays) are replaced.
// Keys with nil values in src are deleted from dst.
func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if v == nil {
			delete(dst, k)
			continue
		}
		if srcMap, ok := v.(map[string]any); ok {
			if dstMap, ok2 := dst[k].(map[string]any); ok2 {
				deepMerge(dstMap, srcMap)
				continue
			}
		}
		dst[k] = v
	}
}
