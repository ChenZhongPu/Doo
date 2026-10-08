<script lang="ts">
  import { onMount } from 'svelte';
  import { t, currentLanguage, setLanguage, type Language } from './lib/i18n';
  import { api } from './lib/api';
  import Compare from './routes/Compare.svelte';
  import Config from './routes/Config.svelte';
  import History from './routes/History.svelte';

  let currentRoute: string = $state(getHashRoute());
  let healthInfo: { version: string; build_time?: string; git_commit?: string } = $state({ version: '1.0.0' });

  function getHashRoute(): string {
    const hash = window.location.hash.replace(/^#\/?/, '');
    if (hash.startsWith('config')) return 'config';
    if (hash.startsWith('history')) return 'history';
    return 'compare';
  }

  function handleHashChange() {
    currentRoute = getHashRoute();
  }

  onMount(async () => {
    window.addEventListener('hashchange', handleHashChange);
    try {
      const h = await api.getHealth();
      healthInfo = h;
    } catch (e) {
      // Ignore
    }
    return () => window.removeEventListener('hashchange', handleHashChange);
  });

  function toggleLanguage() {
    const nextLang: Language = $currentLanguage === 'en' ? 'zh' : 'en';
    setLanguage(nextLang);
  }
</script>

<div class="app-layout">
  <!-- Top Navigation Bar -->
  <header class="navbar">
    <div class="nav-left">
      <a href="#/" class="brand-link">
        <span class="brand-logo">
          <img src="/favicon.svg" alt="logo" class="brand-logo-img" width="24" height="24" />
        </span>
        <span class="brand-name">{$t('appName')}</span>
      </a>
      <div class="version-badge-group" data-tooltip={`Version: ${healthInfo.version}${healthInfo.git_commit && healthInfo.git_commit !== 'release' && healthInfo.git_commit !== 'dev' ? '\nCommit: ' + healthInfo.git_commit : ''}${healthInfo.build_time ? '\nBuild: ' + healthInfo.build_time : ''}`} data-tooltip-pos="bottom">
        <span class="brand-badge">v{healthInfo.version}</span>
        {#if (healthInfo.git_commit && healthInfo.git_commit !== 'release' && healthInfo.git_commit !== 'dev') || healthInfo.build_time}
          <span class="build-tag">
            {#if healthInfo.git_commit && healthInfo.git_commit !== 'release' && healthInfo.git_commit !== 'dev'}
              {healthInfo.git_commit}
              {#if healthInfo.build_time} • {/if}
            {/if}
            {#if healthInfo.build_time}
              {healthInfo.build_time}
            {/if}
          </span>
        {/if}
      </div>
      <span class="brand-tagline">{$t('appTagline')}</span>
    </div>

    <nav class="nav-center">
      <a href="#/" class="nav-link" class:active={currentRoute === 'compare'}>
        <svg class="nav-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="m16 16 3-8 3 8c-.87.65-1.92 1-3 1s-2.13-.35-3-1Z"/>
          <path d="m2 16 3-8 3 8c-.87.65-1.92 1-3 1s-2.13-.35-3-1Z"/>
          <path d="M7 21h10"/>
          <path d="M12 3v18"/>
          <path d="M3 7h2c2 0 5-1 7-2 2 1 5 2 7 2h2"/>
        </svg>
        <span>{$t('navCompare')}</span>
      </a>
      <a href="#/history" class="nav-link" class:active={currentRoute === 'history'}>
        <svg class="nav-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M3 12a9 9 0 1 0 9-9 9.75 9.75 0 0 0-6.74 2.74L3 8"/>
          <path d="M3 3v5h5"/>
          <path d="M12 7v5l4 2"/>
        </svg>
        <span>{$t('navHistory')}</span>
      </a>
      <a href="#/config" class="nav-link" class:active={currentRoute === 'config'}>
        <svg class="nav-icon" viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M12.22 2h-.44a2 2 0 0 0-2 2v.18a2 2 0 0 1-1 1.73l-.43.25a2 2 0 0 1-2 0l-.15-.08a2 2 0 0 0-2.73.73l-.22.38a2 2 0 0 0 .73 2.73l.15.1a2 2 0 0 1 1 1.72v.51a2 2 0 0 1-1 1.74l-.15.09a2 2 0 0 0-.73 2.73l.22.38a2 2 0 0 0 2.73.73l.15-.08a2 2 0 0 1 2 0l.43.25a2 2 0 0 1 1 1.73V20a2 2 0 0 0 2 2h.44a2 2 0 0 0 2-2v-.18a2 2 0 0 1 1-1.73l.43-.25a2 2 0 0 1 2 0l.15.08a2 2 0 0 0 2.73-.73l.22-.39a2 2 0 0 0-.73-2.73l-.15-.08a2 2 0 0 1-1-1.74v-.5a2 2 0 0 1 1-1.74l.15-.09a2 2 0 0 0 .73-2.73l-.22-.38a2 2 0 0 0-2.73-.73l-.15.08a2 2 0 0 1-2 0l-.43-.25a2 2 0 0 1-1-1.73V4a2 2 0 0 0-2-2z"/>
          <circle cx="12" cy="12" r="3"/>
        </svg>
        <span>{$t('navConfig')}</span>
      </a>
    </nav>

    <div class="nav-right">
      <button class="btn btn-sm btn-outline lang-toggle" onclick={toggleLanguage}>
        🌐 {$currentLanguage === 'en' ? '中文' : 'English'}
      </button>
    </div>
  </header>

  <!-- Route View -->
  <div class="content-wrapper">
    {#if currentRoute === 'compare'}
      <Compare />
    {:else if currentRoute === 'config'}
      <Config />
    {:else if currentRoute === 'history'}
      <History />
    {/if}
  </div>
</div>

<style>
  :global(:root) {
    --bg-page: #0f172a;
    --bg-card: #1e293b;
    --bg-card-header: #192434;
    --bg-card-tabs: #16202e;
    --bg-card-metrics: #131c28;
    --bg-item: #182232;
    --bg-selected: #1e3a5f;
    --bg-input: #0f172a;
    --bg-input-row: #162232;
    --bg-code: #0b1120;
    --bg-hover: #273549;

    --border-color: #334155;
    --border-subtle: #243042;

    --text-main: #f8fafc;
    --text-muted: #94a3b8;

    --color-primary: #38bdf8;
    --color-primary-hover: #0284c7;
    --color-success: #4ade80;
    --bg-success-subtle: rgba(74, 222, 128, 0.15);
    --color-danger: #f87171;
    --bg-danger-subtle: rgba(248, 113, 113, 0.15);
    --color-warning: #fbbf24;
    --bg-warning-subtle: rgba(251, 191, 36, 0.15);
  }

  :global(body) {
    margin: 0;
    padding: 0;
    background-color: var(--bg-page);
    color: var(--text-main);
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    -webkit-font-smoothing: antialiased;
    font-size: 14px;
    line-height: 1.5;
  }

  :global(*, *::before, *::after) {
    box-sizing: border-box;
  }

  .app-layout {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
  }

  .navbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.75rem 1.5rem;
    background: var(--bg-card);
    border-bottom: 1px solid var(--border-color);
    position: sticky;
    top: 0;
    z-index: 100;
  }

  .nav-left {
    display: flex;
    align-items: center;
    gap: 0.6rem;
  }
  .brand-link {
    display: flex;
    align-items: center;
    gap: 0.4rem;
    text-decoration: none;
    color: var(--text-main);
  }
  .brand-logo {
    font-size: 1.3rem;
  }
  .brand-name {
    font-size: 1.2rem;
    font-weight: 800;
    letter-spacing: -0.5px;
    color: var(--color-primary);
  }
  .version-badge-group {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: var(--bg-item);
    border: 1px solid var(--border-color);
    padding: 0.1rem 0.45rem;
    border-radius: 4px;
    cursor: default;
  }
  .brand-badge {
    font-size: 0.7rem;
    color: var(--color-primary);
    font-weight: 700;
  }
  .build-tag {
    font-size: 0.65rem;
    color: var(--text-muted);
    font-family: monospace;
    border-left: 1px solid var(--border-color);
    padding-left: 0.35rem;
  }
  .brand-tagline {
    font-size: 0.75rem;
    color: var(--text-muted);
    margin-left: 0.5rem;
    display: none;
  }
  @media (min-width: 860px) {
    .brand-tagline {
      display: inline;
    }
  }

  .brand-logo-img {
    width: 24px;
    height: 24px;
    object-fit: contain;
    display: block;
  }
  .nav-center {
    display: flex;
    align-items: center;
    gap: 0.5rem;
  }
  .nav-link {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
    padding: 0.4rem 0.85rem;
    border-radius: 6px;
    color: var(--text-muted);
    text-decoration: none;
    font-size: 0.88rem;
    font-weight: 500;
    transition: all 0.15s;
  }
  .nav-icon {
    flex-shrink: 0;
  }
  .nav-link:hover {
    color: var(--text-main);
    background: var(--bg-hover);
  }
  .nav-link.active {
    color: var(--color-primary);
    background: var(--bg-selected);
    font-weight: 600;
  }

  .nav-right {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }

  .content-wrapper {
    flex: 1;
    max-width: 1600px;
    width: 100%;
    margin: 0 auto;
    padding: 1.5rem;
  }

  /* Global utility components */
  :global(.btn) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    padding: 0.45rem 0.9rem;
    font-size: 0.85rem;
    font-weight: 500;
    border-radius: 6px;
    border: 1px solid transparent;
    cursor: pointer;
    transition: all 0.15s;
    background: var(--bg-hover);
    color: var(--text-main);
  }
  :global(.btn:hover:not(:disabled)) {
    filter: brightness(1.1);
  }
  :global(.btn:disabled) {
    opacity: 0.5;
    cursor: not-allowed;
  }

  :global(.btn-primary) {
    background: var(--color-primary);
    color: #0f172a;
    font-weight: 600;
  }
  :global(.btn-secondary) {
    background: var(--bg-item);
    border-color: var(--border-color);
  }
  :global(.btn-danger) {
    background: var(--color-danger);
    color: #fff;
    font-weight: 600;
  }
  :global(.btn-outline) {
    background: transparent;
    border-color: var(--border-color);
    color: var(--text-main);
  }
  :global(.btn-outline-danger) {
    background: transparent;
    border-color: var(--color-danger);
    color: var(--color-danger);
  }

  :global(.btn-sm) {
    padding: 0.25rem 0.6rem;
    font-size: 0.78rem;
  }
  :global(.btn-xs) {
    padding: 0.15rem 0.45rem;
    font-size: 0.72rem;
    border-radius: 4px;
  }
  :global(.btn-large) {
    padding: 0.6rem 1.4rem;
    font-size: 0.95rem;
  }
  :global(.btn-text) {
    background: none;
    border: none;
    color: var(--color-primary);
    cursor: pointer;
    font-size: 0.78rem;
    padding: 0.2rem 0.4rem;
  }
  :global(.btn-icon) {
    background: none;
    border: 1px solid var(--border-color);
    border-radius: 4px;
    color: var(--text-muted);
    cursor: pointer;
    width: 24px;
    height: 24px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    font-size: 0.8rem;
  }
  :global(.btn-icon:hover:not(:disabled)) {
    color: var(--text-main);
    border-color: var(--color-primary);
  }
  :global(.text-danger) {
    color: var(--color-danger) !important;
  }

  :global(.badge) {
    display: inline-block;
    padding: 0.15rem 0.45rem;
    border-radius: 4px;
    font-size: 0.72rem;
    font-weight: 600;
    text-transform: uppercase;
  }
  :global(.badge-running) {
    background: rgba(56, 189, 248, 0.2);
    color: var(--color-primary);
  }
  :global(.badge-done) {
    background: var(--bg-success-subtle);
    color: var(--color-success);
  }
  :global(.badge-error) {
    background: var(--bg-danger-subtle);
    color: var(--color-danger);
  }
  :global(.badge-canceled) {
    background: var(--bg-warning-subtle);
    color: var(--color-warning);
  }
  :global(.badge-secondary) {
    background: var(--bg-item);
    color: var(--text-muted);
  }

  :global(.alert) {
    padding: 0.75rem 1rem;
    border-radius: 6px;
    margin-bottom: 1rem;
    font-size: 0.85rem;
  }
  :global(.alert-danger) {
    background: var(--bg-danger-subtle);
    color: var(--color-danger);
    border: 1px solid var(--color-danger);
  }

  :global(.markdown-body) {
    line-height: 1.6;
    font-size: 0.88rem;
  }
  :global(.markdown-body pre) {
    background: var(--bg-code);
    padding: 0.8rem;
    border-radius: 6px;
    overflow-x: auto;
  }
  :global(.markdown-body code) {
    font-family: monospace;
    background: rgba(255, 255, 255, 0.1);
    padding: 0.1rem 0.3rem;
    border-radius: 3px;
  }
  :global(.markdown-body p) {
    margin: 0.5rem 0;
  }

  /* Global Tooltip: Always Positioned on Top to prevent obscuring content */
  :global([data-tooltip]) {
    position: relative;
  }
  :global([data-tooltip]::before) {
    content: '';
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%) translateY(1px);
    border: 5px solid transparent;
    border-top-color: #1e293b;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.15s ease;
    z-index: 10000;
  }
  :global([data-tooltip]::after) {
    content: attr(data-tooltip);
    position: absolute;
    bottom: calc(100% + 5px);
    left: 50%;
    transform: translateX(-50%);
    padding: 0.25rem 0.55rem;
    border-radius: 4px;
    background: #0f172a;
    color: #f8fafc;
    font-size: 0.72rem;
    font-weight: 500;
    line-height: 1.35;
    white-space: pre-line;
    max-width: 320px;
    width: max-content;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.15s ease;
    z-index: 10000;
    border: 1px solid var(--border-color, #334155);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.45);
    text-align: left;
  }
  :global([data-tooltip]:hover::before),
  :global([data-tooltip]:hover::after) {
    opacity: 1;
    visibility: visible;
  }
  /* Optional bottom positioning for navbar elements */
  :global([data-tooltip-pos="bottom"]::before) {
    bottom: auto;
    top: 100%;
    transform: translateX(-50%) translateY(-1px);
    border-top-color: transparent;
    border-bottom-color: #1e293b;
  }
  :global([data-tooltip-pos="bottom"]::after) {
    bottom: auto;
    top: calc(100% + 5px);
    white-space: pre-line;
  }
  /* Optional right aligned positioning for right-edge buttons */
  :global([data-tooltip-pos="top-right"]::after) {
    left: auto;
    right: 0;
    transform: none;
  }
  :global([data-tooltip-pos="top-right"]::before) {
    left: auto;
    right: 8px;
    transform: none;
  }
</style>
