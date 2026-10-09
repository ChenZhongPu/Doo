<script lang="ts">
  let { size = 16 }: { size?: number } = $props();
</script>

<svg width={size} height={size} viewBox="0 0 112 112" aria-hidden="true">
  <path fill="#3776ab" fill-rule="evenodd" d="M55 0C50 0 46 .4 42 1c-11 2-13 6-13 14v10h27v4H19C11 29 4 34 2 42c-3 11-3 17 0 28 2 8 6 13 14 13h9V71c0-9 8-17 17-17h27c8 0 13-6 13-13V15c0-7-6-13-13-14C64 .3 60 0 55 0ZM40.5 8a5 5 0 1 1 0 10 5 5 0 0 1 0-10Z" />
  <path fill="#ffd343" fill-rule="evenodd" d="M86 29v12c0 9-8 17-17 17H42c-7 0-13 6-13 13v26c0 7 6 11 13 13 9 3 17 3 27 0 7-2 13-6 13-13V87H56v-4h40c8 0 11-5 13-13 3-9 3-17 0-28-2-8-6-13-13-13H86ZM70.5 93a5 5 0 1 1 0 10 5 5 0 0 1 0-10Z" />
</svg>

<style>
  svg { display: inline-block; flex-shrink: 0; vertical-align: middle; }
</style>
