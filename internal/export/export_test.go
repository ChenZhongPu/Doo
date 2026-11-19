package export

import (
	"os/exec"
	"strings"
	"testing"
)

func validatePythonAST(t *testing.T, code string) {
	cmd := exec.Command("python3", "-c", "import ast, sys; ast.parse(sys.stdin.read())")
	cmd.Stdin = strings.NewReader(code)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("Python AST validation failed: %v\nOutput: %s\nCode:\n%s", err, string(out), code)
	}
}

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
	validatePythonAST(t, pyCode)

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

func TestToPythonSDK_OpenAI(t *testing.T) {
	snapshot := `{
		"provider": {
			"protocol": "openai_chat",
			"base_url": "https://api.openai.com/v1",
			"api_key_ref": "env:MY_KEY",
			"headers": {"HTTP-Referer": "https://example.com"}
		},
		"target": {
			"label": "gpt-4o",
			"model_name": "gpt-4o"
		}
	}`
	requestJSON := `{"messages":[{"content":"hi","role":"user"}],"model":"gpt-4o","stream":true,"temperature":0.7,"custom_option":{"flag":true}}`

	pyCode, err := ToPythonSDK(snapshot, requestJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	validatePythonAST(t, pyCode)

	if !strings.Contains(pyCode, "from openai import OpenAI") {
		t.Errorf("missing OpenAI import, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `api_key=os.environ.get("MY_KEY", "")`) {
		t.Errorf("missing env var retrieval, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `base_url="https://api.openai.com/v1"`) {
		t.Errorf("missing base_url, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `"HTTP-Referer": "https://example.com"`) {
		t.Errorf("missing default_headers, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `model="gpt-4o"`) {
		t.Errorf("missing model param, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `temperature=0.7`) {
		t.Errorf("missing temperature, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `stream=True`) {
		t.Errorf("missing stream=True, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `extra_body={`) || !strings.Contains(pyCode, `"custom_option": {`) {
		t.Errorf("missing extra_body for unknown option, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "for chunk in response:") {
		t.Errorf("missing streaming loop, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "# Non-streaming variant:") {
		t.Errorf("missing non-streaming variant comment, got: %s", pyCode)
	}
}

func TestToPythonSDK_OpenAI_NonStreaming(t *testing.T) {
	snapshot := `{
		"provider": {
			"protocol": "openai_chat",
			"base_url": "https://api.openai.com/v1",
			"api_key_ref": "enc:secret"
		},
		"target": {
			"label": "gpt-4o",
			"model_name": "gpt-4o"
		}
	}`
	requestJSON := `{"messages":[{"content":"hi","role":"user"}],"model":"gpt-4o","stream":false}`

	pyCode, err := ToPythonSDK(snapshot, requestJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	validatePythonAST(t, pyCode)

	if !strings.Contains(pyCode, `api_key=os.environ.get("OPENAI_API_KEY", "")`) {
		t.Errorf("missing default OPENAI_API_KEY fallback, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `stream=False`) {
		t.Errorf("missing stream=False, got: %s", pyCode)
	}
	if strings.Contains(pyCode, "for chunk in response:") {
		t.Errorf("should not have streaming loop when stream is false, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "print(response.choices[0].message.content)") {
		t.Errorf("missing output print, got: %s", pyCode)
	}
}

func TestToPythonSDK_Anthropic_Streaming(t *testing.T) {
	snapshot := `{
		"provider": {
			"protocol": "anthropic",
			"base_url": "https://api.anthropic.com/v1",
			"api_key_ref": "env:ANTHROPIC_KEY"
		},
		"target": {
			"label": "claude",
			"model_name": "claude-3-5-sonnet-20241022"
		}
	}`
	requestJSON := `{"max_tokens":1024,"messages":[{"content":"hello","role":"user"}],"model":"claude-3-5-sonnet-20241022","stream":true,"system":"Be concise"}`

	pyCode, err := ToPythonSDK(snapshot, requestJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	validatePythonAST(t, pyCode)

	if !strings.Contains(pyCode, "from anthropic import Anthropic") {
		t.Errorf("missing Anthropic import, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `api_key=os.environ.get("ANTHROPIC_KEY", "")`) {
		t.Errorf("missing env var retrieval, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `base_url="https://api.anthropic.com/v1"`) {
		t.Errorf("missing base_url, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "with client.messages.stream(") {
		t.Errorf("missing messages.stream call, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `model="claude-3-5-sonnet-20241022"`) {
		t.Errorf("missing model, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `max_tokens=1024`) {
		t.Errorf("missing max_tokens, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, `system="Be concise"`) {
		t.Errorf("missing system, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "for text in stream.text_stream:") {
		t.Errorf("missing text_stream loop, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "# Non-streaming variant:") {
		t.Errorf("missing non-streaming variant comment, got: %s", pyCode)
	}
}

func TestToPythonSDK_Anthropic_NonStreaming(t *testing.T) {
	snapshot := `{
		"provider": {
			"protocol": "anthropic",
			"base_url": "https://api.anthropic.com/v1",
			"api_key_ref": "enc:secret"
		},
		"target": {
			"label": "claude",
			"model_name": "claude-3-5-sonnet-20241022"
		}
	}`
	requestJSON := `{"max_tokens":512,"messages":[{"content":"hello","role":"user"}],"model":"claude-3-5-sonnet-20241022","stream":false}`

	pyCode, err := ToPythonSDK(snapshot, requestJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	validatePythonAST(t, pyCode)

	if !strings.Contains(pyCode, `api_key=os.environ.get("ANTHROPIC_API_KEY", "")`) {
		t.Errorf("missing default ANTHROPIC_API_KEY fallback, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "response = client.messages.create(") {
		t.Errorf("missing messages.create call, got: %s", pyCode)
	}
	if strings.Contains(pyCode, "with client.messages.stream(") {
		t.Errorf("should not use messages.stream when non-streaming, got: %s", pyCode)
	}
	if !strings.Contains(pyCode, "for block in response.content:") {
		t.Errorf("missing content block loop, got: %s", pyCode)
	}
}
