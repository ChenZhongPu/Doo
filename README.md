# doo 🚀

A lightweight, single-binary, local web tool for **LLM API single-input testing and benchmark pilot studies**.

Before running expensive batch benchmarks or dataset evaluations, `doo` allows you to configure multiple LLM providers and model presets, send a single prompt across several targets in parallel, inspect real-time streaming output side-by-side, analyze latency and throughput metrics (TTFT, tokens/s, finish reason), and export identical curl/Python snippets.

---

## ✨ Features

- **Multi-Target Parallel Comparison**: Send the same system prompt and multi-turn messages to up to 8 targets simultaneously.
- **Side-by-Side Streaming**: Live streaming output with reasoning/thinking trace visibility, plain text or Markdown rendering.
- **Accurate Latency & Throughput Metrics**: Measures Time-to-First-Token (TTFT), First-Content latency, total time, prompt/completion/reasoning tokens, and tokens/second.
- **Exact Byte-Level Request & Response Inspector**: View the raw request payload and raw upstream SSE response.
- **Lossless Export to curl & Python**: Export calls directly into reproducible `curl` commands (with heredoc `--data-binary`) and `httpx` Python scripts.
- **Internationalization (i18n)**: Built-in bilingual UI (English and Chinese, default English), toggleable at any time and saved in local storage.
- **Secure Key Management**: Supports referencing environment variables (`env:VAR_NAME`) or locally encrypted API keys (AES-256-GCM via `master.key`). Keys are never returned in plain text or logged.
- **Single Self-Contained Binary**: Zero CGO dependencies (`CGO_ENABLED=0`), embedded SQLite and embedded web frontend, binary size < 15 MB.

---

## 🛠 Tech Stack

| Layer | Technology |
|---|---|
| **Backend** | Go ≥ 1.22, standard library `net/http` (no web framework) |
| **Storage** | SQLite via `modernc.org/sqlite` (pure Go, WAL mode) |
| **Frontend** | Svelte + Vite + TypeScript, pure CSS (no bloated UI library) |
| **Embedding** | `go:embed` embedding `web/dist` directly into the binary |
| **Target Size**| Single executable < 20 MB |

---

## 🚀 Quick Start

### 1. Build from Source

Prerequisites: Go ≥ 1.22, Node.js ≥ 18, npm.

```bash
# Build frontend and binary
make build

# Or step by step:
make web
make build
```

The resulting executable will be generated at `bin/doo`.

### 2. Run

```bash
# Start doo on default address (127.0.0.1:8787)
./bin/doo

# Or specify custom address, data directory, and open browser automatically:
./bin/doo --addr 127.0.0.1:8787 --data ~/.doo --open
```

CLI Flags:
- `--addr`: Server address (default `127.0.0.1:8787`).
- `--data`: Data directory path storing `doo.db` and `master.key` (default `~/.doo`).
- `--open`: Automatically opens browser on startup.
- `--version`: Prints version and exits.

---

## 📖 Usage Guide

### 1. Provider & Target Setup (`#/config`)
1. Navigate to **Config** (`#/config`).
2. **Add a Provider**:
   - OpenAI compatible (`openai_chat`) or Anthropic (`anthropic`).
   - Base URL (e.g., `https://api.openai.com/v1` or `https://api.anthropic.com/v1`).
   - API Key: Choose an environment variable reference (e.g. `OPENAI_API_KEY`) or direct encrypted input.
   - Test connectivity with the **Test Connection** button.
3. **Add Targets (Model Presets)**:
   - Select a Provider, specify model name (e.g. `gpt-4o`, `claude-3-5-sonnet-20241022`).
   - Enable core parameters (temperature, top_p, max_tokens, stop sequences, seed). Only enabled parameters are sent in the request.
   - Add extra vendor-specific parameters via JSON if needed.

### 2. Comparison (`#/`)
1. On the **Compare** page (`#/`), select which Targets to compare from the left sidebar.
2. Enter an optional System prompt and your user/assistant conversation messages.
3. Click **Preview Request Body** to inspect the exact payload that will be sent.
4. Click **Send** to stream responses concurrently side-by-side.
5. Click **Copy Output**, **Export curl**, or **Export Python** on any card.

### 3. History (`#/history`)
- Inspect previous benchmark pilot runs.
- Click **Load into Compare** to restore historical prompts into the comparison workbench.

---

## 🧪 Testing & Verification

Run the comprehensive unit test suite:

```bash
make test
```

This runs:
- `go vet ./...`
- `go test -v -count=1 ./...` covering adapter stream decoders, mock httptest servers, BuildBody verification, encryption, security middleware, and SQLite migrations.

---

## 📦 Cross-Platform Release

To cross-compile binaries for Linux, macOS, and Windows:

```bash
make release
```

Artifacts will be output to `dist/`:
- `dist/doo-1.0.0-linux-amd64`
- `dist/doo-1.0.0-linux-arm64`
- `dist/doo-1.0.0-darwin-amd64`
- `dist/doo-1.0.0-darwin-arm64`
- `dist/doo-1.0.0-windows-amd64.exe`
