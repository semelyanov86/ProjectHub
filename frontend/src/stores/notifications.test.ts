import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { useNotificationsStore } from './notifications';

let pinia: ReturnType<typeof createPinia>;

beforeEach(() => {
  vi.useFakeTimers();
  pinia = createPinia();
  setActivePinia(pinia);
});

afterEach(() => {
  disposePinia(pinia);
  vi.useRealTimers();
});

describe('notifications', () => {
  it.each([
    ['info', 4000],
    ['success', 4000],
    ['warning', 6000],
    ['error', 8000],
  ] as const)('dismisses %s after its default timeout of %i ms', (tone, timeout) => {
    const store = useNotificationsStore();
    const id = store[tone]('Saved', 'Details');
    expect(store.items).toEqual([{ id, tone, title: 'Saved', detail: 'Details' }]);
    vi.advanceTimersByTime(timeout - 1);
    expect(store.items).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(store.items).toHaveLength(0);
  });

  it('defaults to info and allows a custom timeout', () => {
    const store = useNotificationsStore();
    store.notify({ title: 'Ready', timeoutMs: 1000 });
    expect(store.items[0]?.tone).toBe('info');
    vi.advanceTimersByTime(1000);
    expect(store.items).toHaveLength(0);
  });

  it('keeps timeoutMs=0 notifications sticky without a timer', () => {
    const store = useNotificationsStore();
    store.notify({ title: 'Sticky', timeoutMs: 0 });
    vi.advanceTimersByTime(100000);
    expect(store.items).toHaveLength(1);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('deduplicates identical tone, title and detail and restarts the timer', () => {
    const store = useNotificationsStore();
    const input = { title: 'Ready', detail: 'Project loaded' };
    const id = store.notify(input);
    vi.advanceTimersByTime(3000);
    expect(store.notify(input)).toBe(id);
    expect(store.items).toHaveLength(1);
    vi.advanceTimersByTime(3999);
    expect(store.items).toHaveLength(1);
    vi.advanceTimersByTime(1);
    expect(store.items).toHaveLength(0);
  });

  it('does not deduplicate different tones or details', () => {
    const store = useNotificationsStore();
    const ids = [store.info('Ready'), store.error('Ready'), store.info('Ready', 'Details')];
    expect(new Set(ids).size).toBe(3);
    expect(store.items).toHaveLength(3);
  });

  it('can replace a duplicate timer with a sticky notification', () => {
    const store = useNotificationsStore();
    const id = store.info('Ready');
    expect(store.notify({ title: 'Ready', timeoutMs: 0 })).toBe(id);
    expect(vi.getTimerCount()).toBe(0);
    vi.advanceTimersByTime(10000);
    expect(store.items).toHaveLength(1);
  });

  it('keeps at most four notifications and clears the evicted oldest timer', () => {
    const store = useNotificationsStore();
    const ids = Array.from({ length: 5 }, (_, index) => store.info(`Message ${index}`));
    expect(store.items.map((item) => item.id)).toEqual(ids.slice(1));
    expect(vi.getTimerCount()).toBe(4);
  });

  it('dismiss clears the associated timer and tolerates unknown ids', () => {
    const store = useNotificationsStore();
    const id = store.info('Ready');
    store.dismiss(id);
    store.dismiss(-1);
    expect(store.items).toHaveLength(0);
    expect(vi.getTimerCount()).toBe(0);
  });

  it('clear removes all items and timers while ids continue increasing', () => {
    const store = useNotificationsStore();
    const first = store.info('First');
    store.error('Second');
    store.clear();
    expect(store.items).toHaveLength(0);
    expect(vi.getTimerCount()).toBe(0);
    expect(store.info('Third')).toBeGreaterThan(first);
  });

  it('cleans timers when the store is disposed', () => {
    const store = useNotificationsStore();
    store.info('Ready');
    store.$dispose();
    expect(vi.getTimerCount()).toBe(0);
  });
});
