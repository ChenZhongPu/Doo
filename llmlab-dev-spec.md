# llmlab 开发文档

> 给 code agent 的实现规范。项目名 `llmlab` 为占位名，可自行替换。
> UI 文案使用中文；代码、注释、commit message 使用英文。

## 0. 工作准则（先读）

1. 严格按本文档的范围实现，**不要**自行加入文档未列出的功能（尤其是批量任务、数据集、评分器、聊天历史管理）。
2. 依赖最小化。Go 侧只允许 `modernc.org/sqlite` 一个第三方依赖；前端只允许 `svelte`、`vite`、`typescript`、`marked`、`dompurify`。要加新依赖先说明理由。
3. **正确性靠结构保证**：请求体只由 `BuildBody` 一个函数生成；"预览请求体"、"实际发送"、"导出 curl/Python"三处必须复用同一函数，不允许各写一份。
4. 每个里程碑结束时：`go vet ./...`、`go test ./...` 通过，并更新 README 的使用说明。
5. 遇到文档没覆盖的决策，选更简单的方案并在 `DECISIONS.md` 里记一行。

---

## 1. 项目定位

一个在浏览器中使用的 **LLM API 单条输入测试工具**，用于 benchmark 之前的 pilot study：

- 配置多家 LLM 服务（Provider）和模型/参数预设（Target）。
- 输入**一条** prompt（system + 多轮 messages），勾选若干 Target **并行**发送。
- 并排流式显示输出，展示 TTFT、总耗时、token 用量、tokens/s、finish_reason。
- 可查看每次调用**实际发出的请求体**与**原始响应**，并导出为 curl / Python 片段，便于把 pilot 配置无损迁移到后续的批量评测脚本。

### 非目标（不要做）

批量/数据集运行、实验 Spec、自动评分与 LLM-as-judge、重试与限流调度、多用户与登录、对话式聊天历史、知识库、MCP、插件、多模态输入（v1 仅文本，但数据结构见 §4.1 预留）。

---

## 2. 技术栈与约束

| 项 | 选择 |
|---|---|
| 后端 | Go ≥ 1.22，标准库 `net/http`（使用 1.22 的方法+路径路由），**不引入 Web 框架** |
| 存储 | SQLite，`modernc.org/sqlite`（纯 Go，`CGO_ENABLED=0` 可交叉编译） |
| 前端 | Svelte + Vite + TypeScript，纯 CSS（不引入 UI 组件库），hash 路由 |
| 嵌入 | 前端 `web/dist` 通过 `go:embed` 嵌入二进制 |
| 产物 | 单一二进制，无运行时依赖。目标体积 < 20 MB |
| 默认监听 | `127.0.0.1:8787` |

SQLite 初始化：`PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000; PRAGMA foreign_keys=ON;`，`db.SetMaxOpenConns(1)`（本地单用户，简单优先）。**不要在流式传输期间持有数据库事务**。

---

## 3. 目录结构

```
llmlab/
├── cmd/llmlab/main.go           # flags, wiring, graceful shutdown
├── internal/
│   ├── adapter/
│   │   ├── adapter.go           # interface + shared types
│   │   ├── body.go              # BuildBody, deepMerge   (核心，必须有单测)
│   │   ├── openai_chat.go
│   │   ├── anthropic.go
│   │   ├── sse.go               # SSE line reader
│   │   └── *_test.go
│   ├── secrets/secrets.go       # env: / enc: key refs
│   ├── store/
│   │   ├── store.go
│   │   └── migrations/0001_init.sql
│   ├── api/
│   │   ├── router.go
│   │   ├── security.go          # Host/Origin checks
│   │   ├── providers.go targets.go runs.go export.go
│   │   └── sse_out.go           # run event multiplexer
│   └── export/export.go         # curl / python snippet
├── web/
│   ├── embed.go                 # //go:embed all:dist
│   ├── package.json vite.config.ts
│   └── src/ (App.svelte, routes/, lib/)
├── Makefile
├── README.md
└── DECISIONS.md
```

CLI flags：`--addr 127.0.0.1:8787`、`--data ~/.llmlab`（存放 `llmlab.db` 与 `master.key`）、`--open`（启动后打开浏览器）。

---

## 4. 数据模型

### 4.1 输入格式（API 与 `runs.input_json` 共用）

```json
{
  "system": "optional string",
  "messages": [
    {"role": "user", "content": "text"},
    {"role": "assistant", "content": "text"},
    {"role": "user", "content": "text"}
  ]
}
```

`content` v1 只接受 string。解析时用 `json.RawMessage` 保留原值，以便将来扩展为 parts 数组而不改库表。

### 4.2 DDL（`0001_init.sql`）

迁移使用 `PRAGMA user_version` 递增，SQL 文件嵌入二进制。

```sql
CREATE TABLE providers(
  id            INTEGER PRIMARY KEY,
  name          TEXT NOT NULL UNIQUE,
  protocol      TEXT NOT NULL CHECK(protocol IN ('openai_chat','anthropic')),
  base_url      TEXT NOT NULL,            -- 含版本前缀，如 https://api.openai.com/v1
  api_key_ref   TEXT NOT NULL,            -- 'env:VAR' 或 'enc:<base64>'
  headers_json  TEXT NOT NULL DEFAULT '{}',-- 额外请求头（如 OpenRouter 的 HTTP-Referer）
  stream_usage  INTEGER NOT NULL DEFAULT 1,-- openai_chat: 是否发送 stream_options.include_usage
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE targets(
  id               INTEGER PRIMARY KEY,
  provider_id      INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
  label            TEXT NOT NULL,
  model_name       TEXT NOT NULL,
  core_json        TEXT NOT NULL DEFAULT '{}', -- 仅包含"已设置"的字段，见 §5.1
  extra_json       TEXT NOT NULL DEFAULT '{}', -- 厂商特有参数，深度合并
  max_tokens_field TEXT NOT NULL DEFAULT 'max_tokens'
                   CHECK(max_tokens_field IN ('max_tokens','max_completion_tokens')),
  created_at       INTEGER NOT NULL,
  updated_at       INTEGER NOT NULL
);

CREATE TABLE runs(
  id          INTEGER PRIMARY KEY,
  created_at  INTEGER NOT NULL,
  input_json  TEXT NOT NULL,
  note        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE calls(
  id                   INTEGER PRIMARY KEY,
  run_id               INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  target_id            INTEGER REFERENCES targets(id) ON DELETE SET NULL,
  target_snapshot_json TEXT NOT NULL,  -- 发送时的 provider+target 配置快照（不含密钥）
  status               TEXT NOT NULL CHECK(status IN ('running','done','error','canceled')),
  request_json         TEXT NOT NULL,  -- 实际发出的请求体（字节级一致）
  request_headers_json TEXT NOT NULL,  -- 已脱敏
  response_raw         BLOB,           -- 原始响应字节（流式则为完整 SSE 文本），上限 4 MiB
  response_truncated   INTEGER NOT NULL DEFAULT 0,
  output_text          TEXT NOT NULL DEFAULT '',
  reasoning_text       TEXT NOT NULL DEFAULT '',
  finish_reason        TEXT,
  http_status          INTEGER,
  error                TEXT,
  usage_json           TEXT,           -- 上游原始 usage 对象
  prompt_tokens        INTEGER,
  completion_tokens    INTEGER,
  reasoning_tokens     INTEGER,
  ttfb_ms              REAL,
  first_token_ms       REAL,           -- 首个 reasoning 或 content 增量
  first_content_ms     REAL,           -- 首个非空 content 增量
  total_ms             REAL,
  response_model       TEXT,           -- 上游返回的模型名（别名漂移时有用）
  provider_req_id      TEXT,
  started_at           INTEGER NOT NULL
);
CREATE INDEX idx_calls_run ON calls(run_id);
CREATE INDEX idx_runs_created ON runs(created_at DESC);
```

启动时执行一次：`UPDATE calls SET status='error', error='interrupted' WHERE status='running'`。

---

## 5. 核心逻辑

### 5.1 参数模型

```go
type CoreParams struct {
    Temperature *float64 `json:"temperature,omitempty"`
    TopP        *float64 `json:"top_p,omitempty"`
    MaxTokens   *int     `json:"max_tokens,omitempty"`
    Stop        []string `json:"stop,omitempty"`
    Seed        *int64   `json:"seed,omitempty"`
}
```

**规则：只发送被显式设置（非 nil）的字段，绝不填默认值。** 推理模型不接受 `temperature`，用户只需在 UI 里取消勾选即可。

### 5.2 `BuildBody`（唯一的请求体构造函数）

```go
func BuildBody(p Provider, t Target, in Input) (body []byte, err error)
```

步骤：

1. 构造 `base map[string]any`：`model = t.ModelName`，`stream = true`。
2. 映射 messages：
   - `openai_chat`：若 `system` 非空，在 messages 开头插入 `{"role":"system","content":system}`。
   - `anthropic`：`system` 作为顶层字符串字段；messages 原样放入。
3. 映射核心参数：
   - `openai_chat`：`temperature`、`top_p`、`seed`、`stop` 同名；最大输出长度写入键 `t.MaxTokensField`（`max_tokens` 或 `max_completion_tokens`）。若 `p.StreamUsage`，加 `stream_options: {"include_usage": true}`。
   - `anthropic`：`temperature`、`top_p`；`stop` → `stop_sequences`；`max_tokens` 是**必填**，未设置时返回校验错误 `max_tokens is required for anthropic`（不要擅自补默认值）。`seed` 不支持：若已设置则返回校验错误而不是静默丢弃。
4. `deepMerge(base, extra)`：
   - 对象递归合并；其他类型（含数组）整体替换；
   - **extra 优先于 core**；
   - extra 中值为 `null` 表示**删除**该键（用于移除某个默认字段）。
5. 序列化：使用 `json.NewEncoder` 并 `SetEscapeHTML(false)`；map 键按字典序输出（Go 默认）；**不做 ASCII 转义**，非 ASCII 字符保持 UTF-8 原样。该函数的输出字节就是实际发送的字节，也是写入 `calls.request_json` 的内容。

必须有单测覆盖：未设置字段不出现、extra 覆盖与 null 删除、`<` `&` 不被转义、中文不被转义、Anthropic 缺 `max_tokens` 报错。

### 5.3 适配器接口

```go
type Event struct {
    Kind string // "content" | "reasoning"
    Text string
}

type Result struct {
    Status        string // done | error | canceled
    HTTPStatus    int
    Err           string
    FinishReason  string
    UsageJSON     json.RawMessage
    PromptTokens, CompletionTokens, ReasoningTokens *int
    ResponseModel string
    ProviderReqID string
    RespRaw       []byte
    RespTruncated bool
    Output, Reasoning string
    Metrics       Metrics
}

type Metrics struct {
    TTFBms, FirstTokenMs, FirstContentMs, TotalMs *float64
}

type Adapter interface {
    // body 为 BuildBody 的输出；on 在每个增量到达时同步回调。
    Do(ctx context.Context, p Provider, apiKey string, body []byte, on func(Event)) *Result
}
```

通用要求：

- `http.Client` 不设置整体 `Timeout`（会截断流）；超时由 `context.WithTimeout`（默认 300 s，可在请求里覆盖）控制。
- 计时起点 `t0` 为调用 `client.Do` 之前；使用 `time.Now()` 的单调时钟，并通过可注入的 `clock func() time.Time` 便于测试。
- `ttfb_ms`：收到响应头的时间；`first_token_ms`：首个 reasoning 或 content 非空增量；`first_content_ms`：首个非空 content 增量；`total_ms`：流结束（或出错/取消）的时间。
- `tokens_per_s` 不入库，展示时计算：`completion_tokens / ((total_ms - first_token_ms)/1000)`，仅当 `completion_tokens>0` 且分母>0。token 数**只用上游返回的 usage**，不用本地分词器估算。
- 原始响应 `RespRaw` 记录收到的字节原样（上限 4 MiB，超出则截断并置 `RespTruncated`）；非 2xx 时记录响应体（上限 64 KiB）。
- SSE 读取使用 `bufio.Reader.ReadBytes('\n')`（**不要用 `bufio.Scanner`**，会有 token 长度限制）；处理 `data:` 前缀、可选空格、空行分隔、`:` 开头的注释行；忽略未知字段。
- 流中途断开 → `status=error`，已收到的输出照常保存。`ctx` 被取消 → `status=canceled`。

### 5.4 `openai_chat` 适配器

- 请求：`POST {base_url}/chat/completions`，头：`Authorization: Bearer <key>`、`Content-Type: application/json`、`Accept: text/event-stream`，再附加 provider 自定义头。
- 逐个 `data:` 事件：`data: [DONE]` 结束；否则解析 JSON：
  - `choices[0].delta.content` 非空 → `Event{content}`；
  - `choices[0].delta.reasoning_content` **或** `choices[0].delta.reasoning` 非空 → `Event{reasoning}`；
  - `choices[0].finish_reason` 非空 → 记录；
  - 任意 chunk 中 `usage` 非 null → 记录（最后一个 chunk 通常 `choices` 为空）；
  - `model`、`id` → `ResponseModel`、`ProviderReqID`（取首次出现的值）；
  - chunk 含顶层 `error` 对象 → 视为上游错误，`status=error`。
- usage 映射：`prompt_tokens`、`completion_tokens`、`completion_tokens_details.reasoning_tokens`。

### 5.5 `anthropic` 适配器

- 请求：`POST {base_url}/messages`（`base_url` 约定为 `https://api.anthropic.com/v1`），头：`x-api-key`、`anthropic-version: 2023-06-01`、`Content-Type: application/json`。
- 以每个 `data:` JSON 的 `type` 字段分派（不依赖 `event:` 行）：
  - `message_start`：取 `message.id`、`message.model`、`message.usage.input_tokens`；
  - `content_block_delta`：`delta.type=="text_delta"` → content（`delta.text`）；`delta.type=="thinking_delta"` → reasoning（`delta.thinking`）；其余 delta 类型忽略；
  - `message_delta`：`delta.stop_reason` → finish_reason；`usage.output_tokens`（累计值，取最后一次）；
  - `error`：上游错误；`ping`/`content_block_start`/`content_block_stop`/`message_stop`：忽略。
- `usage_json` 合并 `message_start` 与 `message_delta` 的 usage 对象（后者覆盖同名键）。Anthropic 不单独返回 reasoning token 数，`reasoning_tokens` 置 NULL，不要估算。

### 5.6 密钥（`secrets` 包）

- `api_key_ref` 两种形式：
  - `env:VAR_NAME`：调用时 `os.Getenv`；为空则报错 `env var VAR_NAME is empty`；
  - `enc:<base64>`：AES-256-GCM（随机 12 字节 nonce 前置），主密钥为 `--data/master.key`（首次启动随机生成 32 字节，权限 `0600`）。
- API 永远**不返回**明文密钥；GET provider 时只返回 `api_key_ref` 的类型（`env:VAR_NAME` 原样返回，`enc:` 返回 `enc:***`）。
- 日志与 `request_headers_json` 中的 `Authorization`、`x-api-key` 以及名称匹配 `(?i)key|token|secret|auth` 的自定义头一律写为 `***`。

### 5.7 本地安全

- 默认只监听回环地址。
- 中间件校验：`Host` 必须是 `localhost`、`127.0.0.1`、`[::1]` 或 `--addr` 指定的主机（防 DNS rebinding）；非 GET/HEAD 请求若带 `Origin`，必须与 `Host` 同源（防 CSRF）。不满足返回 403。
- 若 `--addr` 不是回环地址，启动时打印醒目警告。

---

## 6. HTTP API

统一前缀 `/api`，JSON 请求与响应。错误格式：`{"error": "message"}` + 合适的状态码。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | `{"ok":true,"version":"..."}` |
| GET/POST | `/api/providers` | 列表 / 创建 |
| GET/PUT/DELETE | `/api/providers/{id}` | 读取 / 更新 / 删除（级联删除 targets） |
| POST | `/api/providers/{id}/test` | 连通性测试：发送最小请求（`"ping"`，`max_tokens=8`，流式），返回 `{ok, http_status, total_ms, error?}`。该请求**不入库** |
| GET/POST | `/api/targets` | 列表 / 创建 |
| GET/PUT/DELETE | `/api/targets/{id}` | 读取 / 更新 / 删除 |
| POST | `/api/targets/{id}/duplicate` | 复制（label 追加 ` (copy)`） |
| POST | `/api/preview` | 请求体 `{target_id, input}` → 返回将要发送的 `body`（字符串）与 `headers`（已脱敏）。内部调用与实际发送相同的 `BuildBody` |
| POST | `/api/runs` | **SSE 响应**，见 §6.1 |
| POST | `/api/calls/{id}/cancel` | 取消某个仍在运行的 call |
| GET | `/api/runs?limit=50&before=<id>` | 历史列表（含各 call 摘要） |
| GET | `/api/runs/{id}` | run 详情（含所有 call 的完整字段，不含 `response_raw`） |
| DELETE | `/api/runs/{id}` | 删除 |
| GET | `/api/calls/{id}/raw` | `{request_json, request_headers, response_raw(文本), truncated}` |
| GET | `/api/calls/{id}/export?format=curl\|python` | 返回文本片段，见 §7 |

### 6.1 `POST /api/runs`

请求：

```json
{
  "input": { "system": "...", "messages": [ ... ] },
  "target_ids": [1, 3, 4],
  "timeout_s": 300,
  "note": ""
}
```

处理流程：

1. 校验：`target_ids` 非空（上限 8）；所有 target 存在；对每个 target 先调用 `BuildBody`，**任一校验失败则整体返回 400，不发出任何请求**，错误信息指明是哪个 target。
2. 写入 `runs` 与每个 `calls`（`status=running`）。
3. 响应头 `Content-Type: text/event-stream`，`Cache-Control: no-cache`，每个事件后 `Flush()`。
4. 每个 target 一个 goroutine 调用 `Adapter.Do`，增量通过一个 channel 汇入单条 SSE 响应（**单连接多路复用**）。
5. 所有 call 结束后发送 `end` 事件并关闭。
6. 客户端断开（`r.Context()` 取消）→ 取消所有未完成的上游请求；已收到的部分输出以 `canceled` 状态落库。
7. `POST /api/calls/{id}/cancel` 通过 `map[callID]context.CancelFunc`（加互斥锁）取消单个 call。

SSE 事件（每个事件一行 `data: <json>\n\n`，`type` 字段区分）：

```json
{"type":"run","run_id":12,"calls":[{"call_id":31,"target_id":1,"label":"model-a"}]}
{"type":"delta","call_id":31,"kind":"content","text":"Hel"}
{"type":"delta","call_id":31,"kind":"reasoning","text":"Let me think"}
{"type":"done","call_id":31,"status":"done","finish_reason":"stop",
 "usage":{"prompt_tokens":20,"completion_tokens":150,"reasoning_tokens":0},
 "metrics":{"ttfb_ms":210.3,"first_token_ms":480.1,"first_content_ms":480.1,"total_ms":3120.7},
 "response_model":"model-a-2025-xx","http_status":200}
{"type":"error","call_id":32,"http_status":429,"message":"..."}
{"type":"end"}
```

`done` / `error` 事件在落库完成后发送。出错的 call 也会带 `metrics` 与已收到的输出。

---

## 7. 导出（`internal/export`）

输入为某个 call 的 `target_snapshot_json` 与 `request_json`（即实际发出的请求体）。

- **curl**：输出完整命令；URL、头、body 与实际请求一致；密钥使用占位符（`env:` 引用 → `"$VAR_NAME"`；`enc:` → `"$API_KEY"`）。body 通过 heredoc `--data-binary @- <<'EOF'` 传入，避免 shell 转义破坏内容。
- **python**：使用 `httpx`，从环境变量读取密钥；`json.loads` 一个由原始 body 字符串生成的字面量，**不要**重新拼装字段；附带一个非流式版本的注释说明（把 `"stream": true` 去掉并读取 `usage`）。

两者都必须有单测，对照黄金文件。

---

## 8. 前端规范

路由（hash 路由）：`#/`（对比页，默认）、`#/config`、`#/history`。

### 8.1 对比页（`#/`）

布局：

- **左侧栏**：Target 复选列表（`label` + provider 名），以及"配置"链接。
- **顶部输入区**：
  - system 文本框（可折叠）；
  - messages 编辑器：每轮一个 `role` 下拉（user/assistant）+ 多行文本框，可增删、上下移动；
  - "预览请求体"按钮：对每个勾选的 target 调用 `/api/preview`，在抽屉中展示；
  - "发送"、"全部停止"按钮；超时秒数输入框。
- **结果区**：`display:grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr))`，每个已勾选 target 一张卡片：
  - 头部：label、`provider / model_name`、状态徽标（running/done/error/canceled）、单卡"停止"按钮；
  - 主体 Tab：`输出`（默认纯文本 `pre-wrap`；可切换"Markdown 渲染"，使用 `marked` + `DOMPurify`）、`推理`（仅有 reasoning 内容时显示）、`请求`、`响应`（后两者在 call 完成后通过 `/api/calls/{id}/raw` 懒加载，等宽字体展示）；
  - 底部指标栏：`首 token`、`首正文`、`总耗时`、`输入/输出/推理 token`、`tokens/s`、`finish_reason`、`response_model`；缺失值显示 `—`，**不要用 0 代替**；
  - 操作：复制输出、导出 curl、导出 Python。
- 失败卡片显示 HTTP 状态码与上游错误文本。

流式读取：因为是 POST，使用 `fetch` + `response.body.getReader()` + `TextDecoder({stream:true})` 手写 SSE 解析（按 `\n\n` 切分、取 `data:` 行）。增量渲染按 `requestAnimationFrame` 批量刷新，避免高频重绘。点击"停止"通过 `AbortController.abort()` 断开连接。

仅作 UI 便利的状态（上次勾选的 targets、上次输入）存入 `localStorage`，需包 `try/catch`。

### 8.2 配置页（`#/config`）

两个区域：

**Provider 表单**：`name`、`protocol`（下拉）、`base_url`（占位示例：`https://api.openai.com/v1`、`https://api.anthropic.com/v1`）、密钥（单选：环境变量名 / 直接输入（加密保存）；输入后不回显）、额外请求头（JSON 编辑，校验后保存）、`stream_usage` 复选框（仅 openai_chat 显示，说明：部分兼容服务不支持，不支持时取消勾选）、"测试连接"按钮（显示状态与耗时）。

**Target 表单**：`label`、所属 Provider、`model_name`；核心参数每项一行"启用复选框 + 输入"，**未启用的字段不写入 `core_json`**；`max_tokens` 的字段名选择（`max_tokens` / `max_completion_tokens`，仅 openai_chat 显示）；`extra` JSON 编辑框（实时校验语法，错误内联显示）；"预览请求体"（用一条示例输入调用 `/api/preview`）；复制/删除。

### 8.3 历史页（`#/history`）

按时间倒序列出 run：时间、input 前 80 字、各 target 的状态与总耗时。点开展示与对比页相同的卡片（只读）。操作：`载入到对比页`（恢复 input；target 已被删除时，该 target 仅在只读视图中以快照形式显示，不可重发）、删除。

---

## 9. 构建与发布

`Makefile` 目标：

```
make web       # cd web && npm ci && npm run build
make build     # CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o bin/llmlab ./cmd/llmlab
make dev       # 并行运行 go run 与 vite dev（vite 将 /api 代理到 127.0.0.1:8787）
make release   # 交叉编译 linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
make test      # go vet ./... && go test ./...
```

`web/embed.go`：

```go
package web
import "embed"
//go:embed all:dist
var Dist embed.FS
```

非 `/api` 路径从嵌入的 `dist` 提供静态文件；`index.html` 设 `Cache-Control: no-cache`。

---

## 10. 测试要求

- **适配器**：用 `httptest.Server` 模拟上游，覆盖：
  - OpenAI：纯 content 流、带 `reasoning_content` 的流、带 `reasoning` 的流、usage 在末尾空 `choices` chunk、`[DONE]`、非 2xx（429，带 JSON 错误体）、流中断、chunk 内 `error` 对象；
  - Anthropic：text + thinking delta、`message_start`/`message_delta` usage 合并、`error` 事件、`max_tokens` 缺失校验；
  - 取消：`ctx` 取消后 `status=canceled`，且已收到的输出保留。
- **计时**：注入假时钟，断言 `ttfb/first_token/first_content/total` 的口径（reasoning 先于 content 到达时 `first_token < first_content`）。
- **BuildBody**：见 §5.2 的清单，使用黄金文件比较字节。
- **secrets**：加解密往返、`env:` 缺失报错、脱敏函数。
- **API**：创建 provider/target → `/api/preview` → `/api/runs`（指向 mock 上游）→ 校验落库内容与 SSE 事件序列；多 target 中一个校验失败时不发出任何上游请求。
- **安全中间件**：错误 Host、跨源 POST 返回 403。

---

## 11. 里程碑与验收

### M1：单目标打通
- 项目骨架、迁移、`secrets`、`BuildBody`、`openai_chat` 适配器及单测。
- `providers`/`targets` CRUD、`/api/preview`、`/api/runs`（支持 1 个 target）。
- 前端：配置页 + 对比页（单卡片）。
- **验收**：配置一个 OpenAI 兼容服务，发送一条 prompt，流式看到输出，指标与 `usage` 正确，历史可查，原始请求体与 curl 抓包一致。

### M2：并行对比
- 多 target 并行、SSE 多路复用、单卡取消与全部停止、历史页、请求/响应 Tab。
- **验收**：同时对 3 个 target 发送；中途点击"全部停止"，各卡状态为 canceled 且部分输出已保存；校验失败的 target 阻止整个 run 发出。

### M3：Anthropic 与导出
- `anthropic` 适配器、reasoning/thinking 展示、curl/Python 导出、Provider 连通性测试。
- **验收**：Anthropic 与 OpenAI 兼容服务混合对比；导出的 curl 在终端执行可复现同一请求。

### M4：打磨与发布
- 安全中间件、Markdown 渲染开关、错误提示、README（含配置示例与截图占位）、`make release`。
- **验收**：`bin/llmlab` 单文件在干净机器上启动即可使用，体积 < 20 MB，`go vet`/`go test` 全绿。

---

## 12. 实现注意事项清单

- 请求体与 `request_json` 必须是**同一份字节**；不要在入库前重新序列化或格式化。
- 原始响应入库前不做任何字符集转换；`output_text` 由增量文本拼接，JSON 解码带来的无效字节替换不影响 `response_raw` 的保真。
- 不要在 `BuildBody` 外的任何位置读写请求体字段。
- 指标缺失用 `NULL` / `—` 表示，不要用 0。
- 并发写库通过单连接串行化；增量文本在内存里累积，call 结束时一次性写入。
- 日志不得输出请求体中的密钥或完整响应体（仅输出 call_id、状态、耗时）。
