import { afterEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';
import { mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import AppMenu from './AppMenu.vue';
import type { MenuItem, MenuPosition } from './menu';

const items: MenuItem[] = [
  { id: 'missing', label: 'Unavailable', disabled: true, hint: 'Install this editor' },
  { id: 'code', label: 'VS Code', shortcut: 'Enter' },
  { id: 'remove', label: 'Remove', danger: true, separatorBefore: true },
];
let wrapper: VueWrapper | undefined;

async function openMenu(
  props: { open?: boolean; items?: MenuItem[]; position?: MenuPosition } = {},
) {
  wrapper = mount(AppMenu, {
    attachTo: document.body,
    props: {
      open: true,
      items,
      position: { x: 20, y: 30 },
      label: 'Editors',
      title: 'Open with',
      ...props,
    },
  });
  await nextTick();
  return wrapper;
}

function menu() {
  return document.querySelector<HTMLElement>('[role="menu"]')!;
}

function buttons() {
  return Array.from(document.querySelectorAll<HTMLButtonElement>('[role="menuitem"]'));
}

function key(value: string) {
  document.activeElement?.dispatchEvent(
    new KeyboardEvent('keydown', { key: value, bubbles: true, cancelable: true }),
  );
}

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  document.body.innerHTML = '';
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('AppMenu', () => {
  it('teleports title, items, hints, shortcut and separator only while open', async () => {
    const view = await openMenu({ open: false });
    expect(document.querySelector('[role="menu"]')).toBeNull();
    await view.setProps({ open: true });
    expect(menu().parentElement).toBe(document.body);
    expect(menu().getAttribute('aria-label')).toBe('Editors');
    expect(menu().textContent).toContain('Open with');
    expect(menu().textContent).toContain('Install this editor');
    expect(menu().textContent).toContain('Enter');
    expect(buttons()[0]!.title).toBe('Install this editor');
    expect(menu().querySelector('[role="separator"]')).not.toBeNull();
    await view.setProps({ open: false });
    expect(document.querySelector('[role="menu"]')).toBeNull();
  });

  it('focuses the first enabled item and cycles across every item with arrows, Home and End', async () => {
    await openMenu();
    const entries = buttons();
    expect(document.activeElement).toBe(entries[1]);
    key('ArrowDown');
    expect(document.activeElement).toBe(entries[2]);
    key('ArrowDown');
    expect(document.activeElement).toBe(entries[0]);
    key('ArrowUp');
    expect(document.activeElement).toBe(entries[2]);
    key('Home');
    expect(document.activeElement).toBe(entries[0]);
    key('End');
    expect(document.activeElement).toBe(entries[2]);
  });

  it('focuses the first item when all are disabled', async () => {
    await openMenu({ items: items.map((item) => ({ ...item, disabled: true })) });
    expect(document.activeElement).toBe(buttons()[0]);
  });

  it('emits selection for Enter, Space and clicks on enabled items without closing', async () => {
    const view = await openMenu();
    key('Enter');
    key(' ');
    buttons()[2]!.click();
    expect(view.emitted('select')).toEqual([['code'], ['code'], ['remove']]);
    expect(view.emitted('close')).toBeUndefined();
  });

  it('keeps disabled items focusable but does not select them by keyboard or click', async () => {
    const view = await openMenu();
    const disabled = buttons()[0]!;
    disabled.focus();
    expect(document.activeElement).toBe(disabled);
    expect(disabled.getAttribute('aria-disabled')).toBe('true');
    expect(disabled.disabled).toBe(false);
    key('Enter');
    key(' ');
    disabled.click();
    expect(view.emitted('select')).toBeUndefined();
  });

  it('moves focus to the hovered item', async () => {
    await openMenu();
    buttons()[0]!.dispatchEvent(new MouseEvent('mouseenter'));
    expect(document.activeElement).toBe(buttons()[0]);
  });

  it('closes on Escape, prevents default and stops propagation to the document', async () => {
    const view = await openMenu();
    const listener = vi.fn();
    document.addEventListener('keydown', listener);
    try {
      const event = new KeyboardEvent('keydown', {
        key: 'Escape',
        bubbles: true,
        cancelable: true,
      });
      document.activeElement!.dispatchEvent(event);
      expect(event.defaultPrevented).toBe(true);
      expect(view.emitted('close')).toEqual([[]]);
      expect(listener).not.toHaveBeenCalled();
    } finally {
      document.removeEventListener('keydown', listener);
    }
  });

  it('closes on Tab', async () => {
    const view = await openMenu();
    key('Tab');
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('closes only for pointerdown outside the menu', async () => {
    const view = await openMenu();
    buttons()[1]!.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    expect(view.emitted('close')).toBeUndefined();
    document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    expect(view.emitted('close')).toEqual([[]]);
  });

  it.each(['resize', 'blur', 'scroll'])('closes on window %s', async (type) => {
    const view = await openMenu();
    window.dispatchEvent(new Event(type));
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('closes on scrolling a nested element through the capture listener', async () => {
    const view = await openMenu();
    const scroller = document.createElement('div');
    document.body.append(scroller);
    scroller.dispatchEvent(new Event('scroll'));
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('removes every listener between opens and on unmount', async () => {
    const view = await openMenu();
    for (let index = 0; index < 2; index++) {
      await view.setProps({ open: false });
      document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
      for (const type of ['resize', 'blur', 'scroll']) window.dispatchEvent(new Event(type));
      expect(view.emitted('close')).toBeUndefined();
      await view.setProps({ open: true });
    }
    view.unmount();
    wrapper = undefined;
    document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    for (const type of ['resize', 'blur', 'scroll']) window.dispatchEvent(new Event(type));
    expect(view.emitted('close')).toBeUndefined();
  });

  it('restores focus after close and unmount', async () => {
    const trigger = document.createElement('button');
    document.body.append(trigger);
    trigger.focus();
    const view = await openMenu();
    await view.setProps({ open: false });
    expect(document.activeElement).toBe(trigger);
    await view.setProps({ open: true });
    view.unmount();
    wrapper = undefined;
    expect(document.activeElement).toBe(trigger);
  });

  it('does not steal focus from a newly opened dialog or a disconnected trigger', async () => {
    const trigger = document.createElement('button');
    const dialogInput = document.createElement('input');
    document.body.append(trigger, dialogInput);
    trigger.focus();
    const view = await openMenu();
    dialogInput.focus();
    await view.setProps({ open: false });
    expect(document.activeElement).toBe(dialogInput);
    trigger.focus();
    await view.setProps({ open: true });
    trigger.remove();
    const focus = vi.spyOn(trigger, 'focus');
    await view.setProps({ open: false });
    expect(focus).not.toHaveBeenCalled();
  });

  it('flips upward and leftward near the viewport edges and clamps to an 8px margin', async () => {
    vi.stubGlobal('innerWidth', 440);
    vi.stubGlobal('innerHeight', 650);
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 0, 220, 160),
    );
    const view = await openMenu({ position: { x: 430, y: 640 } });
    await nextTick();
    expect(menu().style.left).toBe('210px');
    expect(menu().style.top).toBe('480px');
    await view.setProps({ position: { x: -50, y: -50 } });
    await nextTick();
    expect(menu().style.left).toBe('8px');
    expect(menu().style.top).toBe('8px');
  });

  it('flips entirely above the anchor when avoidTop is supplied and clamps to the margin', async () => {
    vi.stubGlobal('innerHeight', 650);
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 0, 220, 160),
    );
    const view = await openMenu({ position: { x: 20, y: 600, avoidTop: 540 } });
    await nextTick();
    expect(menu().style.top).toBe('380px');
    expect(Number.parseFloat(menu().style.top) + 160).toBeLessThanOrEqual(540);
    await view.setProps({ position: { x: 20, y: 600, avoidTop: 100 } });
    expect(menu().style.top).toBe('8px');
  });

  it('keeps the menu below y when it fits even with avoidTop supplied', async () => {
    vi.stubGlobal('innerHeight', 650);
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 0, 220, 160),
    );
    await openMenu({ position: { x: 20, y: 200, avoidTop: 140 } });
    await nextTick();
    expect(menu().style.top).toBe('200px');
  });
});
