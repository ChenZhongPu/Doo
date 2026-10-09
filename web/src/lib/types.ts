export interface CoreParams {
  temperature?: number;
  top_p?: number;
  max_tokens?: number;
  stream?: boolean;
  stop?: string[];
  seed?: number;
}

export interface Provider {
  id: number;
  name: string;
  protocol: 'openai_chat' | 'anthropic';
  base_url: string;
  api_key_ref: string;
  headers: Record<string, string>;
  stream_usage: boolean;
  created_at: number;
  updated_at: number;
}

export interface Target {
  id: number;
  provider_id: number;
  label: string;
  model_name: string;
  core: CoreParams;
  extra: Record<string, any>;
  max_tokens_field: 'max_tokens' | 'max_completion_tokens';
  created_at: number;
  updated_at: number;
}

export interface Message {
  role: 'user' | 'assistant' | 'system';
  content: string;
}

export interface Input {
  system?: string;
  messages: Message[];
}

export interface Metrics {
  ttfb_ms?: number | null;
  first_token_ms?: number | null;
  first_content_ms?: number | null;
  total_ms?: number | null;
}

export interface CallRecord {
  id: number;
  run_id: number;
  target_id: number | null;
  target_snapshot_json: string;
  status: 'running' | 'done' | 'error' | 'canceled';
  request_json: string;
  request_headers_json: string;
  response_truncated: boolean;
  output_text: string;
  reasoning_text: string;
  finish_reason: string | null;
  http_status: number | null;
  error: string | null;
  usage_json: string | null;
  prompt_tokens: number | null;
  completion_tokens: number | null;
  reasoning_tokens: number | null;
  ttfb_ms: number | null;
  first_token_ms: number | null;
  first_content_ms: number | null;
  total_ms: number | null;
  response_model: string | null;
  provider_req_id: string | null;
  started_at: number;
}

export interface RunSummaryCall {
  id: number;
  target_id: number | null;
  label: string;
  status: string;
  total_ms: number | null;
  prompt_tokens: number | null;
  completion_tokens: number | null;
  error: string | null;
}

export interface RunSummary {
  id: number;
  created_at: number;
  input_json: string;
  note: string;
  calls: RunSummaryCall[];
}

export interface RunDetail {
  id: number;
  created_at: number;
  input_json: string;
  note: string;
  calls: CallRecord[];
}

export interface RunPage {
  runs: RunSummary[];
  total: number;
  page: number;
  page_size: number;
}

export interface CallRaw {
  request_json: string;
  request_headers: string;
  response_raw: string;
  truncated: boolean;
}
