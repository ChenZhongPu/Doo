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

  let tabTitle = $derived.by(() => {
    let pageName = $t('navCompare');
    if (currentRoute === 'history') {
      pageName = $t('navHistory');
    } else if (currentRoute === 'config') {
      pageName = $t('navConfig');
    }
    return `Doo - ${pageName}`;
  });

  $effect(() => {
    if (typeof document !== 'undefined') {
      document.title = tabTitle;
      document.documentElement.lang = $currentLanguage;
    }
  });
</script>

<svelte:head>
  <title>{tabTitle}</title>
</svelte:head>

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
    /* Base canvas & surfaces (Obsidian / Zinc aesthetic) */
    --bg-page: #090a0f;
    --bg-card: #13141c;
    --bg-card-header: #101118;
    --bg-card-tabs: #0d0e14;
    --bg-card-metrics: #0a0b10;
    --bg-item: #171822;
    --bg-selected: rgba(99, 102, 241, 0.08);
    --bg-input: #0d0e14;
    --bg-input-row: #14151e;
    --bg-code: #0a0b10;
    --bg-hover: #1c1d29;
    --bg-active: #232434;

    /* Hairline translucent borders */
    --border-color: rgba(255, 255, 255, 0.08);
    --border-subtle: rgba(255, 255, 255, 0.04);
    --border-focus: rgba(99, 102, 241, 0.45);

    /* Text & contrast */
    --text-main: #f1f2f6;
    --text-secondary: #9ca0af;
    --text-muted: #686b7c;
    --text-dim: #454754;

    /* Refined Primary: Sophisticated Indigo-Violet */
    --color-primary: #6366f1;
    --color-primary-hover: #4f46e5;
    --color-primary-light: #818cf8;
    --color-primary-subtle: rgba(99, 102, 241, 0.12);

    /* Functional semantic colors (subdued, elegant, never neon) */
    --color-success: #10b981;
    --color-success-light: #34d399;
    --bg-success-subtle: rgba(16, 185, 129, 0.1);
    --border-success-subtle: rgba(16, 185, 129, 0.2);

    --color-danger: #f43f5e;
    --color-danger-light: #fb7185;
    --bg-danger-subtle: rgba(244, 63, 94, 0.1);
    --border-danger-subtle: rgba(244, 63, 94, 0.2);

    --color-warning: #f59e0b;
    --color-warning-light: #fbbf24;
    --bg-warning-subtle: rgba(245, 158, 11, 0.1);
    --border-warning-subtle: rgba(245, 158, 11, 0.2);
  }

  :global(body) {
    margin: 0;
    padding: 0;
    background-color: var(--bg-page);
    color: var(--text-main);
    font-family: -apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    -webkit-font-smoothing: antialiased;
    -moz-osx-font-smoothing: grayscale;
    font-size: 14px;
    line-height: 1.5;
  }

  :global(*, *::before, *::after) {
    box-sizing: border-box;
  }

  /* Custom modern minimalist scrollbar */
  :global(::-webkit-scrollbar) {
    width: 6px;
    height: 6px;
  }
  :global(::-webkit-scrollbar-track) {
    background: transparent;
  }
  :global(::-webkit-scrollbar-thumb) {
    background: rgba(255, 255, 255, 0.12);
    border-radius: 9999px;
  }
  :global(::-webkit-scrollbar-thumb:hover) {
    background: rgba(255, 255, 255, 0.22);
  }

  .app-layout {
    min-height: 100vh;
    display: flex;
    flex-direction: column;
    background: radial-gradient(circle at 50% 0%, rgba(99, 102, 241, 0.05) 0%, transparent 60%);
  }

  .navbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 0.65rem 1.75rem;
    background: rgba(13, 14, 20, 0.8);
    backdrop-filter: blur(16px);
    -webkit-backdrop-filter: blur(16px);
    border-bottom: 1px solid var(--border-color);
    position: sticky;
    top: 0;
    z-index: 100;
  }

  .nav-left {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .brand-link {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    text-decoration: none;
    color: var(--text-main);
  }
  .brand-logo-img {
    width: 22px;
    height: 22px;
    object-fit: contain;
    display: block;
    filter: drop-shadow(0 2px 4px rgba(0, 0, 0, 0.3));
  }
  .brand-name {
    font-size: 1.15rem;
    font-weight: 700;
    letter-spacing: -0.03em;
    color: #ffffff;
  }
  .version-badge-group {
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid var(--border-color);
    padding: 0.12rem 0.45rem;
    border-radius: 9999px;
    cursor: default;
  }
  .brand-badge {
    font-size: 0.68rem;
    color: var(--color-primary-light);
    font-weight: 600;
    letter-spacing: 0.02em;
  }
  .build-tag {
    font-size: 0.65rem;
    color: var(--text-muted);
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
    border-left: 1px solid var(--border-color);
    padding-left: 0.35rem;
  }

  .nav-center {
    display: flex;
    align-items: center;
    gap: 0.25rem;
    background: rgba(255, 255, 255, 0.025);
    border: 1px solid var(--border-color);
    padding: 0.2rem;
    border-radius: 9px;
  }
  .nav-link {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    padding: 0.35rem 0.8rem;
    border-radius: 7px;
    color: var(--text-secondary);
    text-decoration: none;
    font-size: 0.82rem;
    font-weight: 500;
    transition: all 0.15s ease;
    border: 1px solid transparent;
  }
  .nav-icon {
    flex-shrink: 0;
    opacity: 0.8;
  }
  .nav-link:hover {
    color: var(--text-main);
    background: rgba(255, 255, 255, 0.04);
  }
  .nav-link.active {
    color: #ffffff;
    background: rgba(255, 255, 255, 0.08);
    border-color: rgba(255, 255, 255, 0.08);
    font-weight: 600;
    box-shadow: 0 1px 4px rgba(0, 0, 0, 0.2);
  }
  .nav-link.active .nav-icon {
    opacity: 1;
    color: var(--color-primary-light);
  }

  .nav-right {
    display: flex;
    align-items: center;
    gap: 0.75rem;
  }
  .lang-toggle {
    background: rgba(255, 255, 255, 0.03);
    border: 1px solid var(--border-color);
    border-radius: 7px;
    color: var(--text-secondary);
    font-size: 0.78rem;
    padding: 0.35rem 0.7rem;
    transition: all 0.15s ease;
  }
  .lang-toggle:hover {
    color: var(--text-main);
    background: rgba(255, 255, 255, 0.06);
    border-color: rgba(255, 255, 255, 0.12);
  }

  .content-wrapper {
    flex: 1;
    max-width: 1600px;
    width: 100%;
    margin: 0 auto;
    padding: 1.5rem 1.75rem;
  }

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
    filter: brightness(1.08);
  }
  :global(.btn:active:not(:disabled)) {
    transform: scale(0.98);
  }
  :global(.btn:disabled) {
    opacity: 0.4;
    cursor: not-allowed;
  }

  :global(.btn-primary) {
    background: linear-gradient(135deg, #6366f1 0%, #4f46e5 100%);
    color: #ffffff;
    font-weight: 600;
    border: 1px solid rgba(255, 255, 255, 0.15);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3), 0 0 14px rgba(99, 102, 241, 0.25);
  }
  :global(.btn-primary:hover:not(:disabled)) {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.4), 0 0 18px rgba(99, 102, 241, 0.4);
  }
  :global(.btn-secondary) {
    background: var(--bg-item);
    border-color: var(--border-color);
    color: var(--text-main);
  }
  :global(.btn-secondary:hover:not(:disabled)) {
    background: var(--bg-hover);
    border-color: rgba(255, 255, 255, 0.12);
  }
  :global(.btn-danger) {
    background: linear-gradient(135deg, #f43f5e 0%, #e11d48 100%);
    color: #ffffff;
    font-weight: 600;
    border: 1px solid rgba(255, 255, 255, 0.15);
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3), 0 0 12px rgba(244, 63, 94, 0.25);
  }
  :global(.btn-outline) {
    background: transparent;
    border-color: var(--border-color);
    color: var(--text-secondary);
  }
  :global(.btn-outline:hover:not(:disabled)) {
    color: var(--text-main);
    background: rgba(255, 255, 255, 0.04);
    border-color: rgba(255, 255, 255, 0.14);
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
    color: #ffffff;
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
    background: rgba(255, 255, 255, 0.04);
    border-color: rgba(255, 255, 255, 0.15);
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
    color: #a5b4fc;
    border: 1px solid rgba(99, 102, 241, 0.25);
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
    color: #a1a1aa;
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
    background: rgba(255, 255, 255, 0.06);
    border: 1px solid rgba(255, 255, 255, 0.06);
    padding: 0.12rem 0.35rem;
    border-radius: 4px;
    font-size: 0.84em;
    color: #e2e8f0;
  }
  :global(.markdown-body pre code) {
    background: transparent;
    border: none;
    padding: 0;
  }
  :global(.markdown-body p) {
    margin: 0.5rem 0;
  }

  /* Global Tooltip: Elegant frosted glass */
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
    border-top-color: #171822;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.15s ease, transform 0.15s ease;
    z-index: 10000;
  }
  :global([data-tooltip]::after) {
    content: attr(data-tooltip);
    position: absolute;
    bottom: calc(100% + 5px);
    left: 50%;
    transform: translateX(-50%) translateY(2px);
    padding: 0.3rem 0.6rem;
    border-radius: 6px;
    background: rgba(18, 19, 27, 0.96);
    backdrop-filter: blur(12px);
    -webkit-backdrop-filter: blur(12px);
    color: #f1f2f6;
    font-size: 0.72rem;
    font-weight: 500;
    line-height: 1.38;
    white-space: pre-line;
    max-width: 320px;
    width: max-content;
    pointer-events: none;
    opacity: 0;
    visibility: hidden;
    transition: opacity 0.15s ease, transform 0.15s ease;
    z-index: 10000;
    border: 1px solid rgba(255, 255, 255, 0.12);
    box-shadow: 0 8px 24px -2px rgba(0, 0, 0, 0.65);
    text-align: left;
  }
  :global([data-tooltip]:hover::before) {
    opacity: 1;
    visibility: visible;
    transform: translateX(-50%) translateY(0);
  }
  :global([data-tooltip]:hover::after) {
    opacity: 1;
    visibility: visible;
    transform: translateX(-50%) translateY(0);
  }
  /* Optional bottom positioning for navbar elements */
  :global([data-tooltip-pos="bottom"]::before) {
    bottom: auto;
    top: 100%;
    transform: translateX(-50%) translateY(-1px);
    border-top-color: transparent;
    border-bottom-color: #171822;
  }
  :global([data-tooltip-pos="bottom"]::after) {
    bottom: auto;
    top: calc(100% + 5px);
    transform: translateX(-50%) translateY(-2px);
    white-space: pre-line;
  }
  :global([data-tooltip-pos="bottom"]:hover::before) {
    transform: translateX(-50%) translateY(0);
  }
  :global([data-tooltip-pos="bottom"]:hover::after) {
    transform: translateX(-50%) translateY(0);
  }
  /* Optional right aligned positioning for right-edge buttons */
  :global([data-tooltip-pos="top-right"]::after) {
    left: auto;
    right: 0;
    transform: translateY(2px);
  }
  :global([data-tooltip-pos="top-right"]::before) {
    left: auto;
    right: 8px;
    transform: translateY(1px);
  }
  :global([data-tooltip-pos="top-right"]:hover::after) {
    transform: translateY(0);
  }
  :global([data-tooltip-pos="top-right"]:hover::before) {
    transform: translateY(0);
  }
</style>
