<script lang="ts">
  import { onMount } from 'svelte';

  let tooltip: HTMLDivElement;

  onMount(() => {
    const node = tooltip;
    if (typeof node.showPopover !== 'function') node.removeAttribute('popover');
    let hovered: HTMLElement | null = null;
    let focused: HTMLElement | null = null;
    let active: HTMLElement | null = null;
    let dismissed: HTMLElement | null = null;
    let previousDescription: string | null = null;
    let timer: ReturnType<typeof setTimeout> | undefined;

    function findTarget(target: EventTarget | null): HTMLElement | null {
      return target instanceof Element ? target.closest<HTMLElement>('[data-tooltip]') : null;
    }

    function hide() {
      clearTimeout(timer);
      if (active) {
        if (previousDescription === null) active.removeAttribute('aria-describedby');
        else active.setAttribute('aria-describedby', previousDescription);
      }
      active = null;
      if (typeof node.hidePopover === 'function' && node.matches(':popover-open')) node.hidePopover();
      node.hidden = true;
    }

    function position() {
      if (!active?.isConnected) { hide(); return; }
      const rect = active.getBoundingClientRect();
      if (rect.bottom <= 0 || rect.top >= window.innerHeight || rect.right <= 0 || rect.left >= window.innerWidth) { hide(); return; }
      const bounds = node.getBoundingClientRect();
      const left = Math.max(8, Math.min(rect.left + rect.width / 2 - bounds.width / 2, window.innerWidth - bounds.width - 8));
      node.style.left = `${left}px`;
      // Keep the placement above the control, with a viewport clamp near its top edge.
      node.style.top = `${Math.max(8, rect.top - bounds.height - 8)}px`;
      node.style.setProperty('--tooltip-arrow-x', `${Math.max(10, Math.min(bounds.width - 10, rect.left + rect.width / 2 - left))}px`);
    }

    function show(target: HTMLElement | null) {
      if (!target || target === dismissed || !target.isConnected || !target.dataset.tooltip?.trim()) { hide(); return; }
      if (active !== target) {
        hide();
        active = target;
        previousDescription = target.getAttribute('aria-describedby');
        target.setAttribute('aria-describedby', [previousDescription, 'doo-tooltip'].filter(Boolean).join(' '));
      }
      node.textContent = target.dataset.tooltip;
      node.hidden = false;
      if (typeof node.showPopover === 'function' && !node.matches(':popover-open')) node.showPopover();
      position();
    }

    function update(immediate = false) {
      clearTimeout(timer);
      const target = hovered ?? focused;
      if (active === target || !target || immediate) show(target);
      else { hide(); timer = setTimeout(() => show(target), 160); }
    }

    function pointerOver(event: PointerEvent) {
      const target = findTarget(event.target);
      if (target === hovered) return;
      hovered = target;
      dismissed = null;
      update();
    }
    function pointerOut(event: PointerEvent) {
      if (findTarget(event.relatedTarget) === hovered) return;
      hovered = null;
      if (focused !== dismissed) dismissed = null;
      update();
    }
    function focusIn(event: FocusEvent) {
      focused = findTarget(event.target);
      if (focused !== dismissed) dismissed = null;
      update(true);
    }
    function focusOut() { focused = null; if (hovered !== dismissed) dismissed = null; update(); }
    function dismiss() { dismissed = hovered ?? focused; hide(); }
    function keyDown(event: KeyboardEvent) { if (event.key === 'Escape') dismiss(); }

    // Copy feedback and other changing labels use the same tooltip without a second UI.
    const observer = new MutationObserver(records => {
      if (!active) return;
      if (!active.isConnected) { hide(); return; }
      if (records.some(record => record.target === active && record.attributeName === 'data-tooltip')) show(active);
    });
    observer.observe(document.body, { subtree: true, childList: true, attributes: true, attributeFilter: ['data-tooltip'] });
    document.addEventListener('pointerover', pointerOver);
    document.addEventListener('pointerout', pointerOut);
    document.addEventListener('focusin', focusIn);
    document.addEventListener('focusout', focusOut);
    document.addEventListener('pointerdown', dismiss);
    document.addEventListener('keydown', keyDown);
    window.addEventListener('scroll', dismiss, true);
    window.addEventListener('resize', dismiss);
    return () => {
      hide();
      observer.disconnect();
      document.removeEventListener('pointerover', pointerOver);
      document.removeEventListener('pointerout', pointerOut);
      document.removeEventListener('focusin', focusIn);
      document.removeEventListener('focusout', focusOut);
      document.removeEventListener('pointerdown', dismiss);
      document.removeEventListener('keydown', keyDown);
      window.removeEventListener('scroll', dismiss, true);
      window.removeEventListener('resize', dismiss);
    };
  });
</script>

<div bind:this={tooltip} id="doo-tooltip" class="tooltip" role="tooltip" popover="manual" hidden></div>

<style>
  .tooltip {
    position: fixed;
    inset: auto;
    margin: 0;
    z-index: 10000;
    width: max-content;
    max-width: min(320px, calc(100vw - 16px));
    padding: 7px 10px;
    border: 1px solid var(--border-color);
    border-radius: 7px;
    background: var(--bg-card);
    color: var(--text-main);
    box-shadow: var(--shadow-menu);
    font-size: 12px;
    font-weight: 400;
    line-height: 1.5;
    text-align: left;
    white-space: pre-line;
    overflow: visible;
    overflow-wrap: anywhere;
    pointer-events: none;
  }
  .tooltip::after {
    content: '';
    position: absolute;
    left: var(--tooltip-arrow-x, 50%);
    bottom: -4px;
    width: 6px;
    height: 6px;
    background: var(--bg-card);
    border-right: 1px solid var(--border-color);
    border-bottom: 1px solid var(--border-color);
    transform: translateX(-50%) rotate(45deg);
  }
</style>
