<script lang="ts">
  import { onMount } from 'svelte';
  import { api } from '../lib/api';
  import { t } from '../lib/i18n';
  import ProviderIcon from '../lib/ProviderIcon.svelte';
  import { PROVIDER_TEMPLATES, getProviderDocs, isLocalOrModelsFetchable, resolveMaxTokensField, cleanProviderName, type ProviderTemplate } from '../lib/providerTemplates';
  import type { Provider, Target, CoreParams } from '../lib/types';

  let providers: Provider[] = $state([]);
  let targets: Target[] = $state([]);
  let loading: boolean = $state(true);
  let errorMsg: string = $state('');

  let selectedProviderId: number | null = $state(null);

  // Provider Form State
  let showProviderModal: boolean = $state(false);
  let editingProviderId: number | null = $state(null);
  let pName: string = $state('');
  let pProtocol: 'openai_chat' | 'anthropic' = $state('openai_chat');
  let pBaseUrl: string = $state('https://api.openai.com/v1');
  let pApiKey: string = $state('');
  let showApiKey: boolean = $state(false);
  let pHeadersRaw: string = $state('{}');
  let pHeadersError: string = $state('');
  let pModalError: string = $state('');
  let pTestingId: number | null = $state(null);
  let pTestResult: { ok: boolean; status?: number; time?: number; error?: string } | null = $state(null);

  // Fetch Models from /v1/models state
  let isFetchingModels: boolean = $state(false);
  let fetchedModels: string[] = $state([]);
  let fetchedModelsError: string = $state('');
  let selectedDiscoveredModels: string[] = $state([]);
  let isFetchModelsModalOpen: boolean = $state(false);

  // Target Form State
  let showTargetModal: boolean = $state(false);
  let editingTargetId: number | null = $state(null);
  let tModalError: string = $state('');
  let tLabel: string = $state('');
  let tModelName: string = $state('');
  let tExtraRaw: string = $state('{}');
  let tExtraError: string = $state('');

  // Core Params Checkboxes & Values (keeping only high-value params)
  let enableTemp: boolean = $state(false);
  let valTemp: number = $state(0.7);
  let enableTopP: boolean = $state(false);
  let valTopP: number = $state(1.0);
  let enableMaxTokens: boolean = $state(false);
  let valMaxTokens: number = $state(4096);
  let enableStream: boolean = $state(false);
  let valStream: boolean = $state(true);

  // Preview Modal
  let previewOpen: boolean = $state(false);
  let previewData: { body: string; headers: Record<string, string> } | null = $state(null);

  // Collapsible template section state with localStorage persistence
  let templatesCollapsed: boolean = $state(
    typeof localStorage !== 'undefined' ? localStorage.getItem('doo_tpls_collapsed') === 'true' : false
  );

  function toggleTemplatesCollapse() {
    templatesCollapsed = !templatesCollapsed;
    if (typeof localStorage !== 'undefined') {
      localStorage.setItem('doo_tpls_collapsed', String(templatesCollapsed));
    }
  }

  let activeProvider = $derived(providers.find(p => p.id === selectedProviderId));
  let modelsForActiveProvider = $derived((targets || []).filter(t => t.provider_id === selectedProviderId));
  let activeDocs = $derived(getProviderDocs(activeProvider));
  let canFetchModels = $derived(isLocalOrModelsFetchable(activeProvider));

  onMount(loadData);

  async function loadData() {
    loading = true;
    errorMsg = '';
    try {
      const [provs, tgts] = await Promise.all([api.listProviders(), api.listTargets()]);
      providers = (provs || []).map(p => ({ ...p, name: cleanProviderName(p.name) }));
      targets = tgts || [];
      if (providers.length > 0) {
        if (selectedProviderId === null || !providers.some(p => p.id === selectedProviderId)) {
          selectedProviderId = providers[0].id;
        }
      } else {
        selectedProviderId = null;
      }
    } catch (e: any) {
      errorMsg = e.message || 'Failed to load configuration';
    } finally {
      loading = false;
    }
  }

  function applyProviderTemplate(tpl: ProviderTemplate) {
    pName = tpl.name;
    pProtocol = tpl.protocol;
    pBaseUrl = tpl.baseUrl;
    pApiKey = '';
    pHeadersRaw = tpl.headers ? JSON.stringify(tpl.headers, null, 2) : '{}';
    pHeadersError = '';
    pModalError = '';
  }

  function openCreateProvider(tpl?: ProviderTemplate) {
    editingProviderId = null;
    pApiKey = '';
    showApiKey = false;
    pModalError = '';
    pTestResult = null;
    if (tpl) {
      applyProviderTemplate(tpl);
    } else {
      pName = '';
      pProtocol = 'openai_chat';
      pBaseUrl = 'https://api.openai.com/v1';
      pHeadersRaw = '{}';
      pHeadersError = '';
    }
    showProviderModal = true;
  }

  function editProvider(p: Provider) {
    editingProviderId = p.id;
    pName = p.name;
    pProtocol = p.protocol;
    pBaseUrl = p.base_url;
    pApiKey = '';
    pHeadersRaw = JSON.stringify(p.headers || {}, null, 2);
    pHeadersError = '';
    pModalError = '';
    showApiKey = false;
    pTestResult = null;
    showProviderModal = true;
  }

  async function saveProvider() {
    pModalError = '';
    pHeadersError = '';

    let parsedHeaders = {};
    if (pHeadersRaw.trim()) {
      try {
        parsedHeaders = JSON.parse(pHeadersRaw);
      } catch (e: any) {
        pHeadersError = `Invalid JSON: ${e.message}`;
        return;
      }
    }

    if (!pName.trim()) {
      pModalError = 'Provider name is required';
      return;
    }
    if (!pBaseUrl.trim()) {
      pModalError = 'Base URL is required';
      return;
    }

    const payload: any = {
      name: cleanProviderName(pName.trim()),
      protocol: pProtocol,
      base_url: pBaseUrl.trim(),
      headers: parsedHeaders,
      stream_usage: pProtocol === 'openai_chat',
    };

    if (pApiKey.trim()) {
      payload.api_key = pApiKey.trim();
    } else if (editingProviderId) {
      payload.api_key_ref = 'enc:***';
    } else {
      const lowerUrl = pBaseUrl.toLowerCase();
      const lowerName = pName.toLowerCase();
      if (lowerUrl.includes('localhost') || lowerUrl.includes('127.0.0.1') || lowerName.includes('ollama') || lowerName.includes('lmstudio')) {
        payload.api_key = 'EMPTY';
      } else {
        pModalError = 'API key cannot be empty';
        return;
      }
    }

    try {
      if (editingProviderId) {
        await api.updateProvider(editingProviderId, payload);
      } else {
        const created = await api.createProvider(payload);
        selectedProviderId = created.id;
      }

      showProviderModal = false;
      await loadData();
    } catch (e: any) {
      pModalError = e.message || 'Failed to save provider';
    }
  }

  async function deleteProvider(id: number) {
    if (!confirm($t('confirmDeleteProvider'))) return;
    try {
      await api.deleteProvider(id);
      if (selectedProviderId === id) {
        const remaining = providers.filter(p => p.id !== id);
        selectedProviderId = remaining.length > 0 ? remaining[0].id : null;
      }
      await loadData();
    } catch (e: any) {
      errorMsg = e.message || 'Failed to delete provider';
    }
  }

  async function testProvider(id: number) {
    pTestingId = id;
    pTestResult = null;
    try {
      const res = await api.testProvider(id);
      pTestResult = {
        ok: res.ok,
        status: res.http_status,
        time: res.total_ms,
        error: res.error,
      };
    } catch (e: any) {
      pTestResult = {
        ok: false,
        error: e.message,
      };
    } finally {
      pTestingId = null;
    }
  }

  async function fetchModelsFromProvider(providerId?: number) {
    const pid = providerId || selectedProviderId;
    if (!pid) return;
    isFetchingModels = true;
    fetchedModelsError = '';
    try {
      const res = await api.fetchProviderModels(pid);
      if (res.ok && res.models && res.models.length > 0) {
        fetchedModels = res.models;
        const existingModelNames = new Set(
          (targets || []).filter(t => t.provider_id === pid).map(t => t.model_name)
        );
        selectedDiscoveredModels = fetchedModels.filter(m => !existingModelNames.has(m));
        isFetchModelsModalOpen = true;
      } else if (res.ok && res.models && res.models.length === 0) {
        fetchedModels = [];
        fetchedModelsError = $t('noModelsFound');
        isFetchModelsModalOpen = true;
      } else {
        fetchedModels = [];
        fetchedModelsError = res.error || $t('noModelsFound');
        isFetchModelsModalOpen = true;
      }
    } catch (e: any) {
      fetchedModels = [];
      fetchedModelsError = e.message || 'Failed to fetch models';
      isFetchModelsModalOpen = true;
    } finally {
      isFetchingModels = false;
    }
  }

  async function addSelectedDiscoveredModels() {
    if (!selectedProviderId || selectedDiscoveredModels.length === 0) return;
    try {
      for (const mName of selectedDiscoveredModels) {
        const exists = targets.some(
          t => t.provider_id === selectedProviderId && t.model_name === mName
        );
        if (!exists) {
          await api.createTarget({
            provider_id: selectedProviderId,
            label: mName,
            model_name: mName,
            core: { stream: true },
            extra: {},
            max_tokens_field: resolveMaxTokensField(mName),
          });
        }
      }
      isFetchModelsModalOpen = false;
      await loadData();
    } catch (e: any) {
      fetchedModelsError = e.message || 'Failed to add models';
    }
  }

  // Targets / Models logic
  function openCreateModel(prefillName?: string) {
    if (!selectedProviderId) return;
    editingTargetId = null;
    tModalError = '';
    tLabel = prefillName || '';
    tModelName = prefillName || '';
    tExtraRaw = '{}';
    tExtraError = '';
    enableTemp = false;
    valTemp = 0.7;
    enableTopP = false;
    valTopP = 1.0;
    enableMaxTokens = false;
    valMaxTokens = 4096;
    enableStream = true;
    valStream = true;
    showTargetModal = true;
  }

  function editTarget(t: Target) {
    editingTargetId = t.id;
    tModalError = '';
    tLabel = t.label;
    tModelName = t.model_name;
    tExtraRaw = JSON.stringify(t.extra || {}, null, 2);
    tExtraError = '';

    enableTemp = t.core.temperature != null;
    valTemp = t.core.temperature ?? 0.7;

    enableTopP = t.core.top_p != null;
    valTopP = t.core.top_p ?? 1.0;

    enableMaxTokens = t.core.max_tokens != null;
    valMaxTokens = t.core.max_tokens ?? 4096;

    enableStream = true;
    valStream = t.core.stream ?? true;

    showTargetModal = true;
  }

  async function saveTarget() {
    if (!selectedProviderId) return;
    tModalError = '';
    tExtraError = '';

    if (!tLabel.trim() || !tModelName.trim()) {
      tModalError = 'Label and Model Name are required';
      return;
    }

    let parsedExtra = {};
    if (tExtraRaw.trim()) {
      try {
        parsedExtra = JSON.parse(tExtraRaw);
      } catch (e: any) {
        tExtraError = `Invalid JSON: ${e.message}`;
        return;
      }
    }

    const core: CoreParams = {};
    if (enableTemp && valTemp != null && !isNaN(Number(valTemp))) {
      core.temperature = Number(valTemp);
    }
    if (enableTopP && valTopP != null && !isNaN(Number(valTopP))) {
      core.top_p = Number(valTopP);
    }
    if (enableMaxTokens && valMaxTokens != null && !isNaN(Number(valMaxTokens))) {
      core.max_tokens = Math.round(Number(valMaxTokens));
    }
    if (enableStream) {
      core.stream = valStream;
    }

    if (activeProvider?.protocol === 'anthropic') {
      if (!enableMaxTokens) {
        tModalError = 'Anthropic requires max_tokens to be set.';
        return;
      }
    }

    const payload: Partial<Target> = {
      provider_id: selectedProviderId,
      label: tLabel.trim(),
      model_name: tModelName.trim(),
      core,
      extra: parsedExtra,
      max_tokens_field: resolveMaxTokensField(tModelName.trim()),
    };

    try {
      if (editingTargetId) {
        await api.updateTarget(editingTargetId, payload);
      } else {
        await api.createTarget(payload);
      }
      showTargetModal = false;
      await loadData();
    } catch (e: any) {
      tModalError = e.message || 'Failed to save model preset';
    }
  }

  async function deleteTarget(id: number) {
    if (!confirm($t('confirmDeleteTarget'))) return;
    try {
      await api.deleteTarget(id);
      await loadData();
    } catch (e: any) {
      errorMsg = e.message || 'Failed to delete model preset';
    }
  }

  async function duplicateTarget(id: number) {
    try {
      await api.duplicateTarget(id);
      await loadData();
    } catch (e: any) {
      errorMsg = e.message || 'Failed to duplicate model preset';
    }
  }

  async function previewTargetReq(targetId: number) {
    try {
      const res = await api.preview(targetId, {
        messages: [{ role: 'user', content: 'Hello, testing prompt.' }]
      });
      previewData = res;
      previewOpen = true;
    } catch (e: any) {
      alert(`Preview failed: ${e.message}`);
    }
  }
</script>

<div class="config-page">
  {#if errorMsg}
    <div class="alert alert-danger">{errorMsg}</div>
  {/if}

  <!-- ROW 1: Quick Templates (Collapsible & Multi-line Adaptive Wrap) -->
  <div class="panel templates-section" class:is-collapsed={templatesCollapsed}>
    <div class="section-header">
      <div class="section-title-group" onclick={toggleTemplatesCollapse} role="button" tabindex="0" onkeydown={e => { if (e.key === 'Enter' || e.key === ' ') toggleTemplatesCollapse(); }}>
        <span class="collapse-icon">{templatesCollapsed ? '▸' : '▾'}</span>
        <svg class="section-title-svg" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <rect x="3" y="3" width="7" height="7" rx="1.5"/>
          <rect x="14" y="3" width="7" height="7" rx="1.5"/>
          <rect x="14" y="14" width="7" height="7" rx="1.5"/>
          <rect x="3" y="14" width="7" height="7" rx="1.5"/>
        </svg>
        <h3>{$t('providerTemplates')}</h3>
        <span class="section-hint">{$t('navCompare') === '对比' ? '点击快速添加' : 'Click to quickly add'}</span>
      </div>
      <button
        type="button"
        class="btn-collapse"
        onclick={toggleTemplatesCollapse}
        title={templatesCollapsed ? $t('expand') : $t('collapse')}
      >
        {templatesCollapsed ? `▾ ${$t('expand')}` : `▴ ${$t('collapse')}`}
      </button>
    </div>

    {#if !templatesCollapsed}
      <div class="templates-list-adaptive">
        {#each PROVIDER_TEMPLATES as tpl}
          <button
            type="button"
            class="tpl-btn"
            onclick={() => openCreateProvider(tpl)}
          >
            <ProviderIcon brand={tpl.brand} size={16} />
            <span>{tpl.name}</span>
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <!-- ROW 2: 2-Column Layout (Left: Configured Providers Sidebar, Right: Model Presets) -->
  <div class="layout-2col">
    <!-- Left Column: Configured Providers (Non-collapsible Sidebar) -->
    <div class="panel providers-panel">
      <div class="section-header">
        <div class="section-title-group">
          <svg class="section-title-svg" viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
            <circle cx="12" cy="12" r="10"/>
            <path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/>
            <path d="M2 12h20"/>
          </svg>
          <h3>{$t('providersTitle')}</h3>
        </div>
      </div>

      {#if providers.length === 0}
        <div class="empty-inline-notice">
          <p>{$t('navCompare') === '对比' ? '暂未添加任何服务商，可点击上方「常用厂商模板」快速添加' : 'No providers added yet. Click a template above to get started.'}</p>
        </div>
      {:else}
        <div class="providers-list">
          {#each providers as prov (prov.id)}
            {@const modelCount = targets.filter(t => t.provider_id === prov.id).length}
            {@const isSelected = selectedProviderId === prov.id}
            <div
              class="provider-item"
              class:selected={isSelected}
              onclick={() => selectedProviderId = prov.id}
              role="button"
              tabindex="0"
              onkeydown={e => { if (e.key === 'Enter' || e.key === ' ') selectedProviderId = prov.id; }}
            >
              <div class="provider-item-top">
                <div class="provider-brand">
                  <ProviderIcon name={prov.name} baseUrl={prov.base_url} protocol={prov.protocol} size={18} />
                  <span class="provider-name" title={prov.name}>{prov.name}</span>
                </div>
                <div class="provider-badges">
                  {#if isSelected}
                    <span class="badge badge-active" title="Selected">✓</span>
                  {/if}
                  <span class="badge badge-secondary">{modelCount} {$t('navCompare') === '对比' ? '模型' : 'models'}</span>
                </div>
              </div>

              <div class="provider-item-sub">
                <span class="protocol-tag">{prov.protocol}</span>
                <span class="url-text truncate" title={prov.base_url}>{prov.base_url}</span>
              </div>

              <div class="provider-item-actions" role="presentation" onclick={e => e.stopPropagation()}>
                <button
                  class="btn btn-xs btn-outline"
                  onclick={() => testProvider(prov.id)}
                  disabled={pTestingId === prov.id}
                  title={$t('testConnection')}
                >
                  {pTestingId === prov.id ? $t('testing') : `⚡ ${$t('testConnection')}`}
                </button>
                <button class="btn btn-xs btn-secondary" onclick={() => editProvider(prov)} title={$t('editProvider')}>✏️</button>
                <button class="btn btn-xs btn-danger" onclick={() => deleteProvider(prov.id)} title={$t('delete')}>✕</button>
              </div>
            </div>
          {/each}
        </div>
      {/if}

      {#if pTestResult}
        <div class="test-result-bar" class:success={pTestResult.ok} class:failed={!pTestResult.ok}>
          <strong>{pTestResult.ok ? `✓ ${$t('testSuccess')}` : `✗ ${$t('testFailed')}`}</strong>
          {#if pTestResult.time}
            <span>({Math.round(pTestResult.time)}ms)</span>
          {/if}
          {#if pTestResult.error}
            <span class="test-err-msg">{pTestResult.error}</span>
          {/if}
        </div>
      {/if}
    </div>

    <!-- Right Column: Models under selected provider -->
    <div class="panel models-panel">
      {#if activeProvider}
        <div class="panel-header">
          <div class="active-prov-info">
            <ProviderIcon name={activeProvider.name} baseUrl={activeProvider.base_url} protocol={activeProvider.protocol} size={24} />
            <div>
              <div class="active-prov-title-row">
                <h3><span class="highlight-name">{activeProvider.name}</span></h3>
                {#each activeDocs as doc}
                  <a
                    href={doc.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="doc-link-badge"
                    data-tooltip={`${$t('viewOfficialDocs')}: ${doc.title}`}
                  >
                    📖 {doc.title || $t('officialDocs')} ↗
                  </a>
                {/each}
              </div>
              <span class="active-prov-sub">{activeProvider.base_url} • {activeProvider.protocol}</span>
            </div>
          </div>

          <div class="panel-header-actions">
            {#if canFetchModels}
              <button
                class="btn btn-sm btn-outline btn-fetch-models"
                onclick={() => fetchModelsFromProvider()}
                disabled={isFetchingModels}
                data-tooltip={$t('fetchModels')}
              >
                {#if isFetchingModels}
                  ⏳ {$t('fetchingModels')}
                {:else}
                  🔄 {$t('fetchModels')}
                {/if}
              </button>
            {/if}
            <button class="btn btn-sm btn-primary" onclick={() => openCreateModel()}>
              + {$t('addTarget')}
            </button>
          </div>
        </div>

        <!-- Models Grid / List under this Provider -->
        {#if modelsForActiveProvider.length === 0}
          <div class="empty-panel">
            <p>{$t('noModelsUnderProvider')}</p>
            <div class="empty-actions">
              {#if canFetchModels}
                <button
                  class="btn btn-primary"
                  onclick={() => fetchModelsFromProvider()}
                  disabled={isFetchingModels}
                >
                  {#if isFetchingModels}
                    ⏳ {$t('fetchingModels')}
                  {:else}
                    🔄 {$t('fetchModels')}
                  {/if}
                </button>
              {/if}
              <button class="btn {canFetchModels ? 'btn-secondary' : 'btn-primary'}" onclick={() => openCreateModel()}>
                + {$t('addTarget')}
              </button>
              {#each activeDocs as doc}
                <a href={doc.url} target="_blank" rel="noopener noreferrer" class="btn btn-outline doc-btn">
                  📖 {doc.title} ↗
                </a>
              {/each}
            </div>
          </div>
        {:else}
          <div class="models-grid">
            {#each modelsForActiveProvider as tgt (tgt.id)}
              <div class="model-card">
                <div class="model-card-header">
                  <div class="model-card-top-row">
                    <h4 class="model-card-label" title={tgt.label}>{tgt.label}</h4>
                    <div class="model-card-actions">
                      <button class="btn btn-xs btn-outline" onclick={() => previewTargetReq(tgt.id)} data-tooltip={$t('previewBody')}>
                        🔍
                      </button>
                      <button class="btn btn-xs btn-secondary" onclick={() => editTarget(tgt)} data-tooltip={$t('editTarget')}>
                        ✏️
                      </button>
                      <button class="btn btn-xs btn-danger" onclick={() => deleteTarget(tgt.id)} data-tooltip={$t('deleteTarget')} data-tooltip-pos="top-right">
                        ✕
                      </button>
                    </div>
                  </div>

                  <div class="model-card-sub">
                    <span class="sub-key">model_name:</span>
                    <code class="model-name-code">{tgt.model_name}</code>
                  </div>
                </div>

                <div class="model-card-body">
                  <div class="param-tags">
                    {#if tgt.core.temperature != null}
                      <span class="param-tag">temp: {tgt.core.temperature}</span>
                    {/if}
                    {#if tgt.core.top_p != null}
                      <span class="param-tag">top_p: {tgt.core.top_p}</span>
                    {/if}
                    {#if tgt.core.max_tokens != null}
                      <span class="param-tag">max_tokens: {tgt.core.max_tokens}</span>
                    {/if}
                    {#if tgt.core.stream != null}
                      <span class="param-tag">stream: {tgt.core.stream}</span>
                    {/if}
                    {#if tgt.max_tokens_field && tgt.max_tokens_field !== 'max_tokens'}
                      <span class="param-tag field-tag">{tgt.max_tokens_field}</span>
                    {/if}
                  </div>

                  {#if tgt.extra && Object.keys(tgt.extra).length > 0}
                    <div class="extra-summary">
                      <span class="extra-label">extra:</span>
                      <code>{JSON.stringify(tgt.extra)}</code>
                    </div>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      {:else}
        <div class="empty-panel">
          <p>{$t('selectProviderFirst')}</p>
        </div>
      {/if}
    </div>
  </div>
</div>

<!-- Modal: Create / Edit Provider -->
{#if showProviderModal}
  <div class="modal-backdrop" role="presentation" onclick={() => showProviderModal = false}>
    <div class="modal-dialog" role="dialog" aria-modal="true" tabindex="-1" onclick={e => e.stopPropagation()}>
      <div class="modal-header">
        <h3>{editingProviderId ? $t('editProvider') : $t('addProvider')}</h3>
        <button class="btn-icon" onclick={() => showProviderModal = false}>×</button>
      </div>

      <div class="modal-body">
        {#if pModalError}
          <div class="modal-alert-error">
            ⚠️ {pModalError}
          </div>
        {/if}

        {#if !editingProviderId}
          <div class="form-group mb-3">
            <span class="templates-label">{$t('providerTemplates')}:</span>
            <div class="templates-list-compact">
              {#each PROVIDER_TEMPLATES as tpl}
                <button
                  type="button"
                  class="tpl-pill"
                  onclick={() => applyProviderTemplate(tpl)}
                >
                  <ProviderIcon brand={tpl.brand} size={14} />
                  <span>{tpl.name}</span>
                </button>
              {/each}
            </div>
          </div>
        {/if}

        <div class="form-grid">
          <div class="form-group">
            <label for="pName">{$t('providerName')}:</label>
            <input id="pName" type="text" class="input-text" bind:value={pName} placeholder="DeepSeek / OpenRouter / OpenAI" />
          </div>

          <div class="form-group">
            <label for="pProtocol">{$t('protocol')}:</label>
            <select id="pProtocol" class="input-select" bind:value={pProtocol}>
              <option value="openai_chat">OpenAI Chat (Compatible)</option>
              <option value="anthropic">Anthropic Messages</option>
            </select>
          </div>

          <div class="form-group span-2">
            <label for="pBaseUrl">{$t('baseUrl')}:</label>
            <input id="pBaseUrl" type="text" class="input-text" bind:value={pBaseUrl} />
          </div>

          <div class="form-group span-2">
            <label for="pApiKey">{$t('apiKey')}:</label>
            <div class="password-input-group">
              <input
                id="pApiKey"
                type={showApiKey ? 'text' : 'password'}
                class="input-text"
                bind:value={pApiKey}
                placeholder={editingProviderId ? '•••••••• (' + $t('leaveEmptyToKeep') + ')' : $t('apiKeyPlaceholder')}
              />
              <button
                type="button"
                class="btn-toggle-visibility"
                onclick={() => showApiKey = !showApiKey}
                data-tooltip={showApiKey ? $t('hideApiKey') : $t('showApiKey')}
                aria-label={showApiKey ? $t('hideApiKey') : $t('showApiKey')}
              >
                {#if showApiKey}
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19m-6.72-1.07a3 3 0 1 1-4.24-4.24"></path>
                    <line x1="1" y1="1" x2="23" y2="23"></line>
                  </svg>
                {:else}
                  <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8z"></path>
                    <circle cx="12" cy="12" r="3"></circle>
                  </svg>
                {/if}
              </button>
            </div>
          </div>



          <div class="form-group span-2">
            <label for="pHeaders">{$t('headersJson')}:</label>
            <textarea id="pHeaders" class="input-textarea" rows="2" bind:value={pHeadersRaw}></textarea>
            {#if pHeadersError}
              <span class="inline-error">{pHeadersError}</span>
            {/if}
          </div>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => showProviderModal = false}>{$t('cancel')}</button>
        <button class="btn btn-primary" onclick={saveProvider}>{$t('save')}</button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal: Create / Edit Target / Model Preset -->
{#if showTargetModal}
  <div class="modal-backdrop" role="presentation" onclick={() => showTargetModal = false}>
    <div class="modal-dialog modal-dialog-lg" role="dialog" aria-modal="true" tabindex="-1" onclick={e => e.stopPropagation()}>
      <div class="modal-header">
        <h3>{editingTargetId ? $t('editTarget') : $t('addTarget')} ({activeProvider?.name})</h3>
        <button class="btn-icon" onclick={() => showTargetModal = false}>×</button>
      </div>

      <div class="modal-body">
        {#if tModalError}
          <div class="modal-alert-error">
            ⚠️ {tModalError}
          </div>
        {/if}

        <div class="form-grid">
          <div class="form-group">
            <label for="tLabel">{$t('targetLabel')}:</label>
            <input id="tLabel" type="text" class="input-text" bind:value={tLabel} placeholder="deepseek-v3 / gpt-4o-pilot" />
          </div>

          <div class="form-group">
            <div class="label-with-doc">
              <label for="tModel">{$t('modelName')}:</label>
              <div class="model-label-actions">
                {#each activeDocs as doc}
                  <a
                    href={doc.url}
                    target="_blank"
                    rel="noopener noreferrer"
                    class="doc-link-small"
                    data-tooltip={`${$t('viewOfficialDocs')}: ${doc.title}`}
                  >
                    📖 {doc.title} ↗
                  </a>
                {/each}
              </div>
            </div>
            <input
              id="tModel"
              type="text"
              class="input-text"
              bind:value={tModelName}
              placeholder="e.g. deepseek-chat / gpt-4o"
            />
          </div>

          <!-- Core Params (directly editable; typing into them auto-enables the parameter) -->
          <div class="form-group span-2">
            <span class="group-label">{$t('coreParams')}:</span>
            <div class="core-grid">
              <div class="core-param-row" class:active={enableTemp}>
                <label class="core-check">
                  <input type="checkbox" bind:checked={enableTemp} />
                  <span>{$t('tempParam')}</span>
                </label>
                <input
                  type="number"
                  step="0.1"
                  min="0"
                  max="2"
                  class="input-sm"
                  bind:value={valTemp}
                  oninput={() => enableTemp = true}
                  placeholder="0.7"
                />
              </div>

              <div class="core-param-row" class:active={enableTopP}>
                <label class="core-check">
                  <input type="checkbox" bind:checked={enableTopP} />
                  <span>{$t('topPParam')}</span>
                </label>
                <input
                  type="number"
                  step="0.05"
                  min="0"
                  max="1"
                  class="input-sm"
                  bind:value={valTopP}
                  oninput={() => enableTopP = true}
                  placeholder="1.0"
                />
              </div>

              <div class="core-param-row" class:active={enableMaxTokens}>
                <label class="core-check">
                  <input type="checkbox" bind:checked={enableMaxTokens} />
                  <span>{$t('maxTokensParam')}</span>
                </label>
                <input
                  type="number"
                  min="1"
                  class="input-sm"
                  bind:value={valMaxTokens}
                  oninput={() => enableMaxTokens = true}
                  placeholder="4096"
                />
              </div>

              <div class="core-param-row" class:active={enableStream}>
                <label class="core-check">
                  <input type="checkbox" bind:checked={enableStream} />
                  <span>{$t('streamParam')}</span>
                </label>
                <select
                  class="input-sm select-sm"
                  bind:value={valStream}
                  onchange={() => enableStream = true}
                >
                  <option value={true}>true</option>
                  <option value={false}>false</option>
                </select>
              </div>
            </div>
          </div>

          <!-- Extra JSON -->
          <div class="form-group span-2">
            <label for="tExtra">{$t('extraJson')} ({$t('extraJsonHint')}):</label>
            <textarea id="tExtra" class="input-textarea" rows="2" bind:value={tExtraRaw}></textarea>
            {#if tExtraError}
              <span class="inline-error">{tExtraError}</span>
            {/if}
          </div>
        </div>
      </div>

      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => showTargetModal = false}>{$t('cancel')}</button>
        <button class="btn btn-primary" onclick={saveTarget}>{$t('save')}</button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal: Preview Request Body -->
{#if previewOpen && previewData}
  <div class="modal-backdrop" role="presentation" onclick={() => previewOpen = false}>
    <div class="modal-dialog" role="dialog" aria-modal="true" tabindex="-1" onclick={e => e.stopPropagation()}>
      <div class="modal-header">
        <h3>{$t('previewModalTitle')}</h3>
        <button class="btn-icon" onclick={() => previewOpen = false}>×</button>
      </div>
      <div class="modal-body">
        <div class="preview-sub">Headers:</div>
        <pre class="code-preview">{JSON.stringify(previewData.headers, null, 2)}</pre>
        <div class="preview-sub">Body:</div>
        <pre class="code-preview">{JSON.stringify(JSON.parse(previewData.body), null, 2)}</pre>
      </div>
      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => previewOpen = false}>{$t('close')}</button>
      </div>
    </div>
  </div>
{/if}

<!-- Modal: Discover / Select Models from /v1/models -->
{#if isFetchModelsModalOpen}
  <div class="modal-backdrop" role="presentation" onclick={() => isFetchModelsModalOpen = false}>
    <div class="modal-dialog" role="dialog" aria-modal="true" tabindex="-1" onclick={e => e.stopPropagation()}>
      <div class="modal-header">
        <div class="modal-title-with-icon">
          <ProviderIcon name={activeProvider?.name} baseUrl={activeProvider?.base_url} protocol={activeProvider?.protocol} size={20} />
          <h3>{$t('discoveredModels')} ({activeProvider?.name})</h3>
        </div>
        <button class="btn-icon" onclick={() => isFetchModelsModalOpen = false}>×</button>
      </div>

      <div class="modal-body">
        {#if fetchedModelsError}
          <div class="modal-alert-error">
            ⚠️ {fetchedModelsError}
          </div>
        {/if}

        {#if fetchedModels.length > 0}
          <div class="selection-controls">
            <button
              type="button"
              class="btn-text-sm"
              onclick={() => selectedDiscoveredModels = [...fetchedModels]}
            >
              {$t('selectAll')}
            </button>
            <span class="ctrl-sep">•</span>
            <button
              type="button"
              class="btn-text-sm"
              onclick={() => selectedDiscoveredModels = []}
            >
              {$t('deselectAll')}
            </button>
            <span class="selection-summary">({selectedDiscoveredModels.length}/{fetchedModels.length})</span>
          </div>

          <div class="discovered-models-list">
            {#each fetchedModels as modelName}
              {@const alreadyAdded = (targets || []).some(t => t.provider_id === selectedProviderId && t.model_name === modelName)}
              <div class="discovered-model-item" class:already-added={alreadyAdded}>
                <label class="discovered-model-check-label">
                  <input
                    type="checkbox"
                    checked={selectedDiscoveredModels.includes(modelName)}
                    disabled={alreadyAdded}
                    onchange={e => {
                      if (e.currentTarget.checked) {
                        selectedDiscoveredModels = [...selectedDiscoveredModels, modelName];
                      } else {
                        selectedDiscoveredModels = selectedDiscoveredModels.filter(m => m !== modelName);
                      }
                    }}
                  />
                  <span class="model-id-text">{modelName}</span>
                </label>
                {#if alreadyAdded}
                  <span class="already-added-tag">{$t('alreadyAdded')}</span>
                {:else}
                  <button
                    type="button"
                    class="btn btn-xs btn-outline"
                    onclick={() => {
                      isFetchModelsModalOpen = false;
                      openCreateModel(modelName);
                    }}
                    data-tooltip={$t('customizeAndAdd')}
                  >
                    ✏️ {$t('customizeAndAdd')}
                  </button>
                {/if}
              </div>
            {/each}
          </div>
        {:else if !fetchedModelsError}
          <p class="text-muted">{$t('noModelsFound')}</p>
        {/if}
      </div>

      <div class="modal-footer">
        <button class="btn btn-secondary" onclick={() => isFetchModelsModalOpen = false}>{$t('cancel')}</button>
        {#if fetchedModels.length > 0}
          <button
            class="btn btn-primary"
            disabled={selectedDiscoveredModels.length === 0}
            onclick={addSelectedDiscoveredModels}
          >
            + {$t('addSelectedModels')} ({selectedDiscoveredModels.length})
          </button>
        {/if}
      </div>
    </div>
  </div>
{/if}

<style>
  .config-page {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }

  /* Section Panel Base */
  .panel {
    background: var(--bg-card);
    border: 1px solid var(--border-color);
    border-radius: 12px;
    padding: 1.15rem;
    display: flex;
    flex-direction: column;
    box-shadow: 0 4px 24px rgba(0, 0, 0, 0.2);
    transition: all 0.2s ease;
  }
  .panel.is-collapsed {
    padding-bottom: 0.85rem;
  }

  /* Section Headers */
  .section-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
  }
  .section-title-group {
    display: flex;
    align-items: center;
    gap: 0.55rem;
    user-select: none;
    white-space: nowrap;
    min-width: 0;
  }
  .section-title-group h3 {
    margin: 0;
    font-size: 0.98rem;
    font-weight: 700;
    letter-spacing: -0.01em;
    white-space: nowrap;
    color: var(--text-main);
  }
  .section-hint {
    font-size: 0.75rem;
    color: var(--text-muted);
    font-weight: normal;
  }
  .section-title-svg {
    color: var(--color-primary-light);
    flex-shrink: 0;
  }
  .btn-collapse {
    background: transparent;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    padding: 0.22rem 0.6rem;
    font-size: 0.75rem;
    font-weight: 500;
    color: var(--text-muted);
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .btn-collapse:hover {
    color: var(--text-main);
    border-color: var(--border-focus);
    background: rgba(255, 255, 255, 0.04);
  }

  /* Row 1: Templates List (Adaptive Wrap) */
  .templates-section {
    padding: 0.95rem 1.15rem;
  }
  .templates-list-adaptive {
    display: flex;
    flex-wrap: wrap;
    gap: 0.5rem;
    margin-top: 0.9rem;
  }
  .tpl-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.4rem 0.85rem;
    font-size: 0.8rem;
    font-weight: 500;
    background: rgba(255, 255, 255, 0.025);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    border-radius: 8px;
    cursor: pointer;
    white-space: nowrap;
    transition: all 0.15s ease;
  }
  .tpl-btn:hover {
    border-color: rgba(99, 102, 241, 0.35);
    background: rgba(99, 102, 241, 0.08);
    color: #ffffff;
    transform: translateY(-1px);
  }

  /* ROW 2: 2-Column Layout */
  .layout-2col {
    display: grid;
    grid-template-columns: 320px 1fr;
    gap: 1.25rem;
    align-items: start;
  }
  @media (max-width: 900px) {
    .layout-2col {
      grid-template-columns: 1fr;
    }
  }

  /* Left Column: Providers Sidebar */
  .providers-panel {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .providers-list {
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    max-height: calc(100vh - 280px);
    overflow-y: auto;
    margin-top: 0.85rem;
    padding-right: 2px;
  }
  .provider-item {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border-color);
    border-radius: 10px;
    padding: 0.8rem 0.9rem;
    cursor: pointer;
    transition: all 0.15s ease;
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
  }
  .provider-item:hover {
    background: rgba(255, 255, 255, 0.045);
    border-color: rgba(255, 255, 255, 0.12);
  }
  .provider-item.selected {
    border-color: rgba(99, 102, 241, 0.45);
    background: rgba(99, 102, 241, 0.08);
    box-shadow: 0 0 16px rgba(99, 102, 241, 0.12);
  }
  .provider-item-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.4rem;
  }
  .provider-brand {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    min-width: 0;
  }
  .provider-name {
    font-weight: 600;
    font-size: 0.88rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    color: var(--text-main);
  }
  .provider-badges {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    flex-shrink: 0;
  }
  .badge-active {
    background: var(--color-primary);
    color: #fff;
    font-size: 0.68rem;
    font-weight: 700;
    padding: 0.05rem 0.35rem;
    border-radius: 9999px;
  }
  .provider-item-sub {
    display: flex;
    align-items: center;
    gap: 0.45rem;
    font-size: 0.75rem;
    color: var(--text-muted);
  }
  .protocol-tag {
    background: rgba(0, 0, 0, 0.35);
    border: 1px solid var(--border-subtle);
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
    font-size: 0.7rem;
    flex-shrink: 0;
    color: var(--text-secondary);
  }
  .url-text {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .provider-item-actions {
    display: flex;
    align-items: center;
    gap: 0.35rem;
    margin-top: 0.2rem;
  }
  .empty-inline-notice {
    padding: 1.5rem;
    text-align: center;
    color: var(--text-muted);
    font-size: 0.82rem;
    background: rgba(255, 255, 255, 0.015);
    border-radius: 8px;
    border: 1px dashed var(--border-color);
    margin-top: 0.85rem;
  }

  /* Right Column: Models Panel */
  .models-panel {
    min-height: 400px;
    min-width: 0;
  }
  .panel-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 1.1rem;
    padding-bottom: 0.85rem;
    border-bottom: 1px solid var(--border-subtle);
  }
  .panel-header h3 {
    margin: 0;
    font-size: 1.05rem;
    font-weight: 700;
    letter-spacing: -0.01em;
  }
  .active-prov-info {
    display: flex;
    align-items: center;
    gap: 0.65rem;
  }
  .highlight-name {
    color: var(--color-primary-light);
  }
  .active-prov-sub {
    font-size: 0.75rem;
    color: var(--text-muted);
  }

  .active-prov-title-row {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    flex-wrap: wrap;
  }
  .doc-link-badge {
    display: inline-flex;
    align-items: center;
    gap: 0.25rem;
    padding: 0.15rem 0.5rem;
    font-size: 0.72rem;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid var(--border-color);
    border-radius: 5px;
    color: var(--color-primary-light);
    text-decoration: none;
    font-weight: 500;
    transition: all 0.15s ease;
  }
  .doc-link-badge:hover {
    background: rgba(99, 102, 241, 0.08);
    border-color: rgba(99, 102, 241, 0.3);
    color: #ffffff;
  }

  .label-with-doc {
    display: flex;
    justify-content: space-between;
    align-items: center;
    width: 100%;
  }
  .label-with-doc label {
    margin: 0;
  }
  .doc-link-small {
    font-size: 0.75rem;
    color: var(--color-primary-light);
    text-decoration: none;
    display: inline-flex;
    align-items: center;
    gap: 0.2rem;
  }
  .doc-link-small:hover {
    text-decoration: underline;
  }

  .doc-btn {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }

  .models-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
    gap: 0.95rem;
  }

  .model-card {
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border-color);
    border-radius: 10px;
    padding: 0.95rem;
    display: flex;
    flex-direction: column;
    gap: 0.65rem;
    transition: all 0.15s ease;
  }
  .model-card:hover {
    border-color: rgba(99, 102, 241, 0.35);
    background: rgba(255, 255, 255, 0.035);
  }
  .model-card-header {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    border-bottom: 1px solid var(--border-subtle);
    padding-bottom: 0.6rem;
  }
  .model-card-top-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 0.5rem;
  }
  .model-card-label {
    margin: 0;
    font-size: 0.95rem;
    font-weight: 600;
    color: var(--text-main);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .model-card-actions {
    display: flex;
    gap: 0.25rem;
    flex-shrink: 0;
  }
  .model-card-sub {
    font-size: 0.82rem;
    color: var(--text-muted);
    display: flex;
    align-items: center;
    gap: 0.4rem;
    flex-wrap: wrap;
    width: 100%;
  }
  .sub-key {
    font-size: 0.78rem;
    color: var(--text-muted);
    flex-shrink: 0;
  }
  .model-name-code {
    font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, monospace;
    font-size: 0.84rem;
    font-weight: 600;
    color: var(--color-primary-light);
    background: rgba(99, 102, 241, 0.08);
    border: 1px solid rgba(99, 102, 241, 0.2);
    padding: 0.15rem 0.45rem;
    border-radius: 5px;
    word-break: break-all;
  }

  .param-tags {
    display: flex;
    flex-wrap: wrap;
    gap: 0.35rem;
  }
  .param-tag {
    background: rgba(255, 255, 255, 0.04);
    border: 1px solid var(--border-color);
    font-size: 0.72rem;
    padding: 0.12rem 0.45rem;
    border-radius: 5px;
    color: var(--text-secondary);
    font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  }
  .field-tag {
    color: var(--color-primary-light);
    border-color: rgba(99, 102, 241, 0.25);
    background: rgba(99, 102, 241, 0.06);
  }
  .extra-summary {
    font-size: 0.72rem;
    color: var(--text-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .empty-panel {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 3rem;
    color: var(--text-muted);
    gap: 1rem;
    text-align: center;
  }

  /* Modals */
  .modal-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.72);
    backdrop-filter: blur(14px);
    -webkit-backdrop-filter: blur(14px);
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
    max-width: 580px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    box-shadow: 0 24px 60px rgba(0, 0, 0, 0.75);
  }
  .modal-dialog-lg {
    max-width: 680px;
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
  }
  .modal-footer {
    display: flex;
    justify-content: flex-end;
    gap: 0.75rem;
    padding: 0.9rem 1.35rem;
    border-top: 1px solid var(--border-color);
  }

  .form-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 1rem;
  }
  .span-2 {
    grid-column: span 2;
  }
  .form-group {
    display: flex;
    flex-direction: column;
    gap: 0.35rem;
  }
  .form-group label, .group-label {
    font-size: 0.8rem;
    font-weight: 600;
    color: var(--text-muted);
  }

  .input-text, .input-select, .input-textarea {
    width: 100%;
    box-sizing: border-box;
    padding: 0.55rem 0.75rem;
    border-radius: 7px;
    border: 1px solid var(--border-color);
    background: var(--bg-input);
    color: var(--text-main);
    font-size: 0.86rem;
    outline: none;
    transition: all 0.15s ease;
  }
  .input-text:focus, .input-select:focus, .input-textarea:focus {
    border-color: var(--border-focus);
    box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.14);
  }
  .input-textarea {
    resize: vertical;
    font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, monospace;
    font-size: 0.8rem;
  }

  .core-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
    gap: 0.6rem;
    background: var(--bg-input);
    padding: 0.85rem;
    border-radius: 8px;
    border: 1px solid var(--border-color);
  }
  .core-param-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.4rem;
    padding: 0.35rem 0.5rem;
    border-radius: 6px;
    border: 1px solid transparent;
    transition: all 0.15s ease;
  }
  .core-param-row.active {
    background: rgba(99, 102, 241, 0.08);
    border-color: rgba(99, 102, 241, 0.3);
  }
  .core-check {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.8rem;
    cursor: pointer;
  }
  .core-check input[type="checkbox"] {
    accent-color: var(--color-primary);
  }
  .input-sm {
    width: 75px;
    padding: 0.25rem 0.45rem;
    font-size: 0.8rem;
    border-radius: 5px;
    border: 1px solid var(--border-color);
    background: var(--bg-card);
    color: var(--text-main);
    outline: none;
    transition: border-color 0.15s ease;
  }
  .input-sm:focus {
    border-color: var(--border-focus);
  }

  .templates-list-compact {
    display: flex;
    gap: 0.35rem;
    flex-wrap: wrap;
    margin-top: 0.35rem;
  }
  .tpl-pill {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
    padding: 0.2rem 0.55rem;
    font-size: 0.74rem;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid var(--border-color);
    color: var(--text-main);
    border-radius: 5px;
    cursor: pointer;
    transition: all 0.15s ease;
  }
  .tpl-pill:hover {
    border-color: rgba(99, 102, 241, 0.35);
    background: rgba(99, 102, 241, 0.08);
  }

  .test-result-bar {
    margin-top: 0.85rem;
    padding: 0.55rem 0.85rem;
    border-radius: 7px;
    font-size: 0.82rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .test-result-bar.success {
    background: var(--bg-success-subtle);
    border: 1px solid var(--border-success-subtle);
    color: var(--color-success-light);
  }
  .test-result-bar.failed {
    background: var(--bg-danger-subtle);
    border: 1px solid var(--border-danger-subtle);
    color: var(--color-danger-light);
  }
  .test-err-msg {
    font-size: 0.75rem;
  }

  .inline-error {
    color: var(--color-danger-light);
    font-size: 0.75rem;
  }
  .truncate {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .mb-3 {
    margin-bottom: 0.75rem;
  }
  .preview-sub {
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    margin: 0.5rem 0 0.25rem;
    color: var(--text-muted);
  }
  .code-preview {
    font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, monospace;
    font-size: 0.8rem;
    background: var(--bg-code);
    padding: 0.9rem;
    border-radius: 7px;
    border: 1px solid var(--border-color);
    overflow-x: auto;
    color: #cbd5e1;
  }

  .password-input-group {
    display: flex;
    align-items: center;
    position: relative;
    width: 100%;
  }
  .password-input-group .input-text {
    padding-right: 2.4rem;
    width: 100%;
  }
  .btn-toggle-visibility {
    position: absolute;
    right: 0.4rem;
    background: none;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0.3rem;
    border-radius: 4px;
    transition: color 0.15s, background-color 0.15s;
  }
  .btn-toggle-visibility:hover {
    color: var(--text-main);
    background: rgba(255, 255, 255, 0.06);
  }

  .modal-alert-error {
    background: var(--bg-danger-subtle);
    color: var(--color-danger-light);
    border: 1px solid var(--border-danger-subtle);
    padding: 0.65rem 0.85rem;
    border-radius: 7px;
    font-size: 0.84rem;
    margin-bottom: 1rem;
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }

  .empty-actions {
    display: flex;
    gap: 0.5rem;
    flex-wrap: wrap;
    justify-content: center;
    margin-top: 0.5rem;
  }

  /* Discovered Models & Quick-Pick styles */
  .panel-header-actions {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .btn-fetch-models {
    display: inline-flex;
    align-items: center;
    gap: 0.3rem;
  }
  .model-label-actions {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }

  .modal-title-with-icon {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .selection-controls {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    margin-bottom: 0.75rem;
    font-size: 0.8rem;
  }
  .btn-text-sm {
    background: none;
    border: none;
    color: var(--color-primary-light);
    font-size: 0.8rem;
    cursor: pointer;
    padding: 0.1rem 0.2rem;
    text-decoration: underline;
  }
  .btn-text-sm:hover {
    color: #ffffff;
  }
  .ctrl-sep {
    color: var(--text-muted);
  }
  .selection-summary {
    color: var(--text-muted);
    font-size: 0.75rem;
  }

  .discovered-models-list {
    display: flex;
    flex-direction: column;
    gap: 0.45rem;
    max-height: 320px;
    overflow-y: auto;
    padding-right: 0.3rem;
  }
  .discovered-model-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.6rem;
    padding: 0.5rem 0.75rem;
    background: rgba(255, 255, 255, 0.02);
    border: 1px solid var(--border-color);
    border-radius: 7px;
    transition: all 0.15s ease;
  }
  .discovered-model-check-label {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    cursor: pointer;
    flex: 1;
    user-select: none;
    margin: 0;
  }
  .discovered-model-check-label input[type="checkbox"] {
    accent-color: var(--color-primary);
  }
  .discovered-model-item:hover:not(.already-added) {
    border-color: rgba(99, 102, 241, 0.35);
    background: rgba(99, 102, 241, 0.06);
  }
  .discovered-model-item.already-added {
    opacity: 0.5;
    cursor: not-allowed;
    background: rgba(255, 255, 255, 0.01);
  }
  .model-id-text {
    font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, monospace;
    font-size: 0.84rem;
    flex: 1;
    word-break: break-all;
    color: var(--text-main);
  }
  .already-added-tag {
    font-size: 0.7rem;
    padding: 0.12rem 0.45rem;
    border-radius: 4px;
    background: rgba(255, 255, 255, 0.04);
    color: var(--text-muted);
    border: 1px solid var(--border-color);
  }
</style>
