<script lang="ts">
  import { t } from './i18n';
</script>

<span class="waiting-dots" role="status" aria-label={$t('waitingOutput')}>
  <span class="dot" aria-hidden="true"></span>
  <span class="dot" aria-hidden="true"></span>
  <span class="dot" aria-hidden="true"></span>
</span>

<style>
  .waiting-dots {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 1.6em;
    vertical-align: middle;
    color: var(--text-muted);
  }
  .dot {
    width: 4px;
    height: 4px;
    border-radius: 50%;
    background: currentColor;
    opacity: .35;
    animation: waiting-dot 1.2s ease-in-out infinite;
  }
  .dot:nth-child(2) { animation-delay: .16s; }
  .dot:nth-child(3) { animation-delay: .32s; }
  @keyframes waiting-dot {
    0%, 60%, 100% { transform: translateY(0); opacity: .35; }
    30% { transform: translateY(-3px); opacity: 1; }
  }
  @media (prefers-reduced-motion: reduce) {
    .dot { animation: none; opacity: .65; }
  }
</style>
