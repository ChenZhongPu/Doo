package export

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"doo/internal/adapter"
)

type Snapshot struct {
	Provider adapter.Provider `json:"provider"`
	Target   adapter.Target   `json:"target"`
}

func isStandardHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "content-type", "accept", "authorization", "x-api-key", "anthropic-version":
		return true
	default:
		return false
	}
}

// ToCurl generates a standalone curl command reproducing the call.
func ToCurl(snapshotJSON string, requestJSON string) (string, error) {
	var snap Snapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snap); err != nil {
		return "", fmt.Errorf("unmarshal target snapshot: %w", err)
	}

	p := snap.Provider
	var targetURL string
	var authHeader string

	keyPlaceholder := "$API_KEY"
	if strings.HasPrefix(p.APIKeyRef, "env:") {
		keyPlaceholder = "$" + strings.TrimPrefix(p.APIKeyRef, "env:")
	}

	var headers [][2]string
	headers = append(headers, [2]string{"Content-Type", "application/json"})

	switch p.Protocol {
	case "openai_chat":
		targetURL = strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
		authHeader = fmt.Sprintf("Authorization: Bearer %s", keyPlaceholder)
		headers = append(headers, [2]string{"Authorization", fmt.Sprintf("Bearer %s", keyPlaceholder)})
	case "anthropic":
		targetURL = strings.TrimRight(p.BaseURL, "/") + "/messages"
		headers = append(headers, [2]string{"x-api-key", keyPlaceholder})
		headers = append(headers, [2]string{"anthropic-version", "2023-06-01"})
	default:
		return "", fmt.Errorf("unknown protocol: %s", p.Protocol)
	}

	// Add custom provider headers in sorted order for determinism
	var customKeys []string
	for k := range p.Headers {
		if !isStandardHeader(k) {
			customKeys = append(customKeys, k)
		}
	}
	sort.Strings(customKeys)
	for _, k := range customKeys {
		headers = append(headers, [2]string{k, p.Headers[k]})
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("curl -N -X POST %q", targetURL))
	for _, h := range headers {
		b.WriteString(fmt.Sprintf(" \\\n  -H %q", fmt.Sprintf("%s: %s", h[0], h[1])))
	}
	b.WriteString(" \\\n  --data-binary @- <<'EOF'\n")
	b.WriteString(strings.TrimRight(requestJSON, "\n"))
	b.WriteString("\nEOF\n")

	_ = authHeader
	return b.String(), nil
}

// ToPython generates an httpx-based Python script reproducing the call.
func ToPython(snapshotJSON string, requestJSON string) (string, error) {
	var snap Snapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snap); err != nil {
		return "", fmt.Errorf("unmarshal target snapshot: %w", err)
	}

	p := snap.Provider
	var targetURL string

	envVar := "API_KEY"
	if strings.HasPrefix(p.APIKeyRef, "env:") {
		envVar = strings.TrimPrefix(p.APIKeyRef, "env:")
	}

	switch p.Protocol {
	case "openai_chat":
		targetURL = strings.TrimRight(p.BaseURL, "/") + "/chat/completions"
	case "anthropic":
		targetURL = strings.TrimRight(p.BaseURL, "/") + "/messages"
	default:
		return "", fmt.Errorf("unknown protocol: %s", p.Protocol)
	}

	var b strings.Builder
	b.WriteString("import json\nimport os\nimport httpx\n\n")
	b.WriteString(fmt.Sprintf("api_key = os.environ.get(%q, \"\")\n", envVar))
	b.WriteString(fmt.Sprintf("url = %q\n", targetURL))
	b.WriteString("headers = {\n")
	b.WriteString("    \"Content-Type\": \"application/json\",\n")

	switch p.Protocol {
	case "openai_chat":
		b.WriteString("    \"Authorization\": f\"Bearer {api_key}\",\n")
	case "anthropic":
		b.WriteString("    \"x-api-key\": api_key,\n")
		b.WriteString("    \"anthropic-version\": \"2023-06-01\",\n")
	}

	var customKeys []string
	for k := range p.Headers {
		if !isStandardHeader(k) {
			customKeys = append(customKeys, k)
		}
	}
	sort.Strings(customKeys)
	for _, k := range customKeys {
		b.WriteString(fmt.Sprintf("    %q: %q,\n", k, p.Headers[k]))
	}
	b.WriteString("}\n\n")

	// Raw body string
	// Escape triple quotes in requestJSON if any
	sanitizedJSON := strings.ReplaceAll(requestJSON, `"""`, `\"\"\"`)
	b.WriteString("body_raw = \"\"\"" + sanitizedJSON + "\"\"\"\n")
	b.WriteString("body = json.loads(body_raw)\n\n")

	b.WriteString("# Streaming request (as executed in doo)\n")
	b.WriteString("with httpx.Client(timeout=300.0) as client:\n")
	b.WriteString("    with client.stream(\"POST\", url, headers=headers, json=body) as response:\n")
	b.WriteString("        for line in response.iter_lines():\n")
	b.WriteString("            if line.startswith(\"data: \"):\n")
	b.WriteString("                print(line[6:])\n\n")

	b.WriteString("# Non-streaming variant:\n")
	b.WriteString("# To run without streaming, delete or set 'stream' to False:\n")
	b.WriteString("# body.pop(\"stream\", None)\n")
	b.WriteString("# resp = httpx.post(url, headers=headers, json=body, timeout=300.0)\n")
	b.WriteString("# data = resp.json()\n")
	b.WriteString("# print(\"Usage:\", data.get(\"usage\"))\n")

	return b.String(), nil
}

var openAIKnownParams = map[string]bool{
	"messages":              true,
	"model":                 true,
	"frequency_penalty":     true,
	"logit_bias":            true,
	"logprobs":              true,
	"top_logprobs":          true,
	"max_tokens":            true,
	"max_completion_tokens": true,
	"n":                     true,
	"presence_penalty":      true,
	"response_format":       true,
	"seed":                  true,
	"service_tier":          true,
	"stop":                  true,
	"stream":                true,
	"stream_options":        true,
	"temperature":           true,
	"top_p":                 true,
	"tools":                 true,
	"tool_choice":           true,
	"parallel_tool_calls":   true,
	"user":                  true,
	"reasoning_effort":      true,
	"audio":                 true,
	"modalities":            true,
	"prediction":            true,
	"store":                 true,
	"metadata":              true,
}

var anthropicKnownParams = map[string]bool{
	"model":          true,
	"messages":       true,
	"max_tokens":     true,
	"system":         true,
	"temperature":    true,
	"top_p":          true,
	"top_k":          true,
	"stop_sequences": true,
	"thinking":       true,
	"tools":          true,
	"tool_choice":    true,
	"metadata":       true,
	"stream":         true,
}

func toPythonLiteral(v any, indent int) string {
	switch val := v.(type) {
	case nil:
		return "None"
	case bool:
		if val {
			return "True"
		}
		return "False"
	case int:
		return strconv.Itoa(val)
	case int64:
		return strconv.FormatInt(val, 10)
	case json.Number:
		return val.String()
	case float64:
		if val == float64(int64(val)) {
			return strconv.FormatInt(int64(val), 10)
		}
		return strconv.FormatFloat(val, 'f', -1, 64)
	case string:
		return strconv.Quote(val)
	case []any:
		if len(val) == 0 {
			return "[]"
		}
		indentStr := strings.Repeat("    ", indent)
		itemIndentStr := strings.Repeat("    ", indent+1)
		var sb strings.Builder
		sb.WriteString("[\n")
		for _, item := range val {
			sb.WriteString(itemIndentStr)
			sb.WriteString(toPythonLiteral(item, indent+1))
			sb.WriteString(",\n")
		}
		sb.WriteString(indentStr)
		sb.WriteString("]")
		return sb.String()
	case map[string]any:
		if len(val) == 0 {
			return "{}"
		}
		indentStr := strings.Repeat("    ", indent)
		itemIndentStr := strings.Repeat("    ", indent+1)

		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i] == "role" && keys[j] == "content" {
				return true
			}
			if keys[i] == "content" && keys[j] == "role" {
				return false
			}
			return keys[i] < keys[j]
		})

		var sb strings.Builder
		sb.WriteString("{\n")
		for _, k := range keys {
			sb.WriteString(itemIndentStr)
			sb.WriteString(strconv.Quote(k))
			sb.WriteString(": ")
			sb.WriteString(toPythonLiteral(val[k], indent+1))
			sb.WriteString(",\n")
		}
		sb.WriteString(indentStr)
		sb.WriteString("}")
		return sb.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func writeDefaultHeaders(b *strings.Builder, headers map[string]string) {
	if len(headers) == 0 {
		return
	}
	var customKeys []string
	for k := range headers {
		if !isStandardHeader(k) {
			customKeys = append(customKeys, k)
		}
	}
	if len(customKeys) == 0 {
		return
	}
	sort.Strings(customKeys)
	b.WriteString("    default_headers={\n")
	for _, k := range customKeys {
		b.WriteString(fmt.Sprintf("        %q: %q,\n", k, headers[k]))
	}
	b.WriteString("    },\n")
}

// ToPythonSDK generates an OpenAI or Anthropic SDK-based Python script reproducing the call.
func ToPythonSDK(snapshotJSON string, requestJSON string) (string, error) {
	var snap Snapshot
	if err := json.Unmarshal([]byte(snapshotJSON), &snap); err != nil {
		return "", fmt.Errorf("unmarshal target snapshot: %w", err)
	}

	p := snap.Provider
	if p.Protocol != "openai_chat" && p.Protocol != "anthropic" {
		return "", fmt.Errorf("unknown protocol: %s", p.Protocol)
	}

	var reqBody map[string]any
	if err := json.Unmarshal([]byte(requestJSON), &reqBody); err != nil {
		return "", fmt.Errorf("unmarshal request json: %w", err)
	}

	isStream := true
	if s, ok := reqBody["stream"].(bool); ok {
		isStream = s
	}

	baseURL := strings.TrimRight(p.BaseURL, "/")

	var b strings.Builder
	b.WriteString("import os\n")

	switch p.Protocol {
	case "openai_chat":
		b.WriteString("from openai import OpenAI\n\n")

		envVar := "OPENAI_API_KEY"
		if strings.HasPrefix(p.APIKeyRef, "env:") {
			envVar = strings.TrimPrefix(p.APIKeyRef, "env:")
		}

		b.WriteString("client = OpenAI(\n")
		b.WriteString(fmt.Sprintf("    api_key=os.environ.get(%q, \"\"),\n", envVar))
		if baseURL != "" {
			b.WriteString(fmt.Sprintf("    base_url=%q,\n", baseURL))
		}
		writeDefaultHeaders(&b, p.Headers)
		b.WriteString(")\n\n")

		// Separate known params and extra_body
		knownParams := make(map[string]any)
		extraBody := make(map[string]any)
		for k, v := range reqBody {
			if openAIKnownParams[k] {
				knownParams[k] = v
			} else {
				extraBody[k] = v
			}
		}

		buildOpenAICall := func(streamVal bool) string {
			var sb strings.Builder
			sb.WriteString("client.chat.completions.create(\n")

			if model, ok := knownParams["model"]; ok {
				sb.WriteString(fmt.Sprintf("    model=%s,\n", toPythonLiteral(model, 1)))
			}
			if messages, ok := knownParams["messages"]; ok {
				sb.WriteString(fmt.Sprintf("    messages=%s,\n", toPythonLiteral(messages, 1)))
			}

			var otherKeys []string
			for k := range knownParams {
				if k != "model" && k != "messages" && k != "stream" {
					otherKeys = append(otherKeys, k)
				}
			}
			sort.Strings(otherKeys)
			for _, k := range otherKeys {
				sb.WriteString(fmt.Sprintf("    %s=%s,\n", k, toPythonLiteral(knownParams[k], 1)))
			}

			sb.WriteString(fmt.Sprintf("    stream=%s,\n", toPythonLiteral(streamVal, 1)))

			if len(extraBody) > 0 {
				sb.WriteString(fmt.Sprintf("    extra_body=%s,\n", toPythonLiteral(extraBody, 1)))
			}

			sb.WriteString(")")
			return sb.String()
		}

		if isStream {
			b.WriteString("# Streaming request (as executed in doo)\n")
			b.WriteString("response = ")
			b.WriteString(buildOpenAICall(true))
			b.WriteString("\n\n")
			b.WriteString("for chunk in response:\n")
			b.WriteString("    if chunk.choices and chunk.choices[0].delta.content:\n")
			b.WriteString("        print(chunk.choices[0].delta.content, end=\"\", flush=True)\n")
			b.WriteString("print()\n\n")

			b.WriteString("# Non-streaming variant:\n")
			b.WriteString("# response = ")
			nonStreamingCall := buildOpenAICall(false)
			lines := strings.Split(nonStreamingCall, "\n")
			for i, line := range lines {
				if i > 0 {
					b.WriteString("# ")
				}
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("# print(response.choices[0].message.content)\n")
		} else {
			b.WriteString("# Non-streaming request (as executed in doo)\n")
			b.WriteString("response = ")
			b.WriteString(buildOpenAICall(false))
			b.WriteString("\n\n")
			b.WriteString("print(response.choices[0].message.content)\n")
		}

	case "anthropic":
		b.WriteString("from anthropic import Anthropic\n\n")

		envVar := "ANTHROPIC_API_KEY"
		if strings.HasPrefix(p.APIKeyRef, "env:") {
			envVar = strings.TrimPrefix(p.APIKeyRef, "env:")
		}

		b.WriteString("client = Anthropic(\n")
		b.WriteString(fmt.Sprintf("    api_key=os.environ.get(%q, \"\"),\n", envVar))
		if baseURL != "" {
			b.WriteString(fmt.Sprintf("    base_url=%q,\n", baseURL))
		}
		writeDefaultHeaders(&b, p.Headers)
		b.WriteString(")\n\n")

		knownParams := make(map[string]any)
		extraBody := make(map[string]any)
		for k, v := range reqBody {
			if anthropicKnownParams[k] {
				knownParams[k] = v
			} else {
				extraBody[k] = v
			}
		}

		buildAnthropicKWArgs := func(indent int, includeStream bool) string {
			var sb strings.Builder
			indentStr := strings.Repeat("    ", indent)

			if model, ok := knownParams["model"]; ok {
				sb.WriteString(fmt.Sprintf("%smodel=%s,\n", indentStr, toPythonLiteral(model, indent)))
			}
			if maxTokens, ok := knownParams["max_tokens"]; ok {
				sb.WriteString(fmt.Sprintf("%smax_tokens=%s,\n", indentStr, toPythonLiteral(maxTokens, indent)))
			}
			if messages, ok := knownParams["messages"]; ok {
				sb.WriteString(fmt.Sprintf("%smessages=%s,\n", indentStr, toPythonLiteral(messages, indent)))
			}

			var otherKeys []string
			for k := range knownParams {
				if k != "model" && k != "max_tokens" && k != "messages" && k != "stream" {
					otherKeys = append(otherKeys, k)
				}
			}
			sort.Strings(otherKeys)
			for _, k := range otherKeys {
				sb.WriteString(fmt.Sprintf("%s%s=%s,\n", indentStr, k, toPythonLiteral(knownParams[k], indent)))
			}

			if includeStream {
				sb.WriteString(fmt.Sprintf("%sstream=True,\n", indentStr))
			}

			if len(extraBody) > 0 {
				sb.WriteString(fmt.Sprintf("%sextra_body=%s,\n", indentStr, toPythonLiteral(extraBody, indent)))
			}

			return sb.String()
		}

		if isStream {
			b.WriteString("# Streaming request (as executed in doo)\n")
			b.WriteString("with client.messages.stream(\n")
			b.WriteString(buildAnthropicKWArgs(1, false))
			b.WriteString(") as stream:\n")
			b.WriteString("    for text in stream.text_stream:\n")
			b.WriteString("        print(text, end=\"\", flush=True)\n")
			b.WriteString("print()\n\n")

			b.WriteString("# Non-streaming variant:\n")
			b.WriteString("# response = client.messages.create(\n")
			for _, line := range strings.Split(strings.TrimRight(buildAnthropicKWArgs(1, false), "\n"), "\n") {
				b.WriteString("# ")
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("# )\n")
			b.WriteString("# for block in response.content:\n")
			b.WriteString("#     if block.type == \"text\":\n")
			b.WriteString("#         print(block.text)\n")
		} else {
			b.WriteString("# Non-streaming request (as executed in doo)\n")
			b.WriteString("response = client.messages.create(\n")
			b.WriteString(buildAnthropicKWArgs(1, false))
			b.WriteString(")\n\n")
			b.WriteString("for block in response.content:\n")
			b.WriteString("    if block.type == \"text\":\n")
			b.WriteString("        print(block.text)\n")
		}
	}

	return b.String(), nil
}
