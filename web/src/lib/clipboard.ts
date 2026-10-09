export async function writeClipboard(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text);
    return;
  }
  // HTTP access on a remote host may not expose the Clipboard API.
  const previousFocus = document.activeElement;
  const input = document.createElement('textarea');
  input.value = text;
  input.style.cssText = 'position:fixed;opacity:0;pointer-events:none';
  document.body.append(input);
  try {
    input.focus({ preventScroll: true });
    input.select();
    if (!document.execCommand('copy')) throw new Error('Copy failed');
  } finally {
    input.remove();
    if (previousFocus instanceof HTMLElement) previousFocus.focus({ preventScroll: true });
  }
}
