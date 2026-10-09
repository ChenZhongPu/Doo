# doo 🚀

A lightweight, single-binary, local web tool for **LLM API single-input testing and benchmark pilot studies**.

Before running expensive batch benchmarks or dataset evaluations, `doo` allows you to configure multiple LLM providers and model presets, send a single prompt across several targets in parallel, inspect real-time streaming output side-by-side, analyze latency and throughput metrics (TTFT, tokens/s, finish reason), and export reproducible curl/Python snippets.

---

## ✨ Features

- **Multi-Target Parallel Comparison**: Send the same system prompt and multi-turn messages to up to 8 targets simultaneously.
- **Side-by-Side Streaming**: Live streaming output with reasoning/thinking trace visibility, plain text or Markdown rendering.
- **Accurate Latency & Throughput Metrics**: Measures Time-to-First-Token (TTFT), First-Content latency, total time, prompt/completion/reasoning tokens, and tokens/second.
- **Exact Byte-Level Inspector**: View raw request payloads and raw upstream SSE streams.
- **Lossless Export to curl & Python**: Export calls directly into reproducible `curl` commands (with heredoc `--data-binary`) and `httpx` Python scripts.
- **Internationalization (i18n)**: Built-in bilingual UI (English and Chinese, default English), toggleable at any time and saved in local storage.
- **Themes & Appearance**: Light, dark, and system themes with a persistent appearance selector.
- **Secure Key Management**: Supports referencing environment variables (`env:VAR_NAME`) or locally encrypted API keys (AES-256-GCM via `master.key`). Keys are never returned in plain text or logged.
- **Single Self-Contained Binary**: Zero CGO dependencies (`CGO_ENABLED=0`), embedded SQLite and embedded web frontend, binary size < 20 MB.

---

## 🚀 Quick Start

### Build from Source

Prerequisites: Go ≥ 1.22, Node.js ≥ 18, npm.

```bash
# Build frontend and binary
make all

# The compiled binary will be placed at bin/doo
```

### Run

```bash
# Start doo with default settings (127.0.0.1:8787)
./bin/doo

# Or specify a custom address, data directory, and open browser automatically:
./bin/doo --addr 127.0.0.1:8787 --data ~/.doo --open
```

### CLI Flags

| Flag | Default | Description |
|---|---|---|
| `--addr` | `127.0.0.1:8787` | Server listen address (`host:port`) |
| `--host` | `localhost` | Server listen host |
| `--port` | `8787` | Server listen port |
| `--data` | `~/.doo` | Data directory storing `doo.db` and `master.key` |
| `--max-targets` | `8` | Maximum concurrent targets in comparison (`0` for unlimited) |
| `--open` | `true` | Automatically open default browser on launch |
| `--version` | `false` | Print version information and exit |
