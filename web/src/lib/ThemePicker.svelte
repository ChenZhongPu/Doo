<script lang="ts">
  import { t } from './i18n';
  import { currentTheme, setTheme, type Theme } from './theme';
  import Icon from './Icon.svelte';

  let picker: HTMLDetailsElement;
  let trigger: HTMLElement;
  const options = [
    { value: 'light', label: 'themeLight', icon: 'sun' },
    { value: 'dark', label: 'themeDark', icon: 'moon' },
    { value: 'system', label: 'themeSystem', icon: 'monitor' },
  ] as const;
  function choose(value: Theme) {
    setTheme(value);
    picker.open = false;
    trigger.focus();
  }
</script>

<svelte:window onclick={event => { if (picker?.open && event.target instanceof Node && !picker.contains(event.target)) picker.open = false; }} onkeydown={event => { if (event.key === 'Escape' && picker?.open) { picker.open = false; trigger.focus(); } }} />

<details class="theme-picker" bind:this={picker}>
  <summary bind:this={trigger} aria-label={$t('appearance')} data-tooltip={$t('appearance')}>
    <Icon name={$currentTheme === 'system' ? 'monitor' : $currentTheme === 'dark' ? 'moon' : 'sun'} size={17} />
    <span>{$t('appearance')}</span>
    <Icon name="chevron" size={13} />
  </summary>
  <div class="theme-menu" role="group" aria-label={$t('appearance')}>
    <div class="menu-title">{$t('appearance')}</div>
    {#each options as option}
      <button class:chosen={$currentTheme === option.value} aria-pressed={$currentTheme === option.value} onclick={() => choose(option.value)}>
        <Icon name={option.icon} />
        <span>{$t(option.label)}</span>
        {#if $currentTheme === option.value}<Icon name="check" size={15} />{/if}
      </button>
    {/each}
  </div>
</details>

<style>
  .theme-picker { position: fixed; bottom: 20px; left: 24px; z-index: 120; }
  summary { display: flex; align-items: center; gap: 9px; padding: 9px 12px; border: 1px solid var(--border-color); border-radius: 8px; background: var(--bg-card); color: var(--text-secondary); cursor: pointer; font-size: 12px; list-style: none; box-shadow: var(--shadow-sm); }
  summary::-webkit-details-marker { display: none; }
  summary:hover, details[open] summary { color: var(--text-main); background: var(--bg-hover); }
  .theme-menu { position: absolute; bottom: calc(100% + 8px); left: 0; width: 196px; padding: 6px; background: var(--bg-card); border: 1px solid var(--border-color); border-radius: 10px; box-shadow: var(--shadow-menu); }
  .menu-title { padding: 8px 10px; color: var(--text-muted); font-size: 11px; }
  button { width: 100%; display: flex; align-items: center; gap: 10px; background: transparent; border: 0; border-radius: 5px; padding: 10px; color: var(--text-secondary); text-align: left; font: inherit; font-size: 13px; cursor: pointer; }
  button span { flex: 1; }
  button:hover { background: var(--bg-hover); color: var(--text-main); }
  button.chosen { background: var(--bg-selected); color: var(--color-primary); }
  @media (max-width: 600px) {
    .theme-picker { bottom: 10px; left: 12px; }
    .theme-picker::before { content: ''; position: fixed; bottom: 0; left: 0; right: 0; height: 58px; background: var(--bg-page); border-top: 1px solid var(--border-color); z-index: -1; }
  }
</style>
