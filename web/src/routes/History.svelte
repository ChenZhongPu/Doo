<script lang="ts">
  import { onMount } from 'svelte';
  import Markdown from '../lib/Markdown.svelte';
  import HistoryPagination from '../lib/HistoryPagination.svelte';
  import { api } from '../lib/api';
  import { t } from '../lib/i18n';
  import { confirmAction, notify } from '../lib/dialog';
  import ProviderIcon from '../lib/ProviderIcon.svelte';
  import Icon from '../lib/Icon.svelte';
  import { cleanProviderName } from '../lib/providerTemplates';
  import type { RunSummary, RunDetail, CallRecord, CallRaw } from '../lib/types';

  let runs: RunSummary[] = $state([]);
  let loading: boolean = $state(true);
  let errorMsg: string = $state('');
  let page = $state(1);
  let total = $state(0);
  let deleting = $state(false);

  let expandedRunId: number | null = $state(null);
  let runDetails: Record<number, RunDetail> = $state({});
  let rawCalls: Record<number, CallRaw> = $state({});
  let activeTabs: Record<number, 'output' | 'reasoning' | 'request' | 'response'> = $state({});
  let renderMdMap: Record<number, boolean> = $state({});

  onMount(() => { void loadRuns(); });

  async function loadRuns(requestedPage = page) {
    loading = true;
    errorMsg = '';
    try {
      const result = await api.listRunsPage(requestedPage);
      runs = result.runs;
      page = result.page;
      total = result.total;
      expandedRunId = null;
      runDetails = {};
      rawCalls = {};
      activeTabs = {};
      renderMdMap = {};
    } catch (e: any) {
      errorMsg = e.message || $t('loadHistoryFailed');
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
        notify(e.message || $t('errorOccurred'), $t('loadDetailsFailed'));
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
    if (deleting || loading) return;
    const run = runs.find(run => run.id === runId);
    if (!await confirmAction({ title: $t('deleteRunTitle'), detail: run ? `${formatTime(run.created_at)} · ${getPromptSnippet(run.input_json)}` : undefined, message: $t('confirmDeleteRun'), destructive: true })) return;
    deleting = true;
    try {
      await api.deleteRun(runId);
      if (expandedRunId === runId) expandedRunId = null;
      await loadRuns();
    } catch (e: any) {
      notify(e.message || $t('errorOccurred'), $t('deleteFailed'));
    } finally {
      deleting = false;
    }
  }

  async function handleDeleteAll() {
    if (deleting || loading || total === 0) return;
    if (!await confirmAction({ title: $t('deleteAllHistoryTitle'), message: $t('confirmDeleteAllHistory'), destructive: true })) return;
    deleting = true;
    try {
      await api.deleteAllRuns();
      await loadRuns(1);
    } catch (e: any) {
      notify(e.message || $t('errorOccurred'), $t('deleteFailed'));
    } finally {
      deleting = false;
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
      notify(e.message || $t('errorOccurred'), $t('loadInputFailed'));
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
  <div class="history-header page-heading">
    <div><span class="page-eyebrow">ACTIVITY</span><h1>{$t('historyTitle')}</h1><p>{$t('historySubtitle')}</p></div>
    <div class="history-header-actions">
      <button class="btn btn-sm btn-outline-danger" disabled={loading || deleting || total === 0} onclick={handleDeleteAll}><Icon name="trash" /> {$t('deleteAllHistory')}</button>
      <button class="btn btn-sm btn-secondary" disabled={loading || deleting} onclick={() => loadRuns()}><Icon name="refresh" /> {$t('refresh')}</button>
    </div>
  </div>

  {#if total > 0}
    <HistoryPagination {page} {total} disabled={loading || deleting} onchange={loadRuns} />
  {/if}

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
            onkeydown={e => { if (e.target === e.currentTarget && (e.key === 'Enter' || e.key === ' ')) { e.preventDefault(); toggleExpand(run.id); } }}
          >
            <div class="run-meta">
              <span class="run-time">{formatTime(run.created_at)}</span>
              <div class="run-tags">
                <span class="run-id">#{run.id}</span>
                {#if run.note}
                  <span class="run-note badge badge-secondary">{run.note}</span>
                {/if}
              </div>
            </div>

            <div class="run-prompt">
              "{getPromptSnippet(run.input_json)}"
            </div>

            <div class="run-calls-summary">
              {#each run.calls as c}
                <span class="badge badge-{c.status} call-pill">
                  {c.label}: {formatMs(c.total_ms)}
                </span>
              {/each}
            </div>

            <div class="run-actions" role="presentation" onclick={e => e.stopPropagation()}>
              <button
                class="btn btn-xs btn-outline"
                onclick={() => handleLoadToCompare(run)}
                data-tooltip={$t('loadToCompare')}
              >
                <Icon name="import" /> {$t('loadToCompare')}
              </button>
              <button
                class="btn btn-xs btn-outline-danger"
                disabled={deleting}
                onclick={() => handleDeleteRun(run.id)}
                aria-label={$t('deleteRun')} data-tooltip={$t('deleteRun')}
              >
                <Icon name="trash" />
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
                            <Markdown text={call.output_text} />
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
                        <span class="metric-value truncate" data-tooltip={call.response_model || ''}>
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
    <HistoryPagination {page} {total} disabled={loading || deleting} onchange={loadRuns} />
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
    flex-wrap: wrap;
    gap: 16px;
  }
  .history-header-actions { display: flex; align-items: center; gap: 8px; }

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
    box-shadow: none;
    transition: all 0.2s ease;
  }
  .run-card.expanded {
    border-color: var(--color-primary-subtle);
    box-shadow: none;
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
    background: var(--bg-hover);
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
  .run-tags { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; }
  .run-id { color: var(--text-muted); font-size: 11px; font-family: ui-monospace, monospace; }

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
    background: var(--bg-hover);
    border: 1px solid var(--border-color);
    color: var(--text-secondary);
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    transition: all 0.15s ease;
  }
  .call-pill:hover {
    border-color: var(--color-primary-subtle);
    color: var(--text-main);
  }

  .run-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .expanded-content {
    border-top: 1px solid var(--border-color);
    padding: 1.35rem;
    background: var(--bg-card-header);
  }

  .results-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(min(100%, 380px), 1fr));
    gap: 1.25rem;
  }

  .result-card {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: none;
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
    background: var(--bg-hover);
  }
  .tab-btn.active {
    background: var(--bg-hover);
    border-color: var(--bg-hover);
    color: var(--text-main);
    font-weight: 600;
    box-shadow: none;
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
    border-left: 2px solid var(--color-primary-subtle);
    background: var(--color-primary-subtle);
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
    color: var(--text-secondary);
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

  .tab-btn.active { background: var(--bg-selected); color: var(--color-primary); border-color: transparent; }
  .run-card.expanded { border-color: var(--border-focus); }
  @media (max-width: 1100px) { .run-summary-bar { grid-template-columns: 1fr; gap: 12px; padding: 20px; } }
  .run-prompt { min-width: 0; overflow-wrap: anywhere; }
  .run-actions { flex-wrap: wrap; }
  .metric-item { min-width: 0; }
  .card-tabs { flex-wrap: wrap; }
  @media (max-width: 600px) { .card-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
