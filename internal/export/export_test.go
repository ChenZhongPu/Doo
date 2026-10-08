package export

import (
	"strings"
	"testing"
)

func TestToCurl_OpenAI(t *testing.T) {
	snapshot := `{
		"provider": {
			"protocol": "openai_chat",
			"base_url": "https://api.openai.com/v1",
			"api_key_ref": "env:OPENAI_API_KEY",
			"headers": {"HTTP-Referer": "https://example.com"}
		},
		"target": {
			"label": "gpt-4o",
			"model_name": "gpt-4o"
		}
	}`
	requestJSON := `{"messages":[{"content":"hi","role":"user"}],"model":"gpt-4o","stream":true}`

	curlCmd, err := ToCurl(snapshot, requestJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(curlCmd, `curl -N -X POST "https://api.openai.com/v1/chat/completions"`) {
		t.Errorf("wrong curl target, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, `-H "Authorization: Bearer $OPENAI_API_KEY"`) {
		t.Errorf("missing Authorization header, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, `-H "HTTP-Referer: https://example.com"`) {
		t.Errorf("missing custom header, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, `--data-binary @- <<'EOF'`) {
		t.Errorf("missing heredoc, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, requestJSON) {
		t.Errorf("missing request body, got: %s", curlCmd)
	}
}

func TestToCurl_Anthropic_EncKey(t *testing.T) {
	snapshot := `{
		"provider": {
			"protocol": "anthropic",
			"base_url": "https://api.anthropic.com/v1",
			"api_key_ref": "enc:base64encryptedkey",
			"headers": {}
		},
		"target": {
			"label": "claude-3-5",
			"model_name": "claude-3-5-sonnet-20241022"
		}
	}`
	requestJSON := `{"max_tokens":1024,"messages":[{"content":"hello","role":"user"}],"model":"claude-3-5-sonnet-20241022","stream":true}`

	curlCmd, err := ToCurl(snapshot, requestJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(curlCmd, `curl -N -X POST "https://api.anthropic.com/v1/messages"`) {
		t.Errorf("wrong curl target, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, `-H "x-api-key: $API_KEY"`) {
		t.Errorf("missing x-api-key header, got: %s", curlCmd)
	}
	if !strings.Contains(curlCmd, `-H "anthropic-version: 2023-06-01"`) {
		t.Errorf("missing anthropic-version header, got: %s", curlCmd)
	}
}

func TestToPython_OpenAI(t *testing.T) {
	snapshot := `{
		"provider": {
			"protocol": "openai_chat",
			"base_url": "https://api.openai.com/v1",
			"api_key_ref": "env:MY_KEY",
			"headers": {}
		},
		"target": {
			"label": "gpt-4o",
			"model_name": "gpt-4o"
		}
	}`
	requestJSON := `{"messages":[{"content":"hi","role":"user"}],"model":"gpt-4o","stream":true}`

	pyCode, err := ToPython(snapshot, requestJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(pyCode, `os.environ.get("MY_KEY", "")`) {
		t.Errorf("missing env var retrieval, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `url = "https://api.openai.com/v1/chat/completions"`) {
		t.Errorf("missing url, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `"Authorization": f"Bearer {api_key}"`) {
		t.Errorf("missing auth header, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `body = json.loads(body_raw)`) {
		t.Errorf("missing json.loads, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `client.stream("POST", url, headers=headers, json=body)`) {
		t.Errorf("missing streaming call, got: %s", pyCode)
	}
}
