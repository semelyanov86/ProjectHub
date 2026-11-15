import { onScopeDispose, ref } from 'vue';
import { defineStore } from 'pinia';

export type ToastTone = 'info' | 'success' | 'warning' | 'error';

export interface Toast {
  id: number;
  tone: ToastTone;
  title: string;
  detail?: string;
}

export interface NotifyInput {
  tone?: ToastTone;
  title: string;
  detail?: string;
  timeoutMs?: number;
}

const defaultTimeouts: Record<ToastTone, number> = {
  info: 4000,
  success: 4000,
  warning: 6000,
  error: 8000,
};

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref<Toast[]>([]);
  const timers = new Map<number, ReturnType<typeof setTimeout>>();
  let nextId = 0;

  function clearTimer(id: number) {
    const timer = timers.get(id);
    if (timer !== undefined) clearTimeout(timer);
    timers.delete(id);
  }

  function dismiss(id: number) {
    clearTimer(id);
    items.value = items.value.filter((item) => item.id !== id);
  }

  function clear() {
    for (const id of timers.keys()) clearTimer(id);
    items.value = [];
  }

  function notify(input: NotifyInput): number {
    const tone = input.tone ?? 'info';
    const existing = items.value.find(
      (item) => item.tone === tone && item.title === input.title && item.detail === input.detail,
    );
    const id = existing?.id ?? ++nextId;
    if (!existing) {
      items.value.push({ id, tone, title: input.title, detail: input.detail });
      if (items.value.length > 4) dismiss(items.value[0]!.id);
    }
    clearTimer(id);
    const timeout = input.timeoutMs ?? defaultTimeouts[tone];
    if (timeout !== 0)
      timers.set(
        id,
        setTimeout(() => dismiss(id), timeout),
      );
    return id;
  }

  function info(title: string, detail?: string): number {
    return notify({ tone: 'info', title, detail });
  }

  function success(title: string, detail?: string): number {
    return notify({ tone: 'success', title, detail });
  }

  function warning(title: string, detail?: string): number {
    return notify({ tone: 'warning', title, detail });
  }

  function error(title: string, detail?: string): number {
    return notify({ tone: 'error', title, detail });
  }

  onScopeDispose(clear);

  return { items, notify, info, success, warning, error, dismiss, clear };
});
