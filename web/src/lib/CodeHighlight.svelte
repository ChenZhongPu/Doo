<script lang="ts">
  import DOMPurify from 'dompurify';
  import { highlightCode } from './highlight';

  let { text, language }: { text: string; language: string } = $props();
  const html = $derived(DOMPurify.sanitize(highlightCode(text, language)));
</script>

<code class="syntax-code">{@html html}</code>
