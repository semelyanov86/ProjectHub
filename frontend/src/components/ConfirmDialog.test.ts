import { afterEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import ConfirmDialog from './ConfirmDialog.vue';

let wrapper: VueWrapper | undefined;

function render(onConfirm: () => Promise<unknown>, errorHandler?: (error: unknown) => void) {
  wrapper = mount(ConfirmDialog, {
    attachTo: document.body,
    props: {
      open: true,
      title: 'Remove project?',
      message: 'The project will be removed. Files on disk are not touched.',
      confirmLabel: 'Remove',
      onConfirm,
    },
    global: { config: { errorHandler } },
  });
  return wrapper;
}

function button(label: string): HTMLButtonElement {
  return Array.from(document.body.querySelectorAll('button')).find(
    (element) => element.textContent?.trim() === label,
  )!;
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('ConfirmDialog', () => {
  it('renders the confirmation message and label in a dialog', () => {
    render(vi.fn().mockResolvedValue(undefined));
    const dialog = document.body.querySelector('[role="dialog"]')!;
    expect(dialog.textContent).toContain('Remove project?');
    expect(dialog.textContent).toContain(
      'The project will be removed. Files on disk are not touched.',
    );
    expect(button('Remove')).toBeDefined();
  });

  it.each(['resolve', 'reject'] as const)(
    'waits for onConfirm, disables buttons while pending and closes after %s',
    async (outcome) => {
      let resolve!: () => void;
      let reject!: (error: Error) => void;
      const pending = new Promise<void>((res, rej) => {
        resolve = res;
        reject = rej;
      });
      const onConfirm = vi.fn(() => pending);
      const onError = vi.fn();
      const view = render(onConfirm, onError);
      button('Remove').click();
      await flushPromises();
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(
        Array.from(document.body.querySelectorAll('button')).every((item) => item.disabled),
      ).toBe(true);
      button('Remove').click();
      button('Cancel').click();
      expect(onConfirm).toHaveBeenCalledOnce();
      expect(view.emitted('close')).toBeUndefined();
      const error = new Error('Removal failed');
      if (outcome === 'resolve') resolve();
      else reject(error);
      await flushPromises();
      expect(view.emitted('close')).toEqual([[]]);
      expect(button('Remove').disabled).toBe(false);
      expect(button('Cancel').disabled).toBe(false);
      if (outcome === 'reject') {
        expect(onError).toHaveBeenCalledOnce();
        expect(onError.mock.calls[0]![0]).toBe(error);
      } else expect(onError).not.toHaveBeenCalled();
    },
  );

  it('Cancel closes without calling onConfirm', async () => {
    const onConfirm = vi.fn().mockResolvedValue(undefined);
    const view = render(onConfirm);
    button('Cancel').click();
    await flushPromises();
    expect(view.emitted('close')).toEqual([[]]);
    expect(onConfirm).not.toHaveBeenCalled();
  });
});
