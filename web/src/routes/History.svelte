<script lang="ts">
  import { onMount } from 'svelte';
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import { api } from '../lib/api';
  import { t } from '../lib/i18n';
  import ProviderIcon from '../lib/ProviderIcon.svelte';
  import { cleanProviderName } from '../lib/providerTemplates';
  import type { RunSummary, RunDetail, CallRecord, CallRaw } from '../lib/types';

  let runs: RunSummary[] = $state([]);
  let loading: boolean = $state(true);
  let errorMsg: string = $state('');

  let expandedRunId: number | null = $state(null);
  let runDetails: Record<number, RunDetail> = $state({});
  let rawCalls: Record<number, CallRaw> = $state({});
  let activeTabs: Record<number, 'output' | 'reasoning' | 'request' | 'response'> = $state({});
  let renderMdMap: Record<number, boolean> = $state({});

  onMount(loadRuns);

  async function loadRuns() {
    loading = true;
    errorMsg = '';
    try {
      runs = (await api.listRuns(50)) || [];
    } catch (e: any) {
      errorMsg = e.message || 'Failed to load history';
    } finally {
      loading = false;
    }
  }

  async function toggleExpand(runId: number) {
    if (expandedRunId === runId) {
      expandedRunId = null;
      return;
    }

    expandedRunId = runId;
    if (!runDetails[runId]) {
      try {
        const detail = await api.getRun(runId);
        runDetails[runId] = detail;
        for (const c of (detail.calls || [])) {
          activeTabs[c.id] = 'output';
          renderMdMap[c.id] = true;
        }
      } catch (e: any) {
        alert(`Failed to load run details: ${e.message}`);
      }
    }
  }

  async function switchTab(callId: number, tab: 'output' | 'reasoning' | 'request' | 'response') {
    activeTabs[callId] = tab;
    if ((tab === 'request' || tab === 'response') && !rawCalls[callId]) {
      try {
        rawCalls[callId] = await api.getCallRaw(callId);
      } catch (e: any) {
        console.error('Failed to load raw call payload:', e);
      }
    }
  }

  async function handleDeleteRun(runId: number) {
    if (!confirm($t('confirmDeleteRun'))) return;
    try {
      await api.deleteRun(runId);
      if (expandedRunId === runId) expandedRunId = null;
      await loadRuns();
    } catch (e: any) {
      alert(`Delete failed: ${e.message}`);
    }
  }

  function handleLoadToCompare(run: RunSummary) {
    try {
      const parsedInput = JSON.parse(run.input_json);
      const targetIds = run.calls.map(c => c.target_id).filter((id): id is number => id !== null);

      // Dispatch custom event caught by Compare.svelte
      window.dispatchEvent(new CustomEvent('doo-load-run', {
        detail: {
          input: parsedInput,
          targetIds,
        }
      }));

      window.location.hash = '#/';
    } catch (e: any) {
      alert(`Failed to parse run input: ${e.message}`);
    }
  }

  function getPromptSnippet(inputJSON: string): string {
    try {
      const parsed = JSON.parse(inputJSON);
      if (parsed.messages && parsed.messages.length > 0) {
        const lastMsg = parsed.messages[parsed.messages.length - 1];
        let content = typeof lastMsg.content === 'string' ? lastMsg.content : JSON.stringify(lastMsg.content);
        if (content.length > 80) content = content.slice(0, 80) + '...';
        return content;
      }
    } catch (e) {
      // Ignore
    }
    return inputJSON.slice(0, 80);
  }

  function formatTime(ts: number): string {
    return new Date(ts).toLocaleString();
  }

  function formatMs(ms?: number | null): string {
    if (ms == null) return '—';
    if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`;
    return `${Math.round(ms)}ms`;
  }

  function renderMd(text: string): string {
    const raw = marked.parse(text, { async: false }) as string;
    return DOMPurify.sanitize(raw);
  }

  function parseSnapshot(call: CallRecord): { provider?: any; target?: any } {
    try {
      return JSON.parse(call.target_snapshot_json) || {};
    } catch {
      return {};
    }
  }

  function getTargetLabel(call: CallRecord): string {
    const snap = parseSnapshot(call);
    return snap.target?.label || `Target #${call.target_id}`;
  }

  function getCallTooltip(call: CallRecord): string {
    const pLabel = $t('providerLabel');
    const mLabel = $t('modelLabel');
    const snap = parseSnapshot(call);
    const provName = snap.provider?.name ? cleanProviderName(snap.provider.name) : '—';
    const modelName = snap.target?.model_name || '—';
    return `${pLabel}: ${provName}\n${mLabel}: ${modelName}`;
  }

  function calcSpeed(call: CallRecord): string {
    const comp = call.completion_tokens;
    const total = call.total_ms;
    const first = call.first_token_ms;
    if (comp && comp > 0 && total && first && total > first) {
      const durationSec = (total - first) / 1000;
      if (durationSec > 0) {
        return (comp / durationSec).toFixed(1);
      }
    }
    return '—';
  }
</script>

<div class="history-page">
  <div class="history-header">
    <h2>{$t('historyTitle')}</h2>
    <button class="btn btn-sm btn-secondary" onclick={loadRuns}>🔄 Refresh</button>
  </div>

  {#if errorMsg}
    <div class="alert alert-danger">{errorMsg}</div>
  {/if}

  {#if loading}
    <div class="loading-state">{$t('loading')}</div>
  {:else if runs.length === 0}
    <div class="empty-state">
      <p>{$t('noHistory')}</p>
    </div>
  {:else}
    <div class="run-list">
      {#each runs as run (run.id)}
        <div class="run-card" class:expanded={expandedRunId === run.id}>
          <!-- Run Item Header Summary -->
          <div
            class="run-summary-bar"
            role="button"
            tabindex="0"
            onclick={() => toggleExpand(run.id)}
            onkeydown={e => { if (e.key === 'Enter' || e.key === ' ') toggleExpand(run.id); }}
          >
            <div class="run-meta">
              <span class="run-time">{formatTime(run.created_at)}</span>
              {#if run.note}
                <span class="run-note badge badge-secondary">{run.note}</span>
              {/if}
            </div>

            <div class="run-prompt">
              "{getPromptSnippet(run.input_json)}"
            </div>

            <div class="run-calls-summary">
              {#each run.calls as c}
                <span class="badge badge-{c.status} call-pill">
                  {c.label}: {formatMs(c.totalMs)}
                </span>
              {/each}
            </div>

            <div class="run-actions" role="presentation" onclick={e => e.stopPropagation()}>
              <button
                class="btn btn-xs btn-outline"
                onclick={() => handleLoadToCompare(run)}
                title={$t('loadToCompare')}
              >
                📥 {$t('loadToCompare')}
              </button>
              <button
                class="btn btn-xs btn-danger"
                onclick={() => handleDeleteRun(run.id)}
                data-tooltip={$t('deleteRun')}
              >
                ✕
              </button>
              <button class="btn-text" onclick={() => toggleExpand(run.id)}>
                {expandedRunId === run.id ? $t('hideDetails') : $t('viewDetails')}
              </button>
            </div>
          </div>

          <!-- Expanded Cards (Read-only View) -->
          {#if expandedRunId === run.id && runDetails[run.id]}
            <div class="expanded-content">
              <div class="results-grid">
                {#each runDetails[run.id].calls as call (call.id)}
                  {@const snap = parseSnapshot(call)}
                  <div class="card result-card status-{call.status}">
                    <div class="card-header">
                      <div
                        class="card-title-group"
                        data-tooltip={getCallTooltip(call)}
                        data-tooltip-pos="bottom"
                      >
                        <div class="card-title-row">
                          {#if snap.provider}
                            <ProviderIcon name={snap.provider.name} baseUrl={snap.provider.base_url} protocol={snap.provider.protocol} size={18} />
                          {/if}
                          <h4 class="card-title">{getTargetLabel(call)}</h4>
                        </div>
                      </div>
                      <span class="badge badge-{call.status}">{call.status}</span>
                    </div>

                    <div class="card-tabs">
                      <button
                        class="tab-btn"
                        class:active={activeTabs[call.id] === 'output'}
                        onclick={() => switchTab(call.id, 'output')}
                      >
                        {$t('tabOutput')}
                      </button>

                      {#if call.reasoning_text}
                        <button
                          class="tab-btn"
                          class:active={activeTabs[call.id] === 'reasoning'}
                          onclick={() => switchTab(call.id, 'reasoning')}
                        >
                          {$t('tabReasoning')}
                        </button>
                      {/if}

                      <button
                        class="tab-btn"
                        class:active={activeTabs[call.id] === 'request'}
                        onclick={() => switchTab(call.id, 'request')}
                      >
                        {$t('tabRequest')}
                      </button>
                      <button
                        class="tab-btn"
                        class:active={activeTabs[call.id] === 'response'}
                        onclick={() => switchTab(call.id, 'response')}
                      >
                        {$t('tabResponse')}
                      </button>

                      {#if activeTabs[call.id] === 'output'}
                        <label class="md-toggle">
                          <input type="checkbox" checked={renderMdMap[call.id] ?? true} onchange={e => renderMdMap[call.id] = e.currentTarget.checked} />
                          <span>{$t('markdownRender')}</span>
                        </label>
                      {/if}
                    </div>

                    <div class="card-body">
                      {#if call.status === 'error' && call.error && activeTabs[call.id] === 'output'}
                        <div class="error-banner">
                          <strong>{call.http_status ? `HTTP ${call.http_status}: ` : ''}</strong>
                          {call.error}
                        </div>
                      {/if}

                      {#if activeTabs[call.id] === 'output'}
                        {#if renderMdMap[call.id] ?? true}
                          <div class="markdown-body">
                            {@html renderMd(call.output_text)}
                          </div>
                        {:else}
                          <pre class="pre-output">{call.output_text}</pre>
                        {/if}
                      {:else if activeTabs[call.id] === 'reasoning'}
                        <pre class="pre-reasoning">{call.reasoning_text}</pre>
                      {:else if activeTabs[call.id] === 'request'}
                        {#if rawCalls[call.id]}
                          <pre class="pre-raw">{rawCalls[call.id].request_json}</pre>
                        {:else}
                          <div class="loading-state">{$t('loadingRaw')}</div>
                        {/if}
                      {:else if activeTabs[call.id] === 'response'}
                        {#if rawCalls[call.id]}
                          <pre class="pre-raw">{rawCalls[call.id].response_raw}</pre>
                        {:else}
                          <div class="loading-state">{$t('loadingRaw')}</div>
                        {/if}
                      {/if}
                    </div>

                    <div class="card-metrics">
                      <div class="metric-item">
                        <span class="metric-label">{$t('metricTTFT')}</span>
                        <span class="metric-value">{formatMs(call.first_token_ms)}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">{$t('metricFirstContent')}</span>
                        <span class="metric-value">{formatMs(call.first_content_ms)}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">{$t('metricTotal')}</span>
                        <span class="metric-value">{formatMs(call.total_ms)}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">{$t('metricTokens')}</span>
                        <span class="metric-value">
                          {call.prompt_tokens ?? '—'} / {call.completion_tokens ?? '—'} / {call.reasoning_tokens ?? '—'}
                        </span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">{$t('metricSpeed')}</span>
                        <span class="metric-value">{calcSpeed(call)}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">{$t('metricFinish')}</span>
                        <span class="metric-value">{call.finish_reason || '—'}</span>
                      </div>
                      <div class="metric-item">
                        <span class="metric-label">{$t('metricModel')}</span>
                        <span class="metric-value truncate" title={call.response_model || ''}>
                          {call.response_model || '—'}
                        </span>
                      </div>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          {/if}
        </div>
      {/each}
    </div>
  {/if}
</div>

<style>
  .history-page {
    display: flex;
    flex-direction: column;
    gap: 1.5rem;
  }
  .history-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .history-header h2 {
    font-size: 1.25rem;
    font-weight: 700;
    letter-spacing: -0.02em;
    margin: 0;
    color: var(--text-main);
  }

  .run-list {
    display: flex;
    flex-direction: column;
    gap: 0.85rem;
  }

  .run-card {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    overflow: hidden;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
    transition: all 0.2s ease;
  }
  .run-card.expanded {
    border-color: rgba(99, 102, 241, 0.45);
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.25);
  }

  .run-summary-bar {
    display: grid;
    grid-template-columns: 190px 1fr auto auto;
    align-items: center;
    gap: 1.2rem;
    padding: 0.95rem 1.35rem;
    cursor: pointer;
    background: var(--bg-card);
    transition: background-color 0.15s ease;
  }
  .run-summary-bar:hover {
    background: rgba(255, 255, 255, 0.025);
  }

  .run-meta {
    display: flex;
    flex-direction: column;
    gap: 0.25rem;
  }
  .run-time {
    font-size: 0.78rem;
    font-weight: 500;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    color: var(--text-secondary);
  }
  .run-note {
    font-size: 0.72rem;
  }

  .run-prompt {
    font-size: 0.86rem;
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .run-calls-summary {
    display: flex;
    gap: 0.4rem;
    flex-wrap: wrap;
  }
  .call-pill {
    font-size: 0.72rem;
    padding: 0.15rem 0.55rem;
    border-radius: 9999px;
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid var(--border-color);
    color: var(--text-secondary);
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    transition: all 0.15s ease;
  }
  .call-pill:hover {
    border-color: rgba(99, 102, 241, 0.35);
    color: #ffffff;
  }

  .run-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .expanded-content {
    border-top: 1px solid var(--border-color);
    padding: 1.35rem;
    background: rgba(0, 0, 0, 0.25);
  }

  .results-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(380px, 1fr));
    gap: 1.25rem;
  }

  .result-card {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.2);
  }
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.8rem 1rem;
    background: var(--bg-card-header);
    border-bottom: 1px solid var(--border-color);
  }
  .card-title-group {
    display: inline-flex;
    align-items: center;
    cursor: default;
    min-width: 0;
  }
  .card-title-row {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    min-width: 0;
  }
  .card-title {
    margin: 0;
    font-size: 0.92rem;
    font-weight: 600;
    letter-spacing: -0.01em;
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .card-tabs {
    display: flex;
    align-items: center;
    padding: 0.35rem 0.75rem;
    background: var(--bg-card-tabs);
    border-bottom: 1px solid var(--border-color);
    gap: 0.35rem;
  }
  .tab-btn {
    background: transparent;
    border: 1px solid transparent;
    font-size: 0.76rem;
    font-weight: 500;
    padding: 0.25rem 0.65rem;
    border-radius: 6px;
    cursor: pointer;
    color: var(--text-muted);
    transition: all 0.15s ease;
  }
  .tab-btn:hover {
    color: var(--text-main);
    background: rgba(255, 255, 255, 0.03);
  }
  .tab-btn.active {
    background: rgba(255, 255, 255, 0.08);
    border-color: rgba(255, 255, 255, 0.06);
    color: #ffffff;
    font-weight: 600;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }
  .md-toggle {
    margin-left: auto;
    font-size: 0.74rem;
    display: flex;
    align-items: center;
    gap: 0.35rem;
    color: var(--text-muted);
    cursor: pointer;
    user-select: none;
  }
  .md-toggle input[type="checkbox"] {
    accent-color: var(--color-primary);
  }

  .card-body {
    flex: 1;
    padding: 1rem 1.1rem;
    min-height: 200px;
    max-height: 480px;
    overflow-y: auto;
    line-height: 1.6;
  }

  .pre-output {
    white-space: pre-wrap;
    word-break: break-word;
    font-family: inherit;
    font-size: 0.88rem;
    line-height: 1.6;
    margin: 0;
    color: var(--text-main);
  }
  .pre-reasoning {
    white-space: pre-wrap;
    word-break: break-word;
    font-family: inherit;
    font-size: 0.82rem;
    line-height: 1.5;
    color: var(--text-muted);
    border-left: 2px solid rgba(99, 102, 241, 0.4);
    background: rgba(99, 102, 241, 0.02);
    padding: 0.6rem 0.8rem;
    border-radius: 0 6px 6px 0;
    margin: 0;
  }
  .pre-raw {
    font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, monospace;
    font-size: 0.78rem;
    line-height: 1.45;
    white-space: pre-wrap;
    word-break: break-all;
    margin: 0;
    color: #cbd5e1;
  }

  .card-metrics {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 0.6rem;
    padding: 0.7rem 1rem;
    background: var(--bg-card-metrics);
    border-top: 1px solid var(--border-color);
  }
  .metric-item {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
  }
  .metric-label {
    font-size: 0.65rem;
    font-weight: 600;
    color: var(--text-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .metric-value {
    font-size: 0.82rem;
    font-weight: 600;
    color: var(--text-main);
    font-variant-numeric: tabular-nums;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  }

  .truncate {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
</style>
