import { afterEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';
import { mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import AppDialog from './AppDialog.vue';

let wrapper: VueWrapper | undefined;

async function openDialog(
  props: { open?: boolean; busy?: boolean; description?: string; closeOnBackdrop?: boolean } = {},
  body = '<input aria-label="Name"><button>Body action</button>',
  footer?: string,
) {
  wrapper = mount(AppDialog, {
    attachTo: document.body,
    props: { open: true, title: 'Add project', ...props },
    slots: { default: body, ...(footer ? { footer } : {}) },
  });
  await nextTick();
  return wrapper;
}

function panel() {
  return document.querySelector<HTMLElement>('[role="dialog"]')!;
}

function overlay() {
  return panel().parentElement!;
}

function key(value: string, shiftKey = false) {
  const event = new KeyboardEvent('keydown', {
    key: value,
    shiftKey,
    bubbles: true,
    cancelable: true,
  });
  document.activeElement!.dispatchEvent(event);
  return event;
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
});

describe('AppDialog', () => {
  it('renders only while open with accessible title and description ids', async () => {
    const view = await openDialog({ open: false, description: 'Choose a directory' });
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    await view.setProps({ open: true });
    expect(panel().getAttribute('aria-modal')).toBe('true');
    expect(document.getElementById(panel().getAttribute('aria-labelledby')!)?.textContent).toBe(
      'Add project',
    );
    expect(document.getElementById(panel().getAttribute('aria-describedby')!)?.textContent).toBe(
      'Choose a directory',
    );
    expect(overlay().parentElement).toBe(document.body);
    await view.setProps({ description: undefined });
    expect(panel().hasAttribute('aria-describedby')).toBe(false);
    expect(panel().querySelector('footer')).toBeNull();
  });

  it('focuses the data-autofocus element before other body controls', async () => {
    await openDialog({}, '<button>First</button><input data-autofocus aria-label="Directory">');
    expect(document.activeElement).toBe(panel().querySelector('[data-autofocus]'));
  });

  it('otherwise focuses the first enabled body control, then the panel when the body has none', async () => {
    const view = await openDialog(
      {},
      '<button disabled>Disabled</button><input aria-label="Name">',
    );
    expect(document.activeElement).toBe(panel().querySelector('input'));
    view.unmount();
    wrapper = undefined;
    await openDialog({}, '<p>Plain content</p>');
    expect(document.activeElement).toBe(panel());
  });

  it('wraps Tab and Shift+Tab across panel controls including the header and footer', async () => {
    await openDialog({}, undefined, '<button>Save</button>');
    const first = panel().querySelector<HTMLButtonElement>('[aria-label="Close"]')!;
    const last = panel().querySelector<HTMLButtonElement>('footer button')!;
    last.focus();
    expect(key('Tab').defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(first);
    expect(key('Tab', true).defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(last);
  });

  it('keeps focus on the panel when busy and there are no focusable controls', async () => {
    await openDialog({ busy: true }, '<p>Working</p>');
    expect(key('Tab').defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(panel());
  });

  it('emits close on Escape, preventing default and propagation', async () => {
    const view = await openDialog();
    const listener = vi.fn();
    document.addEventListener('keydown', listener);
    try {
      expect(key('Escape').defaultPrevented).toBe(true);
      expect(view.emitted('close')).toEqual([[]]);
      expect(listener).not.toHaveBeenCalled();
    } finally {
      document.removeEventListener('keydown', listener);
    }
  });

  it('blocks Escape, backdrop and the disabled close button while busy', async () => {
    const view = await openDialog({ busy: true });
    key('Escape');
    overlay().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    overlay().click();
    const close = panel().querySelector<HTMLButtonElement>('[aria-label="Close"]')!;
    expect(close.disabled).toBe(true);
    close.click();
    expect(view.emitted('close')).toBeUndefined();
  });

  it('requires pointerdown and click on the backdrop and ignores drags from the panel', async () => {
    const view = await openDialog();
    overlay().click();
    panel().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    overlay().click();
    panel().click();
    expect(view.emitted('close')).toBeUndefined();
    overlay().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    overlay().click();
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('respects closeOnBackdrop=false', async () => {
    const view = await openDialog({ closeOnBackdrop: false });
    overlay().dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    overlay().click();
    expect(view.emitted('close')).toBeUndefined();
  });

  it('emits close from its close button', async () => {
    const view = await openDialog();
    panel().querySelector<HTMLButtonElement>('[aria-label="Close"]')!.click();
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('restores focus to the trigger on close and on unmount', async () => {
    const trigger = document.createElement('button');
    document.body.append(trigger);
    trigger.focus();
    const view = await openDialog();
    await view.setProps({ open: false });
    expect(document.activeElement).toBe(trigger);
    await view.setProps({ open: true });
    view.unmount();
    wrapper = undefined;
    expect(document.activeElement).toBe(trigger);
  });
});
