import { writable } from 'svelte/store';

interface DialogOptions {
  title: string;
  message: string;
  detail?: string;
  destructive?: boolean;
}

interface DialogRequest extends DialogOptions {
  id: number;
  kind: 'confirm' | 'notice';
  resolve: (accepted: boolean) => void;
}

export const dialogState = writable<DialogRequest | null>(null);
let active: DialogRequest | null = null;
const queue: DialogRequest[] = [];
let nextId = 0;

function enqueue(options: DialogOptions, kind: DialogRequest['kind']): Promise<boolean> {
  return new Promise(resolve => {
    const request = { ...options, kind, id: ++nextId, resolve };
    if (active) {
      queue.push(request);
    } else {
      active = request;
      dialogState.set(active);
    }
  });
}

export function confirmAction(options: DialogOptions): Promise<boolean> {
  return enqueue(options, 'confirm');
}

export function notify(message: string, title: string): void {
  void enqueue({ message, title }, 'notice');
}

export function resolveDialog(id: number, accepted: boolean): void {
  if (!active || active.id !== id) return;
  const current = active;
  active = queue.shift() ?? null;
  dialogState.set(active);
  current.resolve(accepted);
}

export function dismissDialogs(): void {
  const pending = active ? [active, ...queue] : [...queue];
  active = null;
  queue.length = 0;
  dialogState.set(null);
  for (const request of pending) request.resolve(false);
}
