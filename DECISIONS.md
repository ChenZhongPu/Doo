# Architectural & Design Decisions

This document records key decisions made during the development of `doo` adhering to Section 0 Rule 5 of the design specification:

1. **Project Naming (`doo`)**:
   - Replaced placeholder name `llmlab` with `doo` across the codebase, module name, binary output (`bin/doo`), default data directory (`~/.doo`), and database (`doo.db`).

2. **UI Internationalization (i18n)**:
   - Implemented reactive language store (`web/src/lib/i18n.ts`) supporting English (`en`) and Chinese (`zh`).
   - Default language is English (`en`), switchable via the navbar toggle button at any time.
   - Selected language preference is persisted in `localStorage` under key `doo_language`.

3. **Request Body Construction (`BuildBody`)**:
   - Formatted request serialization using `json.NewEncoder` with `SetEscapeHTML(false)` and no Unicode escaping to ensure raw UTF-8 and symbol preservation.
   - Trimming trailing newline ensures identical bytes between HTTP wire payload, `calls.request_json`, and export generators.

4. **SSE Event Multiplexing**:
   - Multiple targets executed concurrently per run are multiplexed across a single SSE response connection using typed event frames (`run`, `delta`, `done`, `error`, `end`).
   - `requestAnimationFrame` batching on the frontend buffers high-frequency streaming deltas, preventing UI thread blocking.

5. **Secrets & Security**:
   - Encrypted keys use AES-256-GCM with a 32-byte master key stored with `0600` permissions at `~/.doo/master.key`.
   - Host and Origin header validation prevents DNS rebinding and cross-origin attacks while allowing local benchmarking.
   - Log and database sanitization masks keys and tokens matching case-insensitive sensitive patterns.

6. **SQLite Concurrency & Migrations**:
   - Embedded version-tracked migration (`0001_init.sql`) using `PRAGMA user_version`.
   - `modernc.org/sqlite` pure Go driver configured with WAL mode and `db.SetMaxOpenConns(1)` for zero CGO dependency cross-compilation.

7. **Mainstream Provider Templates & Icons**:
   - Built SVG icon component (`ProviderIcon.svelte`) and provider quick templates for DeepSeek, OpenRouter, GLM (智谱 AI), OpenAI, Anthropic, Groq, Moonshot (Kimi), and Ollama.
   - Pre-fills gateway base URLs, default environment variables, and vendor-specific headers (e.g. OpenRouter HTTP-Referer).

8. **Terminology: "Target" -> "Model / Model Preset (模型 / 模型预设)"**:
   - Shifted user-facing terminology from abstract "Target" to intuitive "Model" and "Model Preset" in both English and Chinese UI copies.

9. **Hierarchical 2-Column Provider & Model Architecture**:
   - Separated Provider (API gateway, credentials, endpoints) and Model (presets, parameters, model names) into a clear parent-child 2-panel layout.
   - Left side manages service providers; right side manages models under the active provider with 1-click recommended model chips.
   - Comparison sidebar groups models under their respective provider with brand logo icons.

10. **Build & Version Metadata**:
    - Embedded git commit and build timestamp into binary via Go `-ldflags` (`-X main.buildTime=... -X main.gitCommit=...`).
    - Exposes version and build metadata via `/api/health` and displays it prominently in the top-left navigation header.
