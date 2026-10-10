<script lang="ts">
  import { onMount } from 'svelte';
  import { t, currentLanguage, setLanguage } from './lib/i18n';
  import { initializeTheme } from './lib/theme';
  import { api } from './lib/api';
  import Icon from './lib/Icon.svelte';
  import ThemePicker from './lib/ThemePicker.svelte';
  import DialogHost from './lib/DialogHost.svelte';
  import TooltipHost from './lib/TooltipHost.svelte';
  import { dismissDialogs } from './lib/dialog';
  import Compare from './routes/Compare.svelte';
  import Config from './routes/Config.svelte';
  import History from './routes/History.svelte';

  function getHashRoute() {
    const hash = window.location.hash.replace(/^#\/?/, '');
    return hash.startsWith('config') ? 'config' : hash.startsWith('history') ? 'history' : 'compare';
  }
  let currentRoute = $state(getHashRoute());
  let healthInfo: { version: string; build_time?: string; git_commit?: string } = $state({ version: '0.2.0' });
  const handleHashChange = () => { dismissDialogs(); currentRoute = getHashRoute(); };

  onMount(() => {
    const cleanupTheme = initializeTheme();
    window.addEventListener('hashchange', handleHashChange);
    api.getHealth().then(health => { healthInfo = health; }).catch(() => {});
    return () => { cleanupTheme(); window.removeEventListener('hashchange', handleHashChange); };
  });
  let tabTitle = $derived(`Doo — ${$t(currentRoute === 'history' ? 'navHistory' : currentRoute === 'config' ? 'navConfig' : 'navCompare')}`);
  $effect(() => { document.documentElement.lang = $currentLanguage; });
</script>

<svelte:head><title>{tabTitle}</title></svelte:head>

<div class="app-layout">
  <header class="navbar">
    <div class="nav-left">
      <a href="#/" class="brand-link" aria-label="Doo">
        <svg class="brand-mark" width="28" height="28" viewBox="0 0 32 32" fill="none" aria-hidden="true"><path d="M7 6h8c7 0 11 4 11 10s-4 10-11 10H7V6Z" stroke="currentColor" stroke-width="2"/><path d="M12 11h3c4 0 6 2 6 5s-2 5-6 5h-3V11Z" fill="currentColor"/></svg>
        <span class="brand-name">Doo<span class="brand-dot">.</span></span>
      </a>
      <span class="brand-badge" data-tooltip={`Version: ${healthInfo.version}${healthInfo.git_commit ? '\nCommit: ' + healthInfo.git_commit : ''}${healthInfo.build_time ? '\nBuild: ' + healthInfo.build_time : ''}`}>v{healthInfo.version}</span>
    </div>
    <nav class="nav-center" aria-label={$currentLanguage === 'zh' ? '主导航' : 'Main navigation'}>
      <a href="#/" class="nav-link" class:active={currentRoute === 'compare'} aria-current={currentRoute === 'compare' ? 'page' : undefined}><Icon name="compare" /><span>{$t('navCompare')}</span></a>
      <a href="#/history" class="nav-link" class:active={currentRoute === 'history'} aria-current={currentRoute === 'history' ? 'page' : undefined}><Icon name="history" /><span>{$t('navHistory')}</span></a>
      <a href="#/config" class="nav-link" class:active={currentRoute === 'config'} aria-current={currentRoute === 'config' ? 'page' : undefined}><Icon name="settings" /><span>{$t('navConfig')}</span></a>
    </nav>
    <div class="nav-right"><button class="lang-toggle" onclick={() => setLanguage($currentLanguage === 'en' ? 'zh' : 'en')} aria-label={$currentLanguage === 'en' ? '切换到中文' : 'Switch to English'}><Icon name="language" /><span>{$currentLanguage === 'en' ? '中文' : 'English'}</span></button></div>
  </header>
  <div class="content-wrapper">
    <div hidden={currentRoute !== 'compare'}><Compare active={currentRoute === 'compare'} /></div>
    {#if currentRoute === 'config'}<Config />{:else if currentRoute === 'history'}<History />{/if}
  </div>
  <ThemePicker />
  <DialogHost />
  <TooltipHost />
</div>

<style>
  .app-layout { min-height: 100vh; }
  .navbar { display: grid; grid-template-columns: 1fr auto 1fr; align-items: center; height: 72px; padding: 0 36px; border-bottom: 1px solid var(--border-color); background: var(--bg-page); position: sticky; top: 0; z-index: 100; }
  .nav-left { display: flex; align-items: center; gap: 12px; }
  .brand-link { display: inline-flex; align-items: center; gap: 9px; color: var(--text-main); text-decoration: none; }
  .brand-mark { color: var(--color-primary); }
  .brand-name { font-size: 23px; font-weight: 650; letter-spacing: -.07em; }
  .brand-dot { color: var(--color-primary); }
  .brand-badge { font-size: 10px; color: var(--text-muted); border-left: 1px solid var(--border-color); padding-left: 12px; font-family: ui-monospace, monospace; }
  .nav-center { display: flex; height: 100%; align-items: stretch; gap: 28px; }
  .nav-link { display: inline-flex; align-items: center; gap: 8px; color: var(--text-secondary); text-decoration: none; font-size: 13px; border-bottom: 2px solid transparent; padding: 2px 2px 0; }
  .nav-link:hover { color: var(--text-main); }
  .nav-link.active { color: var(--color-primary); border-bottom-color: var(--color-primary); font-weight: 600; }
  .nav-right { justify-self: end; }
  .lang-toggle { display: inline-flex; align-items: center; gap: 7px; padding: 7px 9px; background: transparent; border: 1px solid transparent; border-radius: 6px; color: var(--text-secondary); font-size: 12px; cursor: pointer; }
  .lang-toggle:hover { background: var(--bg-hover); color: var(--text-main); }
  .content-wrapper { max-width: 1540px; width: 100%; margin: 0 auto; padding: 36px 36px 90px; }
  @media (max-width: 700px) { .navbar { height: auto; min-height: 64px; grid-template-columns: 1fr auto; padding: 0 18px; } .nav-left { padding: 16px 0; } .nav-center { grid-row: 2; grid-column: 1 / -1; justify-content: center; gap: 32px; height: 42px; } .nav-right { grid-row: 1; grid-column: 2; } .content-wrapper { padding: 24px 16px 80px; } }
  /* Global utility components */
  :global(.btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    padding: 0.45rem 0.9rem;
    font-size: 0.83rem;
    font-weight: 500;
    border-radius: 7px;
    border: 1px solid transparent;
    cursor: pointer;
    transition: all 0.15s ease;
    background: var(--bg-hover);
    color: var(--text-main);
    line-height: 1.35;
  }
  :global(.btn:hover:not(:disabled)) {
    background-color: var(--bg-active);
  }
  :global(.btn:active:not(:disabled)) {
    transform: scale(0.98);
  }
  :global(.btn:disabled) {
    opacity: 0.4;
    cursor: not-allowed;
  }

  :global(.btn-primary) {
    background: var(--bg-primary-button);
    color: var(--color-on-primary);
    font-weight: 600;
    border: 1px solid var(--bg-hover);
    box-shadow: none;
  }
  :global(.btn-primary:hover:not(:disabled)) {
    box-shadow: none;
  }
  :global(.btn-secondary) {
    background: var(--bg-item);
    border-color: var(--border-color);
    color: var(--text-main);
  }
  :global(.btn-secondary:hover:not(:disabled)) {
    background: var(--bg-hover);
    border-color: var(--bg-hover);
  }
  :global(.btn-danger) {
    background: var(--color-danger);
    color: var(--bg-card);
    font-weight: 600;
    border: 1px solid var(--bg-hover);
    box-shadow: none;
  }
  :global(.btn-outline) {
    background: transparent;
    border-color: var(--border-color);
    color: var(--text-secondary);
  }
  :global(.btn-outline:hover:not(:disabled)) {
    color: var(--text-main);
    background: var(--bg-hover);
    border-color: var(--bg-hover);
  }
  :global(.btn-outline-danger) {
    background: transparent;
    border-color: var(--border-danger-subtle);
    color: var(--color-danger-light);
  }
  :global(.btn-outline-danger:hover:not(:disabled)) {
    background: var(--bg-danger-subtle);
    border-color: var(--color-danger);
  }

  :global(.btn-sm) {
    padding: 0.28rem 0.65rem;
    font-size: 0.78rem;
    border-radius: 6px;
  }
  :global(.btn-xs) {
    padding: 0.16rem 0.45rem;
    font-size: 0.72rem;
    border-radius: 5px;
  }
  :global(.btn-large) {
    padding: 0.6rem 1.4rem;
    font-size: 0.92rem;
    border-radius: 8px;
  }
  :global(.btn-text) {
    background: none;
    border: none;
    color: var(--color-primary-light);
    cursor: pointer;
    font-size: 0.78rem;
    padding: 0.2rem 0.4rem;
  }
  :global(.btn-text:hover) {
    color: var(--text-main);
    text-decoration: underline;
  }
  :global(.btn-icon) {
    background: none;
    border: 1px solid var(--border-color);
    border-radius: 6px;
    color: var(--text-muted);
    cursor: pointer;
    width: 26px;
    height: 26px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8rem;
    transition: all 0.15s ease;
  }
  :global(.btn-icon:hover:not(:disabled)) {
    color: var(--text-main);
    background: var(--bg-hover);
    border-color: var(--bg-hover);
  }
  :global(.text-danger) {
    color: var(--color-danger-light) !important;
  }

  :global(.badge) {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.15rem 0.5rem;
    border-radius: 9999px;
    font-size: 0.7rem;
    font-weight: 500;
    letter-spacing: 0.02em;
    line-height: 1.3;
  }
  :global(.badge-running) {
    background: var(--color-primary-subtle);
    color: var(--color-primary-light);
    border: 1px solid var(--color-primary-subtle);
  }
  :global(.badge-done) {
    background: var(--bg-success-subtle);
    color: var(--color-success-light);
    border: 1px solid var(--border-success-subtle);
  }
  :global(.badge-error) {
    background: var(--bg-danger-subtle);
    color: var(--color-danger-light);
    border: 1px solid var(--border-danger-subtle);
  }
  :global(.badge-canceled) {
    background: rgba(161, 161, 170, 0.08);
    color: var(--text-muted);
    border: 1px solid rgba(161, 161, 170, 0.2);
  }
  :global(.badge-secondary) {
    background: var(--bg-item);
    color: var(--text-muted);
    border: 1px solid var(--border-color);
  }

  :global(.alert) {
    padding: 0.75rem 1rem;
    border-radius: 8px;
    margin-bottom: 1rem;
    font-size: 0.84rem;
  }
  :global(.alert-danger) {
    background: var(--bg-danger-subtle);
    color: var(--color-danger-light);
    border: 1px solid var(--border-danger-subtle);
  }

  :global(.markdown-body) {
    line-height: 1.65;
    font-size: 0.88rem;
    color: var(--text-main);
  }
  :global(.markdown-body pre) {
    background: var(--bg-code);
    border: 1px solid var(--border-color);
    padding: 0.85rem 1rem;
    border-radius: 8px;
    overflow-x: auto;
    font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, monospace;
    font-size: 0.82rem;
  }
  :global(.markdown-body code) {
    font-family: ui-monospace, SFMono-Regular, "JetBrains Mono", Menlo, Consolas, monospace;
    background: var(--bg-hover);
    border: 1px solid var(--bg-hover);
    padding: 0.12rem 0.35rem;
    border-radius: 4px;
    font-size: 0.84em;
    color: var(--text-main);
  }
  :global(.markdown-body pre code) {
    background: transparent;
    border: none;
    padding: 0;
  }
  :global(.markdown-body p) {
    margin: 0.5rem 0;
  }

  :global(.btn-primary:hover:not(:disabled)) { background: var(--bg-primary-button-hover); }
  :global(.btn) { gap: 7px; font-weight: 500; }
  :global(.btn-icon:disabled) { opacity: .35; cursor: not-allowed; }
  :global(.btn-footer) { display: inline-flex; align-items: center; gap: 6px; }
  :global(.badge-idle) { color: var(--text-muted); background: var(--bg-item); }
  :global(.badge-running) { border-color: var(--border-focus); }
</style>
