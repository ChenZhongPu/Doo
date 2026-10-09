import hljs from 'highlight.js/lib/common';

export function highlightCode(text: string, language: string): string {
  const name = language.toLowerCase();
  if (hljs.getLanguage(name)) {
    try {
      return hljs.highlight(text, { language: name, ignoreIllegals: true }).value;
    } catch {
      // Keep incomplete or unsupported code readable while streaming.
    }
  }
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
