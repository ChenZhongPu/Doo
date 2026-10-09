<script lang="ts">
  import { onDestroy } from 'svelte';
  import Icon from './Icon.svelte';
  import { t } from './i18n';
  import { writeClipboard } from './clipboard';

  let { text, onclear, disabled = false }: { text: string; onclear: () => void; disabled?: boolean } = $props();
  let result = $state<'copied' | 'failed' | null>(null);
  let copying = $state(false);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let destroyed = false;
  let copyLabel = $derived(result === 'copied' ? $t('copied') : result === 'failed' ? $t('copyMessageFailed') : $t('copyMessage'));

  $effect(() => {
    text;
    clearTimeout(timer);
    result = null;
  });

  onDestroy(() => {
    destroyed = true;
    clearTimeout(timer);
  });

  async function copyMessage() {
    const value = text;
    copying = true;
    let nextResult: 'copied' | 'failed';
    try {
      await writeClipboard(value);
      nextResult = 'copied';
    } catch {
      nextResult = 'failed';
    } finally {
      copying = false;
    }
    if (destroyed || text !== value) return;
    clearTimeout(timer);
    result = nextResult;
    timer = setTimeout(() => { result = null; }, 2000);
  }
</script>

<div class="message-actions">
  <button type="button" class="btn-icon" disabled={disabled || !text} onclick={onclear} aria-label={$t('clearMessage')} title={$t('clearMessage')} data-tooltip={$t('clearMessage')}>
    <Icon name="erase" size={14} />
  </button>
  <button type="button" class="btn-icon" class:copied={result === 'copied'} disabled={!text || copying} onclick={copyMessage} aria-label={copyLabel} title={copyLabel} data-tooltip={copyLabel}>
    <Icon name={result === 'copied' ? 'check' : 'copy'} size={14} />
  </button>
  <span class="copy-feedback" role="status" aria-live="polite">{result ? copyLabel : ''}</span>
</div>

<style>
  .message-actions { display: flex; gap: 0.3rem; padding-right: 0.45rem; margin-right: 0.15rem; border-right: 1px solid var(--border-color); }
  .copied { color: var(--color-success); }
  .copy-feedback { position: absolute; width: 1px; height: 1px; margin: -1px; overflow: hidden; clip-path: inset(50%); white-space: nowrap; }
</style>
