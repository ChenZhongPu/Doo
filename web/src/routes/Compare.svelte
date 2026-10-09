<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import Markdown from '../lib/Markdown.svelte';
  import MessageActions from '../lib/MessageActions.svelte';
  import { api } from '../lib/api';
  import { t } from '../lib/i18n';
  import { notify } from '../lib/dialog';
  import ProviderIcon from '../lib/ProviderIcon.svelte';
  import Icon from '../lib/Icon.svelte';
  import { cleanProviderName } from '../lib/providerTemplates';
  import type { Provider, Target, Message, CallRaw } from '../lib/types';

  interface TargetRunState {
    target: Target;
    provider?: Provider;
    callId?: number;
    status: 'idle' | 'running' | 'done' | 'error' | 'canceled';
    output: string;
    reasoning: string;
    activeTab: 'output' | 'reasoning' | 'request' | 'response';
    renderMarkdown: boolean;
    finishReason?: string;
    httpStatus?: number;
    error?: string;
    usage?: {
      prompt_tokens?: number;
      completion_tokens?: number;
      reasoning_tokens?: number;
    };
    metrics?: {
      ttfb_ms?: number;
      first_token_ms?: number;
      first_content_ms?: number;
      total_ms?: number;
    };
    responseModel?: string;
    raw?: CallRaw;
    loadingRaw?: boolean;
    copyTooltip?: boolean;
  }

  let providers: Provider[] = $state([]);
  let targets: Target[] = $state([]);
  let checkedTargetIds: number[] = $state([]);

  // Prompt inputs
  let systemPrompt: string = $state('');
  let isSystemCollapsed: boolean = $state(false);
  let messages: Message[] = $state([
    { role: 'user', content: '' }
  ]);
  let timeoutS: number = $state(60);
  let note: string = $state('');

  // Execution state
  let isRunning: boolean = $state(false);
  let runElapsedMs: number = $state(0);
  let abortController: AbortController | null = null;
  let targetStates: Record<number, TargetRunState> = $state({});
  let generalError: string = $state('');

  // Modals
  let previewModalOpen: boolean = $state(false);
  let previewData: { targetLabel: string; body: string; headers: Record<string, string> }[] = $state([]);
  let exportModalOpen: boolean = $state(false);
  let exportCode: string = $state('');
  let exportTitle: string = $state('');
  let exportCopied: boolean = $state(false);

  // RAF batch rendering
  let animFrameId: number | null = null;
  let pendingDeltas: { callId: number; kind: 'content' | 'reasoning'; text: string }[] = [];

  let maxTargets: number = $state(8);

  const STORAGE_TARGETS = 'doo_selected_targets';
  const STORAGE_INPUTS = 'doo_saved_inputs';

  onMount(async () => {
    await loadConfig();
    restoreFromStorage();
    try {
      const h = await api.getHealth();
      if (typeof h.max_targets === 'number') {
        maxTargets = h.max_targets;
      }
    } catch {
      // Keep default
    }

    // Listen for custom event from History page
    window.addEventListener('doo-load-run', handleLoadRun as EventListener);
  });

  onDestroy(() => {
    if (abortController) {
      abortController.abort();
    }
    if (animFrameId) {
      cancelAnimationFrame(animFrameId);
    }
    window.removeEventListener('doo-load-run', handleLoadRun as EventListener);
  });

  function getModelTooltip(provName?: string, modelName?: string): string {
    const pLabel = $t('providerLabel');
    const mLabel = $t('modelLabel');
    return `${pLabel}: ${provName || '—'}\n${mLabel}: ${modelName || '—'}`;
  }

  async function loadConfig() {
    try {
      const [provs, tgts] = await Promise.all([api.listProviders(), api.listTargets()]);
      providers = (provs || []).map(p => ({ ...p, name: cleanProviderName(p.name) }));
      targets = tgts || [];

      // Keep checked targets that still exist
      const validIds = new Set(targets.map(t => t.id));
      checkedTargetIds = checkedTargetIds.filter(id => validIds.has(id));
      if (checkedTargetIds.length === 0 && targets.length > 0) {
        checkedTargetIds = [targets[0].id];
      }
    } catch (e: any) {
      generalError = e.message || 'Failed to load config';
    }
  }

  function restoreFromStorage() {
    try {
      const savedTargets = localStorage.getItem(STORAGE_TARGETS);
      if (savedTargets) {
        const ids = JSON.parse(savedTargets);
        if (Array.isArray(ids)) checkedTargetIds = ids;
      }

      const savedInputs = localStorage.getItem(STORAGE_INPUTS);
      if (savedInputs) {
        const inp = JSON.parse(savedInputs);
        if (typeof inp.system === 'string') systemPrompt = inp.system;
        if (Array.isArray(inp.messages) && inp.messages.length > 0) messages = inp.messages;
        if (typeof inp.timeoutS === 'number') timeoutS = inp.timeoutS;
      }
    } catch (e) {
      // Ignore
    }
  }

  function saveToStorage() {
    try {
      localStorage.setItem(STORAGE_TARGETS, JSON.stringify(checkedTargetIds));
      localStorage.setItem(STORAGE_INPUTS, JSON.stringify({
        system: systemPrompt,
        messages,
        timeoutS,
      }));
    } catch (e) {
      // Ignore
    }
  }

  function handleLoadRun(e: CustomEvent) {
    const detail = e.detail;
    if (!detail) return;
    if (detail.input) {
      systemPrompt = detail.input.system || '';
      messages = detail.input.messages || [{ role: 'user', content: '' }];
    }
    if (detail.targetIds && Array.isArray(detail.targetIds)) {
      checkedTargetIds = detail.targetIds.filter((id: number) => targets.some(t => t.id === id));
    }
    saveToStorage();
    window.location.hash = '#/';
  }

  function addMessage() {
    const lastRole = messages.length > 0 ? messages[messages.length - 1].role : 'assistant';
    const nextRole = lastRole === 'user' ? 'assistant' : 'user';
    messages = [...messages, { role: nextRole, content: '' }];
    saveToStorage();
  }

  function removeMessage(index: number) {
    if (messages.length <= 1) return;
    messages = messages.filter((_, i) => i !== index);
    saveToStorage();
  }

  function clearMessage(index: number) {
    messages[index].content = '';
    saveToStorage();
  }

  function moveMessage(index: number, direction: 'up' | 'down') {
    const newIdx = direction === 'up' ? index - 1 : index + 1;
    if (newIdx < 0 || newIdx >= messages.length) return;
    const copy = [...messages];
    const temp = copy[index];
    copy[index] = copy[newIdx];
    copy[newIdx] = temp;
    messages = copy;
    saveToStorage();
  }

  function toggleTarget(id: number) {
    if (checkedTargetIds.includes(id)) {
      checkedTargetIds = checkedTargetIds.filter(x => x !== id);
    } else {
      if (maxTargets > 0 && checkedTargetIds.length >= maxTargets) {
        notify($t('selectUpToN').replace('{n}', String(maxTargets)), $t('selectModels'));
        return;
      }
      checkedTargetIds = [...checkedTargetIds, id];
    }
    saveToStorage();
  }

  async function openPreview() {
    if (checkedTargetIds.length === 0) {
      notify($t('noModelsSelected'), $t('selectModels'));
      return;
    }

    generalError = '';
    previewData = [];
    try {
      const input = { system: systemPrompt, messages };
      for (const tId of checkedTargetIds) {
        const tgt = targets.find(t => t.id === tId);
        if (!tgt) continue;
        const res = await api.preview(tId, input);
        previewData.push({
          targetLabel: tgt.label,
          body: res.body,
          headers: res.headers,
        });
      }
      previewModalOpen = true;
    } catch (e: any) {
      generalError = e.message || 'Preview failed';
    }
  }

  async function handleSend() {
    if (checkedTargetIds.length === 0) {
      notify($t('noModelsSelected'), $t('selectModels'));
      return;
    }
    if (maxTargets > 0 && checkedTargetIds.length > maxTargets) {
      notify($t('selectUpToN').replace('{n}', String(maxTargets)), $t('selectModels'));
      return;
    }

    generalError = '';
    isRunning = true;
    abortController = new AbortController();
    const runStartTime = performance.now();
    runElapsedMs = 0;

    // Initialize target states
    const provMap = new Map(providers.map(p => [p.id, p]));
    const newStates: Record<number, TargetRunState> = {};
    for (const tId of checkedTargetIds) {
      const tgt = targets.find(t => t.id === tId);
      if (!tgt) continue;
      const existing = targetStates[tId];
      newStates[tId] = {
        target: tgt,
        provider: provMap.get(tgt.provider_id),
        status: 'running',
        output: '',
        reasoning: '',
        activeTab: 'output',
        renderMarkdown: existing ? existing.renderMarkdown : true,
      };
    }
    targetStates = newStates;
    saveToStorage();

    const input = { system: systemPrompt, messages };

    // Start RAF loop for fluid stream rendering
    function flushPendingDeltas() {
      if (isRunning) {
        runElapsedMs = performance.now() - runStartTime;
      }
      if (pendingDeltas.length > 0) {
        const map = new Map<number, { content: string; reasoning: string }>();
        for (const d of pendingDeltas) {
          if (!map.has(d.callId)) map.set(d.callId, { content: '', reasoning: '' });
          const entry = map.get(d.callId)!;
          if (d.kind === 'content') entry.content += d.text;
          else if (d.kind === 'reasoning') entry.reasoning += d.text;
        }
        pendingDeltas = [];

        // Apply deltas to matching targetStates
        let modified = false;
        for (const [cId, accumulated] of map.entries()) {
          for (const state of Object.values(targetStates)) {
            if (state.callId === cId) {
              if (accumulated.content) {
                state.output += accumulated.content;
                modified = true;
              }
              if (accumulated.reasoning) {
                state.reasoning += accumulated.reasoning;
                modified = true;
              }
              break;
            }
          }
        }
        if (modified) {
          targetStates = { ...targetStates };
        }
      }
      if (isRunning) {
        animFrameId = requestAnimationFrame(flushPendingDeltas);
      }
    }
    animFrameId = requestAnimationFrame(flushPendingDeltas);

    try {
      const res = await api.startRun(input, checkedTargetIds, timeoutS, note, abortController.signal);
      if (!res.ok) {
        const errObj = await res.json().catch(() => ({}));
        throw new Error(errObj.error || `Run failed: ${res.status}`);
      }

      const reader = res.body?.getReader();
      if (!reader) throw new Error('No readable response body stream');

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { value, done } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const events = buffer.split('\n\n');
        buffer = events.pop() || '';

        for (const rawEvent of events) {
          const trimmed = rawEvent.trim();
          if (!trimmed.startsWith('data:')) continue;
          const jsonStr = trimmed.slice(5).trim();
          if (!jsonStr) continue;

          try {
            const ev = JSON.parse(jsonStr);
            handleSSEEvent(ev);
          } catch (err) {
            console.error('Failed to parse SSE event:', jsonStr, err);
          }
        }
      }
    } catch (e: any) {
      if (e.name === 'AbortError') {
        for (const state of Object.values(targetStates)) {
          if (state.status === 'running') state.status = 'canceled';
        }
      } else {
        generalError = e.message || 'Stream error';
        for (const state of Object.values(targetStates)) {
          if (state.status === 'running') {
            state.status = 'error';
            state.error = e.message;
          }
        }
      }
    } finally {
      isRunning = false;
      runElapsedMs = 0;
      if (animFrameId) cancelAnimationFrame(animFrameId);
      // Flush any leftover deltas
      flushPendingDeltas();
    }
  }

  function handleSSEEvent(ev: any) {
    if (ev.type === 'run') {
      for (const call of ev.calls) {
        const state = targetStates[call.target_id];
        if (state) {
          state.callId = call.call_id;
        }
      }
    } else if (ev.type === 'delta') {
      pendingDeltas.push({ callId: ev.call_id, kind: ev.kind, text: ev.text });
    } else if (ev.type === 'done') {
      for (const state of Object.values(targetStates)) {
        if (state.callId === ev.call_id) {
          state.status = ev.status;
          state.finishReason = ev.finish_reason;
          state.usage = ev.usage;
          state.metrics = ev.metrics;
          state.responseModel = ev.response_model;
          state.httpStatus = ev.http_status;
          if (ev.output && !state.output) {
            state.output = ev.output;
          }
          break;
        }
      }
      targetStates = { ...targetStates };
    } else if (ev.type === 'error') {
      for (const state of Object.values(targetStates)) {
        if (state.callId === ev.call_id) {
          state.status = 'error';
          state.httpStatus = ev.http_status;
          state.error = ev.message;
          state.metrics = ev.metrics;
          if (ev.output) state.output = ev.output;
          break;
        }
      }
    } else if (ev.type === 'end') {
      isRunning = false;
    }
  }

  function handleStopAll() {
    if (abortController) {
      abortController.abort();
    }
  }

  async function handleStopCard(state: TargetRunState) {
    if (state.callId && state.status === 'running') {
      try {
        await api.cancelCall(state.callId);
        state.status = 'canceled';
      } catch (e) {
        console.error('Cancel call error:', e);
      }
    }
  }

  async function switchTab(state: TargetRunState, tab: 'output' | 'reasoning' | 'request' | 'response') {
    state.activeTab = tab;
    if ((tab === 'request' || tab === 'response') && !state.raw && state.callId) {
      state.loadingRaw = true;
      try {
        state.raw = await api.getCallRaw(state.callId);
      } catch (e: any) {
        console.error('Failed to load raw call payload:', e);
      } finally {
        state.loadingRaw = false;
      }
    }
  }

  function copyText(text: string, state?: TargetRunState) {
    navigator.clipboard.writeText(text);
    if (state) {
      state.copyTooltip = true;
      setTimeout(() => { state.copyTooltip = false; }, 2000);
    }
  }

  async function openExport(callId: number, format: 'curl' | 'python') {
    try {
      exportTitle = format === 'curl' ? $t('exportCurl') : $t('exportPython');
      exportCode = await api.exportCall(callId, format);
      exportCopied = false;
      exportModalOpen = true;
    } catch (e: any) {
      notify(e.message || $t('errorOccurred'), $t('exportFailed'));
    }
  }

  function copyExportCode() {
    navigator.clipboard.writeText(exportCode);
    exportCopied = true;
    setTimeout(() => { exportCopied = false; }, 2000);
  }


  function calcSpeed(state: TargetRunState): string {
    const comp = state.usage?.completion_tokens;
    const total = state.metrics?.total_ms;
    const first = state.metrics?.first_token_ms;
    if (comp && comp > 0 && total && first && total > first) {
      const durationSec = (total - first) / 1000;
      if (durationSec > 0) {
        return (comp / durationSec).toFixed(1);
      }
    }
    return '—';
  }

  function formatMs(ms?: number | null): string {
    if (ms == null) return '—';
    if (ms >= 1000) return `${(ms / 1000).toFixed(2)}s`;
    return `${Math.round(ms)}ms`;
  }

  function formatElapsedSec(ms: number): string {
    return `${(Math.max(0, ms) / 1000).toFixed(1)}s`;
  }
</script>

<div class="page-heading"><div><span class="page-eyebrow">WORKBENCH</span><h1>{$t('compareTitle')}</h1><p>{$t('compareSubtitle')}</p></div></div>

<div class="compare-container">
  <!-- Left Sidebar: Models list grouped by provider -->
  <aside class="sidebar">
    <div class="sidebar-header">
      <h3>
        {$t('selectModels')}
        <span
          class="models-count-badge"
          data-tooltip={maxTargets > 0
            ? $t('maxTargetsTooltip').replace('{max}', String(maxTargets))
            : $t('unlimitedTargetsTooltip')}
          data-tooltip-pos="bottom"
        >({checkedTargetIds.length}{maxTargets > 0 ? `/${maxTargets}` : ''})</span>
      </h3>
      <a href="#/config" class="link-config">{$t('goToConfig')} →</a>
    </div>

    {#if targets.length === 0}
      <div class="empty-notice">
        <p>{$t('noModelsConfigured')}</p>
        <a href="#/config" class="btn btn-sm btn-primary">{$t('goToConfig')}</a>
      </div>
    {:else}
      <div class="target-list">
        {#each providers as prov (prov.id)}
          {@const provModels = targets.filter(t => t.provider_id === prov.id)}
          {#if provModels.length > 0}
            <div class="sidebar-prov-group">
              <div class="sidebar-prov-header">
                <ProviderIcon name={prov.name} baseUrl={prov.base_url} protocol={prov.protocol} size={15} />
                <span>{prov.name}</span>
              </div>
              <div class="sidebar-prov-models">
                {#each provModels as tgt (tgt.id)}
                  <label
                    class="target-item"
                    class:checked={checkedTargetIds.includes(tgt.id)}
                    data-tooltip={getModelTooltip(prov.name, tgt.model_name)}
                    data-tooltip-pos="bottom"
                  >
                    <input
                      type="checkbox"
                      checked={checkedTargetIds.includes(tgt.id)}
                      disabled={isRunning}
                      onchange={() => toggleTarget(tgt.id)}
                    />
                    <div class="target-info">
                      <span class="target-label">{tgt.label}</span>
                    </div>
                  </label>
                {/each}
              </div>
            </div>
          {/if}
        {/each}
      </div>
    {/if}
  </aside>

  <!-- Main Content: Inputs & Results -->
  <main class="main-content">
    {#if generalError}
      <div class="alert alert-danger">
        {generalError}
      </div>
    {/if}

    <!-- Top Input Area -->
    <section class="prompt-box">
      <!-- System prompt collapsible -->
      <div class="system-section">
        <button type="button" class="system-header" onclick={() => isSystemCollapsed = !isSystemCollapsed}>
          <span class="heading-with-icon"><Icon name="settings" /> {$t('systemPrompt')}</span>
          <span class="btn-text">
            {isSystemCollapsed ? $t('expand') : $t('collapse')}
          </span>
        </button>
        {#if !isSystemCollapsed}
          <textarea
            class="input-textarea system-textarea"
            placeholder="You are a helpful and concise assistant..."
            bind:value={systemPrompt}
            onchange={saveToStorage}
            disabled={isRunning}
            rows="2"
          ></textarea>
        {/if}
      </div>

      <!-- Messages Editor -->
      <div class="messages-section">
        <div class="messages-header">
          <h4 class="heading-with-icon"><Icon name="message" /> {$t('messages')}</h4>
          <button type="button" class="btn btn-sm" onclick={addMessage} disabled={isRunning}>
            <Icon name="plus" size={14} /> {$t('addMessage')}
          </button>
        </div>

        {#each messages as msg, i (i)}
          <div class="message-row">
            <div class="message-meta">
              <select
                class="role-select"
                bind:value={msg.role}
                onchange={saveToStorage}
                disabled={isRunning}
              >
                <option value="user">{$t('userRole')}</option>
                <option value="assistant">{$t('assistantRole')}</option>
              </select>

              <div class="row-actions">
                <MessageActions text={msg.content} onclear={() => clearMessage(i)} disabled={isRunning} />
                <button
                  type="button"
                  class="btn-icon"
                  disabled={i === 0 || isRunning}
                  onclick={() => moveMessage(i, 'up')}
                  data-tooltip={$t('moveUp')}
                 aria-label={$t('moveUp')}><Icon name="up" size={14} /></button>
                <button
                  type="button"
                  class="btn-icon"
                  disabled={i === messages.length - 1 || isRunning}
                  onclick={() => moveMessage(i, 'down')}
                  data-tooltip={$t('moveDown')}
                 aria-label={$t('moveDown')}><Icon name="down" size={14} /></button>
                <button
                  type="button"
                  class="btn-icon text-danger"
                  disabled={messages.length <= 1 || isRunning}
                  onclick={() => removeMessage(i)}
                  data-tooltip={$t('remove')}
                  data-tooltip-pos="top-right"
                 aria-label={$t('close')}><Icon name="close" size={14} /></button>
              </div>
            </div>

            <textarea
              class="input-textarea msg-textarea"
              placeholder={$t('messagePlaceholder')}
              bind:value={msg.content}
              onchange={saveToStorage}
              disabled={isRunning}
              rows="8"
            ></textarea>
          </div>
        {/each}
      </div>

      <!-- Controls & Actions -->
      <div class="controls-bar">
        <div class="controls-left">
          <button
            type="button"
            class="btn btn-secondary"
            onclick={openPreview}
            disabled={checkedTargetIds.length === 0}
          >
            <Icon name="code" /> {$t('previewBody')}
          </button>

          <label class="timeout-label" data-tooltip={$t('timeoutTooltip')} data-tooltip-pos="bottom">
            <span>{$t('timeoutSec')}:</span>
            <input
              type="number"
              class="input-sm"
              bind:value={timeoutS}
              min="10"
              max="900"
              disabled={isRunning}
              onchange={saveToStorage}
            />
          </label>
        </div>

        <div class="controls-right">
          {#if isRunning}
            <button type="button" class="btn btn-danger" onclick={handleStopAll}>
              <Icon name="stop" /> {$t('stopAll')}
            </button>
          {:else}
            <button
              type="button"
              class="btn btn-primary btn-large"
              onclick={handleSend}
              disabled={checkedTargetIds.length === 0}
            >
              <Icon name="send" /> {$t('send')} ({checkedTargetIds.length})
            </button>
          {/if}
        </div>
      </div>
    </section>

    {#if checkedTargetIds.length === 0}
      <div class="workbench-empty"><Icon name="compare" size={28} /><h3>{$t('compareEmpty')}</h3><p>{$t('compareEmptyHint')}</p></div>
    {/if}

    <!-- Results Cards Grid -->
    <section class="results-grid">
      {#each checkedTargetIds as tId (tId)}
        {@const tgt = targets.find(t => t.id === tId)}
        {@const prov = tgt ? providers.find(p => p.id === tgt.provider_id) : undefined}
        {@const state = targetStates[tId] || {
          target: tgt!,
          provider: prov,
          status: 'idle',
          output: '',
          reasoning: '',
          activeTab: 'output',
          renderMarkdown: true,
        }}

        {#if tgt}
          <div class="card result-card status-{state.status}">
            <!-- Card Header -->
            <div class="card-header">
              <div
                class="card-title-group"
                data-tooltip={getModelTooltip(prov?.name, tgt.model_name)}
                data-tooltip-pos="bottom"
              >
                <div class="card-title-row">
                  {#if prov}
                    <ProviderIcon name={prov.name} baseUrl={prov.base_url} protocol={prov.protocol} size={18} />
                  {/if}
                  <h4 class="card-title">{tgt.label}</h4>
                </div>
              </div>

              <div class="card-header-actions">
                <span class="badge badge-{state.status}">
                  {#if state.status === 'running'}
                    {$t('statusRunning')} ({formatElapsedSec(runElapsedMs)})
                  {:else if state.status === 'done'}
                    {$t('statusDone')}
                  {:else if state.status === 'error'}
                    {$t('statusError')}
                  {:else if state.status === 'canceled'}
                    {$t('statusCanceled')}
                  {:else}
                    {$t('readyToRun')}
                  {/if}
                </span>

                {#if state.status === 'running'}
                  <button
                    type="button"
                    class="btn btn-xs btn-outline-danger"
                    onclick={() => handleStopCard(state)}
                  >
                    {$t('stop')}
                  </button>
                {/if}
              </div>
            </div>

            <!-- Card Tabs -->
            <div class="card-tabs">
              <button
                class="tab-btn"
                class:active={state.activeTab === 'output'}
                onclick={() => switchTab(state, 'output')}
              >
                {$t('tabOutput')}
              </button>

              {#if state.reasoning || state.activeTab === 'reasoning'}
                <button
                  class="tab-btn"
                  class:active={state.activeTab === 'reasoning'}
                  onclick={() => switchTab(state, 'reasoning')}
                >
                  {$t('tabReasoning')}
                </button>
              {/if}

              {#if state.callId}
                <button
                  class="tab-btn"
                  class:active={state.activeTab === 'request'}
                  onclick={() => switchTab(state, 'request')}
                >
                  {$t('tabRequest')}
                </button>
                <button
                  class="tab-btn"
                  class:active={state.activeTab === 'response'}
                  onclick={() => switchTab(state, 'response')}
                >
                  {$t('tabResponse')}
                </button>
              {/if}

              {#if state.activeTab === 'output'}
                <label class="md-toggle">
                  <input type="checkbox" bind:checked={state.renderMarkdown} />
                  <span>{$t('markdownRender')}</span>
                </label>
              {/if}
            </div>

            <!-- Card Body Content -->
            <div class="card-body">
              {#if state.status === 'error' && state.error && state.activeTab === 'output'}
                <div class="error-banner">
                  <strong>{state.httpStatus ? `HTTP ${state.httpStatus}: ` : ''}</strong>
                  {state.error}
                </div>
              {/if}

              {#if state.activeTab === 'output'}
                {#if state.renderMarkdown}
                  <div class="markdown-body">
                    {#if state.output}
                      <Markdown text={state.output} />
                      {#if state.status === 'running'}
                        <span class="streaming-cursor">▊</span>
                      {/if}
                    {:else if state.status === 'running'}
                      <span class="streaming-indicator">...</span>
                    {:else}
                      <div class="output-empty"><span class="empty-rule"></span><p>{$t('outputPlaceholder')}</p></div>
                    {/if}
                  </div>
                {:else}
                  <pre class="pre-output">{state.output}{#if state.status === 'running' && state.output}<span class="streaming-cursor">▊</span>{:else if state.status === 'running'}...{/if}</pre>
                {/if}
              {:else if state.activeTab === 'reasoning'}
                <pre class="pre-reasoning">{state.reasoning}</pre>
              {:else if state.activeTab === 'request'}
                {#if state.loadingRaw}
                  <div class="loading-state">{$t('loadingRaw')}</div>
                {:else if state.raw}
                  <pre class="pre-raw">{state.raw.request_json}</pre>
                {/if}
              {:else if state.activeTab === 'response'}
                {#if state.loadingRaw}
                  <div class="loading-state">{$t('loadingRaw')}</div>
                {:else if state.raw}
                  <pre class="pre-raw">{state.raw.response_raw}</pre>
                {/if}
              {/if}
            </div>

            <!-- Card Bottom Metrics -->
            <div class="card-metrics">
              <div class="metric-item">
                <span class="metric-label">{$t('metricTTFT')}</span>
                <span class="metric-value">{formatMs(state.metrics?.first_token_ms)}</span>
              </div>
              <div class="metric-item">
                <span class="metric-label">{$t('metricFirstContent')}</span>
                <span class="metric-value">{formatMs(state.metrics?.first_content_ms)}</span>
              </div>
              <div class="metric-item">
                <span class="metric-label">{$t('metricTotal')}</span>
                <span class="metric-value">{formatMs(state.metrics?.total_ms)}</span>
              </div>
              <div class="metric-item">
                <span class="metric-label">{$t('metricTokens')}</span>
                <span class="metric-value">
                  {state.usage?.prompt_tokens ?? '—'} / {state.usage?.completion_tokens ?? '—'} / {state.usage?.reasoning_tokens ?? '—'}
                </span>
              </div>
              <div class="metric-item">
                <span class="metric-label">{$t('metricSpeed')}</span>
                <span class="metric-value">{calcSpeed(state)}</span>
              </div>
              <div class="metric-item">
                <span class="metric-label">{$t('metricFinish')}</span>
                <span class="metric-value">{state.finishReason || '—'}</span>
              </div>
              <div class="metric-item">
                <span class="metric-label">{$t('metricModel')}</span>
                <span class="metric-value truncate" title={state.responseModel || ''}>
                  {state.responseModel || '—'}
                </span>
              </div>
            </div>

            <!-- Card Actions -->
            <div class="card-footer">
              <button
                type="button"
                class="btn-footer"
                onclick={() => copyText(state.output, state)}
              >
                <Icon name={state.copyTooltip ? "check" : "copy"} />
                {state.copyTooltip ? $t('copied') : $t('copyOutput')}
              </button>

              {#if state.callId}
                <button
                  type="button"
                  class="btn-footer"
                  onclick={() => openExport(state.callId!, 'curl')}
                >
                  <Icon name="connection" /> {$t('exportCurl')}
                </button>
                <button
                  type="button"
                  class="btn-footer"
                  onclick={() => openExport(state.callId!, 'python')}
                >
                  <Icon name="code" /> {$t('exportPython')}
                </button>
              {/if}
            </div>
          </div>
        {/if}
      {/each}
    </section>
  </main>
</div>

<!-- Modal: Preview Request Body -->
{#if previewModalOpen}
  <div class="modal-backdrop" role="presentation" onclick={e => { if (e.target === e.currentTarget) previewModalOpen = false; }}>
    <div class="modal-dialog" role="dialog" aria-modal="true" tabindex="-1" onkeydown={e => { if (e.key === 'Escape') previewModalOpen = false; }}>
      <div class="modal-header">
        <h3>{$t('previewModalTitle')}</h3>
        <button class="btn-icon" onclick={() => previewModalOpen = false} aria-label={$t('close')}><Icon name="close" size={14} /></button>
      </div>
      <div class="modal-body">
        {#each previewData as item}
          <div class="preview-item">
            <h4>{item.targetLabel}</h4>
            <div class="preview-sub">Headers:</div>
            <pre class="code-preview">{JSON.stringify(item.headers, null, 2)}</pre>
            <div class="preview-sub">Body:</div>
            <pre class="code-preview">{JSON.stringify(JSON.parse(item.body), null, 2)}</pre>
          </div>
        {/each}
      </div>
      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => previewModalOpen = false}>{$t('close')}</button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal: Export Snippet -->
{#if exportModalOpen}
  <div class="modal-backdrop" role="presentation" onclick={e => { if (e.target === e.currentTarget) exportModalOpen = false; }}>
    <div class="modal-dialog" role="dialog" aria-modal="true" tabindex="-1" onkeydown={e => { if (e.key === 'Escape') exportModalOpen = false; }}>
      <div class="modal-header">
        <h3>{exportTitle}</h3>
        <button class="btn-icon" onclick={() => exportModalOpen = false} aria-label={$t('close')}><Icon name="close" size={14} /></button>
      </div>
      <div class="modal-body">
        <pre class="code-preview export-code">{exportCode}</pre>
      </div>
      <div class="modal-footer">
        <button class="btn btn-primary" onclick={copyExportCode}>
          {exportCopied ? $t('copied') : $t('copyCode')}
        </button>
        <button class="btn btn-secondary" onclick={() => exportModalOpen = false}>{$t('close')}</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .compare-container {
    display: flex;
    gap: 1.5rem;
    align-items: flex-start;
  }

  .sidebar {
    width: 280px;
    flex-shrink: 0;
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    padding: 1.25rem;
    box-shadow: none;
  }
  @media (max-width: 900px) {
    .compare-container {
      flex-direction: column;
    }
    .sidebar {
      width: 100%;
    }
    .main-content {
      width: 100%;
    }
  }

  .sidebar-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.6rem;
    margin-bottom: 1.25rem;
    flex-wrap: wrap;
  }
  .sidebar-header h3 {
    font-size: 0.92rem;
    font-weight: 600;
    letter-spacing: -0.01em;
    margin: 0;
    display: inline-flex;
    align-items: center;
    color: var(--text-main);
  }
  .models-count-badge {
    display: inline-flex;
    align-items: center;
    cursor: help;
    font-size: 0.72rem;
    font-weight: 600;
    color: var(--text-muted);
    background: var(--bg-hover);
    border: 1px solid var(--border-color);
    padding: 0.1rem 0.45rem;
    border-radius: 9999px;
    margin-left: 0.45rem;
    line-height: 1.2;
    transition: all 0.15s ease;
  }
  .models-count-badge:hover {
    color: var(--color-primary-light);
    border-color: var(--border-focus);
    background: var(--color-primary-subtle);
  }
  .link-config {
    font-size: 0.78rem;
    font-weight: 500;
    color: var(--color-primary-light);
    text-decoration: none;
    white-space: nowrap;
    flex-shrink: 0;
    transition: color 0.15s ease;
  }
  .link-config:hover {
    color: var(--text-main);
  }

  .empty-notice {
    padding: 1.5rem 1rem;
    text-align: center;
    color: var(--text-muted);
    font-size: 0.82rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.75rem;
  }

  .target-list {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
    max-height: calc(100vh - 220px);
    overflow-y: auto;
    padding-right: 2px;
  }

  .sidebar-prov-group {
    background: var(--bg-hover);
    border: 1px solid var(--border-color);
    border-radius: 9px;
    padding: 0.55rem;
  }
  .sidebar-prov-header {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.75rem;
    font-weight: 600;
    color: var(--text-secondary);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    margin-bottom: 0.45rem;
    padding-bottom: 0.35rem;
    border-bottom: 1px solid var(--border-subtle);
  }
  .sidebar-prov-models {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }

  .target-item {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    padding: 0.48rem 0.6rem;
    border-radius: 6px;
    border: 1px solid transparent;
    cursor: pointer;
    background: var(--bg-hover);
    transition: all 0.15s ease;
    user-select: none;
  }
  .target-item input[type="checkbox"] {
    accent-color: var(--color-primary);
    cursor: pointer;
    border-radius: 4px;
  }
  .target-item:hover {
    background: var(--bg-hover);
    border-color: var(--bg-hover);
  }
  .target-item.checked {
    background: var(--color-primary-subtle);
    border-color: var(--border-focus);
  }

  .target-info {
    display: flex;
    flex-direction: column;
    overflow: hidden;
    min-width: 0;
  }
  .target-label {
    font-size: 0.83rem;
    font-weight: 500;
    color: var(--text-main);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .target-item.checked .target-label {
    color: var(--text-main);
    font-weight: 600;
  }

  .main-content {
    flex: 1;
    min-width: 0;
  }

  .prompt-box {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    padding: 1.25rem;
    margin-bottom: 1.5rem;
    box-shadow: none;
  }

  .system-section {
    margin-bottom: 1rem;
    border-bottom: 1px solid var(--border-subtle);
    padding-bottom: 0.85rem;
  }
  .system-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
    background: none;
    border: none;
    color: var(--text-main);
    padding: 0;
    font-size: 0.82rem;
    font-weight: 600;
    cursor: pointer;
    margin-bottom: 0.5rem;
  }
  .system-header:hover .btn-text {
    color: var(--text-main);
  }
  .system-textarea {
    margin-top: 0.4rem;
  }

  .messages-section {
    margin-bottom: 0.5rem;
  }
  .messages-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.75rem;
  }
  .messages-header h4 {
    margin: 0;
    font-size: 0.88rem;
    font-weight: 600;
    color: var(--text-main);
  }

  .message-row {
    background: var(--bg-hover);
    border: 1px solid var(--border-color);
    border-radius: 9px;
    padding: 0.85rem;
    margin-bottom: 0.75rem;
    transition: all 0.15s ease;
  }
  .message-row:focus-within {
    border-color: var(--border-focus);
    background: var(--bg-hover);
  }
  .message-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 0.55rem;
  }
  .role-select {
    font-size: 0.78rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    padding: 0.25rem 0.65rem;
    border-radius: 6px;
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    color: var(--color-primary-light);
    outline: none;
    cursor: pointer;
    transition: border-color 0.15s ease;
  }
  .role-select:focus {
    border-color: var(--border-focus);
  }
  .row-actions {
    display: flex;
    gap: 0.3rem;
  }

  .input-textarea {
    width: 100%;
    box-sizing: border-box;
    font-family: inherit;
    font-size: 0.88rem;
    line-height: 1.55;
    padding: 0.65rem 0.8rem;
    border-radius: 7px;
    border: 1px solid var(--border-color);
    background: var(--bg-input);
    color: var(--text-main);
    resize: vertical;
    outline: none;
    transition: all 0.15s ease;
  }
  .input-textarea:focus {
    border-color: var(--border-focus);
    box-shadow: none;
  }
  .input-textarea::placeholder {
    color: var(--text-muted);
  }

  .controls-bar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-top: 1.1rem;
    padding-top: 0.95rem;
    border-top: 1px solid var(--border-subtle);
  }
  .controls-left, .controls-right {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .timeout-label {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.8rem;
    color: var(--text-muted);
    cursor: help;
  }
  .input-sm {
    width: 68px;
    padding: 0.3rem 0.5rem;
    border-radius: 6px;
    border: 1px solid var(--border-color);
    background: var(--bg-input);
    color: var(--text-main);
    font-size: 0.82rem;
    font-weight: 500;
    outline: none;
    transition: border-color 0.15s ease;
  }
  .input-sm:focus {
    border-color: var(--border-focus);
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
    transition: border-color 0.2s ease, box-shadow 0.2s ease;
  }
  .result-card.status-running {
    border-color: var(--border-focus);
    box-shadow: none;
  }
  .result-card.status-error {
    border-color: var(--border-danger-subtle);
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
  .card-header-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-shrink: 0;
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
    min-height: 220px;
    max-height: 520px;
    overflow-y: auto;
    line-height: 1.6;
  }

  .streaming-indicator {
    color: var(--text-muted);
    font-style: italic;
  }

  .streaming-cursor {
    display: inline-block;
    color: var(--color-primary-light);
    font-size: 0.9em;
    animation: blink-cursor 0.8s infinite;
    margin-left: 2px;
    vertical-align: baseline;
  }
  @keyframes blink-cursor {
    0%, 100% { opacity: 1; }
    50% { opacity: 0; }
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
  .loading-state {
    color: var(--text-muted);
    font-size: 0.82rem;
    font-style: italic;
  }

  .error-banner {
    background: var(--bg-danger-subtle);
    border: 1px solid var(--border-danger-subtle);
    color: var(--color-danger-light);
    padding: 0.65rem 0.85rem;
    border-radius: 7px;
    font-size: 0.82rem;
    margin-bottom: 0.75rem;
    line-height: 1.45;
  }

  .card-metrics {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 0.6rem;
    padding: 0.7rem 1rem;
    background: var(--bg-card-metrics);
    border-top: 1px solid var(--border-color);
    border-bottom: 1px solid var(--border-color);
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

  .card-footer {
    display: flex;
    gap: 0.5rem;
    padding: 0.45rem 1rem;
    background: var(--bg-card-header);
  }
  .btn-footer {
    background: transparent;
    border: 1px solid transparent;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--color-primary-light);
    cursor: pointer;
    padding: 0.25rem 0.55rem;
    border-radius: 5px;
    transition: all 0.15s ease;
  }
  .btn-footer:hover {
    background: var(--bg-hover);
    border-color: var(--border-subtle);
    color: var(--text-main);
  }

  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: var(--bg-backdrop);
    backdrop-filter: blur(4px);
    -webkit-backdrop-filter: blur(4px);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1000;
  }
  .modal-dialog {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    width: 90%;
    max-width: 760px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: none;
  }
  .modal-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 1.1rem 1.35rem;
    border-bottom: 1px solid var(--border-color);
  }
  .modal-header h3 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 600;
    color: var(--text-main);
  }
  .modal-body {
    padding: 1.35rem;
    overflow-y: auto;
    font-size: 0.85rem;
  }
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.5rem;
    padding: 0.9rem 1.35rem;
    border-top: 1px solid var(--border-color);
  }
  .code-preview {
    font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, monospace;
    font-size: 0.8rem;
    background: var(--bg-code);
    padding: 0.9rem;
    border-radius: 7px;
    border: 1px solid var(--border-color);
    overflow-x: auto;
    color: var(--text-secondary);
  }
  .export-code {
    max-height: 50vh;
  }
  .preview-item {
    margin-bottom: 1.5rem;
  }
  .preview-item:last-child {
    margin-bottom: 0;
  }
  .preview-item h4 {
    margin: 0 0 0.5rem 0;
    font-size: 0.92rem;
    font-weight: 600;
    color: var(--text-main);
  }
  .preview-sub {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: var(--text-muted);
    margin: 0.6rem 0 0.25rem;
  }
  .truncate {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .sidebar-prov-group { background: transparent; border: 0; border-radius: 0; padding: 0; margin-bottom: 16px; }
  .sidebar-prov-header { border: 0; padding: 0 4px 10px; letter-spacing: .06em; font-size: 10px; }
  .target-item { padding: 10px; background: transparent; }
  .target-item.checked { background: var(--bg-selected); border-color: var(--border-focus); }
  .target-label { font-weight: 500; }
  .output-empty { min-height: 180px; display: flex; align-items: center; justify-content: center; flex-direction: column; gap: 10px; color: var(--text-muted); font-size: 12px; }
  .empty-rule { height: 1px; width: 24px; background: var(--border-focus); }
  .workbench-empty { text-align: center; padding: 60px 28px; border: 1px dashed var(--border-color); border-radius: 12px; color: var(--text-muted); }
  .workbench-empty h3 { color: var(--text-secondary); font-size: 15px; font-weight: 500; margin: 16px 0 8px; }
  .workbench-empty p { font-size: 13px; margin: 0; }
  .tab-btn.active { background: var(--bg-selected); color: var(--color-primary); border-color: transparent; }
  .controls-bar { gap: 12px; flex-wrap: wrap; }
  .controls-left { flex-wrap: wrap; }
  .card-tabs { flex-wrap: wrap; }
  .metric-item { min-width: 0; }
  .modal-dialog { box-shadow: var(--shadow-menu); }
  @media (max-width: 600px) { .prompt-box { padding: 16px; } .card-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
