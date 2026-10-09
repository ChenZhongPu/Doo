<script lang="ts">
  import { dialogState, resolveDialog } from './dialog';
  import { t } from './i18n';
  import Icon from './Icon.svelte';

  function openDialog(node: HTMLDialogElement) {
    const previousFocus = document.activeElement;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    node.showModal();
    node.querySelector<HTMLButtonElement>('[data-initial-focus]')?.focus();
    return {
      destroy() {
        node.close();
        document.body.style.overflow = previousOverflow;
        if (previousFocus instanceof HTMLElement && previousFocus.isConnected) {
          previousFocus.focus({ preventScroll: true });
        }
      },
    };
  }
</script>

{#if $dialogState}
  {@const request = $dialogState}
  {#key request.id}
    <dialog
      class="app-dialog"
      role="alertdialog"
      aria-labelledby="doo-dialog-title"
      aria-describedby={request.detail ? 'doo-dialog-detail doo-dialog-message' : 'doo-dialog-message'}
      use:openDialog
      oncancel={event => { event.preventDefault(); resolveDialog(request.id, false); }}
      onkeydown={event => {
        if (event.key !== 'Tab') return;
        const buttons = [...event.currentTarget.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')];
        const first = buttons[0];
        const last = buttons[buttons.length - 1];
        if (event.shiftKey && document.activeElement === first) {
          event.preventDefault();
          last?.focus();
        } else if (!event.shiftKey && document.activeElement === last) {
          event.preventDefault();
          first?.focus();
        }
      }}
      onclick={event => {
        if (event.target !== event.currentTarget) return;
        const rect = event.currentTarget.getBoundingClientRect();
        if (event.clientX < rect.left || event.clientX > rect.right || event.clientY < rect.top || event.clientY > rect.bottom) {
          resolveDialog(request.id, false);
        }
      }}
    >
      <div class="dialog-heading">
        <h2 id="doo-dialog-title">{request.title}</h2>
        <button type="button" class="dialog-close" aria-label={$t('close')} onclick={() => resolveDialog(request.id, false)}><Icon name="close" size={16} /></button>
      </div>
      {#if request.detail}<div class="dialog-detail" id="doo-dialog-detail">{request.detail}</div>{/if}
      <p class="dialog-message" id="doo-dialog-message">{request.message}</p>
      <div class="dialog-actions">
        {#if request.kind === 'confirm'}
          <button type="button" class="dialog-button dialog-cancel" data-initial-focus onclick={() => resolveDialog(request.id, false)}>{$t('cancel')}</button>
          <button type="button" class="dialog-button dialog-confirm" class:destructive={request.destructive} onclick={() => resolveDialog(request.id, true)}>{request.destructive ? $t('delete') : $t('confirmAction')}</button>
        {:else}
          <button type="button" class="dialog-button dialog-confirm" data-initial-focus onclick={() => resolveDialog(request.id, true)}>{$t('gotIt')}</button>
        {/if}
      </div>
    </dialog>
  {/key}
{/if}

<style>
  .app-dialog {
    width: min(440px, calc(100vw - 32px));
    max-height: calc(100dvh - 48px);
    margin: auto;
    padding: 26px;
    border: 1px solid var(--border-color);
    border-radius: 14px;
    background: var(--bg-card);
    color: var(--text-main);
    box-shadow: var(--shadow-menu);
    overflow-y: auto;
    animation: dialog-enter 140ms ease-out;
  }
  .app-dialog::backdrop { background: var(--bg-backdrop); backdrop-filter: blur(4px); }
  .dialog-heading { display: flex; justify-content: space-between; align-items: flex-start; gap: 16px; }
  h2 { margin: 0; font-size: 20px; font-weight: 600; letter-spacing: -.025em; line-height: 1.4; }
  .dialog-close { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; padding: 0; margin: -2px -4px 0 0; flex-shrink: 0; border: none; border-radius: 5px; background: transparent; color: var(--text-muted); cursor: pointer; }
  .dialog-close:hover { background: var(--bg-hover); color: var(--text-main); }
  .dialog-detail { margin-top: 18px; padding: 12px 14px; border: 1px solid var(--border-subtle); border-radius: 7px; background: var(--bg-item); font-size: 13px; font-weight: 500; overflow-wrap: anywhere; }
  .dialog-message { margin: 14px 0 0; font-size: 13px; line-height: 1.7; color: var(--text-secondary); white-space: pre-wrap; overflow-wrap: anywhere; }
  .dialog-actions { display: flex; justify-content: flex-end; gap: 9px; margin-top: 26px; }
  .dialog-button { min-width: 76px; min-height: 36px; padding: 8px 16px; border: 1px solid transparent; border-radius: 7px; font-size: 13px; font-weight: 500; line-height: 1.4; cursor: pointer; transition: background 120ms; }
  .dialog-cancel { border-color: var(--border-color); background: var(--bg-card); color: var(--text-main); }
  .dialog-cancel:hover { background: var(--bg-hover); }
  .dialog-confirm { background: var(--bg-primary-button); color: var(--color-on-primary); }
  .dialog-confirm:hover { background: var(--bg-primary-button-hover); }
  .dialog-confirm.destructive { background: var(--bg-danger-button); color: var(--color-on-danger); }
  .dialog-confirm.destructive:hover { background: var(--bg-danger-button-hover); }
  @keyframes dialog-enter { from { opacity: 0; transform: translateY(6px); } to { opacity: 1; transform: translateY(0); } }
  @media (max-width: 480px) { .app-dialog { padding: 22px; } .dialog-actions { gap: 8px; } .dialog-button { flex: 1; } }
</style>
