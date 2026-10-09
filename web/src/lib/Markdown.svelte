<script lang="ts">
  import { marked } from 'marked';
  import DOMPurify from 'dompurify';
  import { t } from './i18n';
  import { writeClipboard } from './clipboard';
  import { highlightCode } from './highlight';

  let { text }: { text: string } = $props();
  let feedback = $state('');

  const copyIcon = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M8 8h13v13H8ZM16 4V3H3v13h1" /></svg>';
  const checkIcon = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="m5 12 4 4L19 6" /></svg>';

  type RenderOptions = { text: string; copy: string; copied: string; failed: string };

  function renderContent(node: HTMLDivElement, options: RenderOptions) {
    const timers = new Set<ReturnType<typeof setTimeout>>();
    function clearTimers() {
      for (const timer of timers) clearTimeout(timer);
      timers.clear();
    }
    function render(next: RenderOptions) {
      clearTimers();
      feedback = '';
      node.innerHTML = DOMPurify.sanitize(marked.parse(next.text, { async: false }) as string);
      for (const element of node.querySelectorAll<HTMLElement>('[title]')) {
        element.dataset.tooltip = element.getAttribute('title') || '';
        element.removeAttribute('title');
      }
      for (const pre of node.querySelectorAll('pre')) {
        const code = pre.querySelector('code');
        if (!code) continue;
        const language = Array.from(code.classList).find(name => name.startsWith('language-'))?.slice(9) || '';
        code.innerHTML = DOMPurify.sanitize(highlightCode(code.textContent || '', language));
        code.classList.add('syntax-code');
        const block = document.createElement('div');
        block.className = 'code-block';
        const toolbar = document.createElement('div');
        toolbar.className = 'code-block-toolbar';
        const button = document.createElement('button');
        button.type = 'button';
        button.className = 'code-copy-button';
        function setLabel(label: string, copied = false) {
          button.dataset.tooltip = label;
          button.setAttribute('aria-label', label);
          button.innerHTML = copied ? checkIcon : copyIcon;
          button.classList.toggle('copied', copied);
        }
        setLabel(next.copy);
        button.addEventListener('click', async () => {
          button.disabled = true;
          let label: string;
          let copied = false;
          try {
            await writeClipboard(code.textContent || '');
            label = next.copied;
            copied = true;
          } catch {
            label = next.failed;
          }
          if (!node.contains(button)) return;
          button.disabled = false;
          setLabel(label, copied);
          feedback = label;
          const timer = setTimeout(() => {
            setLabel(next.copy);
            timers.delete(timer);
          }, 2000);
          timers.add(timer);
        });
        toolbar.append(button);
        pre.before(block);
        block.append(toolbar, pre);
      }
    }
    render(options);
    return { update: render, destroy: clearTimers };
  }
</script>

<div class="markdown-content" use:renderContent={{ text, copy: $t('copyCode'), copied: $t('copied'), failed: $t('copyFailed') }}></div>
<span class="copy-feedback" role="status" aria-live="polite">{feedback}</span>

<style>
  .markdown-content :global(.code-block) {
    margin: 0.85rem 0;
    border: 1px solid var(--border-color);
    border-radius: 8px;
    background: var(--bg-code);
    overflow: hidden;
  }
  .markdown-content :global(.code-block-toolbar) {
    display: flex;
    justify-content: flex-end;
    padding: 0.3rem 0.45rem 0;
  }
  .markdown-content :global(.code-block pre) {
    margin: 0;
    border: none;
    border-radius: 0;
    padding-top: 0.25rem;
  }
  .markdown-content :global(.code-copy-button) {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    padding: 0;
    border: 1px solid transparent;
    border-radius: 5px;
    background: transparent;
    color: var(--text-secondary);
    cursor: pointer;
    transition: background 0.15s, color 0.15s;
  }
  .markdown-content :global(.code-copy-button:hover) {
    background: var(--bg-hover);
    color: var(--text-main);
  }
  .markdown-content :global(.code-copy-button:focus-visible) {
    outline: 2px solid var(--color-primary);
    outline-offset: 2px;
  }
  .markdown-content :global(.code-copy-button.copied) { color: var(--color-success); }
  .copy-feedback {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip-path: inset(50%);
    white-space: nowrap;
  }
</style>
