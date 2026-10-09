<script lang="ts">
  import { t } from './i18n';
  let { page, total, disabled = false, onchange }: {
    page: number; total: number; disabled?: boolean; onchange: (page: number) => void;
  } = $props();
  const pageSize = 100;
  let pages = $derived(Math.max(1, Math.ceil(total / pageSize)));
  let range = $derived($t('historyRange').replace('{start}', String(total ? (page - 1) * pageSize + 1 : 0)).replace('{end}', String(Math.min(page * pageSize, total))).replace('{total}', String(total)));

  function goToPage(input: HTMLInputElement) {
    if (disabled) return;
    const value = input.valueAsNumber;
    const next = Number.isFinite(value) ? Math.min(pages, Math.max(1, Math.trunc(value))) : page;
    input.value = String(next);
    if (next !== page) onchange(next);
  }
</script>

<nav class="history-pagination" aria-label={$t('historyPagination')}>
  <span class="history-range" aria-live="polite">{range}</span>
  <div class="page-controls">
    <button class="btn btn-sm btn-secondary" disabled={disabled || page <= 1} onclick={() => onchange(page - 1)}>{$t('previousPage')}</button>
    <label class="page-position">
      <span class="sr-only">{$t('goToPage')}</span>
      <input type="number" min="1" max={pages} value={page} disabled={disabled} aria-label={$t('goToPage')}
        onchange={event => goToPage(event.currentTarget)}
        onkeydown={event => { if (event.key === 'Enter') { event.preventDefault(); goToPage(event.currentTarget); } }} />
      <span>/ {pages}</span>
    </label>
    <button class="btn btn-sm btn-secondary" disabled={disabled || page >= pages} onclick={() => onchange(page + 1)}>{$t('nextPage')}</button>
  </div>
</nav>

<style>
  .history-pagination { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; }
  .history-range { color: var(--text-muted); font-size: 12px; }
  .page-controls { display: flex; align-items: center; gap: 12px; }
  .page-position { display: flex; align-items: center; gap: 7px; color: var(--text-secondary); font-size: 12px; font-variant-numeric: tabular-nums; }
  input { width: 56px; padding: 5px 6px; border: 1px solid var(--border-color); border-radius: 6px; background: var(--bg-card); color: var(--text-main); font: inherit; text-align: center; appearance: textfield; }
  input::-webkit-inner-spin-button, input::-webkit-outer-spin-button { appearance: none; margin: 0; }
  input:disabled { opacity: .4; }
  .sr-only { position: absolute; width: 1px; height: 1px; padding: 0; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
  @media (max-width: 600px) { .history-pagination { justify-content: center; } .history-range { width: 100%; text-align: center; } }
</style>
