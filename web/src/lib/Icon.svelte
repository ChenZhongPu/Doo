<script lang="ts">
  const paths = {
    compare: 'M8 3H3v18h5M16 3h5v18h-5M12 3v18',
    history: 'M3 11a9 9 0 1 1 2.6 7.4M3 4v7h7M12 7v5l3 2',
    settings: 'M4 7h16M4 17h16M8 4v6M16 14v6',
    language: 'M3 5h12M9 3v2M5 5c0 6 6 10 6 10M13 5c0 6-6 10-10 10M14 21l4-10 4 10M16 17h4',
    sun: 'M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.4 1.4M17.6 17.6 19 19M5 19l1.4-1.4M17.6 6.4 19 5M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
    moon: 'M20.9 13A9 9 0 0 1 11 3.1 9 9 0 1 0 20.9 13Z',
    monitor: 'M4 3h16a1 1 0 0 1 1 1v12H3V4a1 1 0 0 1 1-1ZM8 21h8M12 16v5',
    plus: 'M12 5v14M5 12h14',
    close: 'm6 6 12 12M6 18 18 6',
    up: 'm6 10 6-6 6 6M12 4v16',
    down: 'm6 14 6 6 6-6M12 4v16',
    chevron: 'm7 10 5 5 5-5',
    send: 'M5 12h14m-6-6 6 6-6 6',
    stop: 'M6 6h12v12H6Z',
    code: 'm8 7-5 5 5 5m8-10 5 5-5 5M14 4l-4 16',
    copy: 'M8 8h13v13H8ZM16 4V3H3v13h1',
    erase: 'm16 3 5 5-12 12H5l-4-4L16 3ZM8 11l8 8M9 20h12',
    check: 'm5 12 4 4L19 6',
    refresh: 'M20 7a9 9 0 0 0-15-1L3 8m0-5v5h5M4 17a9 9 0 0 0 15 1l2-2m0 5v-5h-5',
    edit: 'm16 3 5 5-12 12-6 1 1-6L16 3ZM13 6l5 5',
    trash: 'M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7',
    connection: 'M8 3v5M16 3v5M6 8h12v3a6 6 0 0 1-12 0V8ZM12 17v4',
    book: 'M12 5C9 3 5 3 3 4v15c2-1 6-1 9 1 3-2 7-2 9-1V4c-2-1-6-1-9 1ZM12 5v15',
    import: 'M12 3v12m-5-5 5 5 5-5M4 16v5h16v-5',
    grid: 'M3 3h7v7H3ZM14 3h7v7h-7ZM3 14h7v7H3ZM14 14h7v7h-7Z',
    message: 'M4 4h16v13H8l-4 4V4ZM8 8h8M8 12h5',
  };
  let { name, size = 16 }: { name: keyof typeof paths; size?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" class="ui-icon">
  <path d={paths[name]} />
</svg>

<style>
  svg { display: inline-block; flex-shrink: 0; vertical-align: middle; }
</style>
