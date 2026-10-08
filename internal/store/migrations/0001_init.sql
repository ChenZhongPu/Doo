CREATE TABLE IF NOT EXISTS providers(
  id            INTEGER PRIMARY KEY,
  name          TEXT NOT NULL UNIQUE,
  protocol      TEXT NOT NULL CHECK(protocol IN ('openai_chat','anthropic')),
  base_url      TEXT NOT NULL,
  api_key_ref   TEXT NOT NULL,
  headers_json  TEXT NOT NULL DEFAULT '{}',
  stream_usage  INTEGER NOT NULL DEFAULT 1,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS targets(
  id               INTEGER PRIMARY KEY,
  provider_id      INTEGER NOT NULL REFERENCES providers(id) ON DELETE CASCADE,
  label            TEXT NOT NULL,
  model_name       TEXT NOT NULL,
  core_json        TEXT NOT NULL DEFAULT '{}',
  extra_json       TEXT NOT NULL DEFAULT '{}',
  max_tokens_field TEXT NOT NULL DEFAULT 'max_tokens'
                   CHECK(max_tokens_field IN ('max_tokens','max_completion_tokens')),
  created_at       INTEGER NOT NULL,
  updated_at       INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS runs(
  id          INTEGER PRIMARY KEY,
  created_at  INTEGER NOT NULL,
  input_json  TEXT NOT NULL,
  note        TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS calls(
  id                   INTEGER PRIMARY KEY,
  run_id               INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
  target_id            INTEGER REFERENCES targets(id) ON DELETE SET NULL,
  target_snapshot_json TEXT NOT NULL,
  status               TEXT NOT NULL CHECK(status IN ('running','done','error','canceled')),
  request_json         TEXT NOT NULL,
  request_headers_json TEXT NOT NULL,
  response_raw         BLOB,
  response_truncated   INTEGER NOT NULL DEFAULT 0,
  output_text          TEXT NOT NULL DEFAULT '',
  reasoning_text       TEXT NOT NULL DEFAULT '',
  finish_reason        TEXT,
  http_status          INTEGER,
  error                TEXT,
  usage_json           TEXT,
  prompt_tokens        INTEGER,
  completion_tokens    INTEGER,
  reasoning_tokens     INTEGER,
  ttfb_ms              REAL,
  first_token_ms       REAL,
  first_content_ms     REAL,
  total_ms             REAL,
  response_model       TEXT,
  provider_req_id      TEXT,
  started_at           INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_calls_run ON calls(run_id);
CREATE INDEX IF NOT EXISTS idx_runs_created ON runs(created_at DESC);
