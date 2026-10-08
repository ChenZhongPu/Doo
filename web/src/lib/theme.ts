import { writable } from 'svelte/store';

export type Theme = 'light' | 'dark' | 'system';
const STORAGE_KEY = 'doo_theme';
const isTheme = (value: unknown): value is Theme => value === 'light' || value === 'dark' || value === 'system';

function readTheme(): Theme {
  try {
    const saved = localStorage.getItem(STORAGE_KEY);
    if (isTheme(saved)) return saved;
  } catch { /* Appearance remains available when storage is disabled. */ }
  return 'system';
}

export const currentTheme = writable<Theme>(readTheme());

export function setTheme(theme: Theme) {
  currentTheme.set(theme);
  try { localStorage.setItem(STORAGE_KEY, theme); } catch { /* Use in-memory preference. */ }
}

export function initializeTheme() {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  let preference: Theme = 'system';
  const apply = () => {
    document.documentElement.dataset.theme = preference === 'system' ? (media.matches ? 'dark' : 'light') : preference;
  };
  const unsubscribe = currentTheme.subscribe(value => { preference = value; apply(); });
  const syncStorage = (event: StorageEvent) => {
    if (event.key === STORAGE_KEY || event.key === null) currentTheme.set(readTheme());
  };
  media.addEventListener('change', apply);
  window.addEventListener('storage', syncStorage);
  return () => {
    unsubscribe();
    media.removeEventListener('change', apply);
    window.removeEventListener('storage', syncStorage);
  };
}
