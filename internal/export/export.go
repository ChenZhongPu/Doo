package export

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"doo/internal/adapter"
)

type Snapshot struct {
	Provider adapter.Provider `json:"provider"`
	Target   adapter.Target   `json:"target"`
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
		customKeys = append(customKeys, k)
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
		customKeys = append(customKeys, k)
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
