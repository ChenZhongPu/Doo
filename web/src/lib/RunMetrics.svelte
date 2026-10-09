<script lang="ts">
  import type { Snippet } from 'svelte';
  import { t } from './i18n';

  let { children }: { children: Snippet } = $props();
</script>

<section class="metrics-panel" aria-label={$t('runMetrics')}>
  <h5 class="metrics-heading">{$t('runMetrics')}</h5>
  <div class="card-metrics">{@render children()}</div>
</section>

<style>
  .metrics-panel {
    container-type: inline-size;
    margin: 6px 16px 16px;
    padding: 14px 16px 16px;
    border: 1px solid var(--border-color);
    border-radius: 9px;
    background: var(--bg-card-metrics);
  }
  .metrics-heading {
    margin: 0 0 14px;
    padding-bottom: 10px;
    border-bottom: 1px solid var(--border-color);
    color: var(--text-secondary);
    font-size: 11px;
    font-weight: 600;
    letter-spacing: .04em;
  }
  .card-metrics { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 16px 20px; }
  .card-metrics :global(.metric-item) { display: flex; flex-direction: column; gap: 5px; min-width: 0; }
  .card-metrics :global(.metric-label) { color: var(--text-muted); font-size: 11px; font-weight: 400; line-height: 1.4; }
  .card-metrics :global(.metric-value) {
    color: var(--text-secondary);
    font-size: 12px;
    font-weight: 500;
    line-height: 1.5;
    font-variant-numeric: tabular-nums;
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    overflow-wrap: anywhere;
  }
  @container (max-width: 420px) { .card-metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
  @media (max-width: 600px) { .metrics-panel { margin: 6px 12px 12px; padding: 12px; } }
</style>
