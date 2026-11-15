import { afterEach, describe, expect, it } from 'vitest';
import { mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import ProjectTabs from './ProjectTabs.vue';
let wrapper: VueWrapper | undefined;
function render() {
  const view = mount(ProjectTabs, {
    attachTo: document.body,
    props: { modelValue: 'all', counts: { all: 7, favorites: 2, recent: 3 } },
  });
  wrapper = view;
  return view;
}
afterEach(() => {
  wrapper?.unmount();
  document.body.innerHTML = '';
});
describe('ProjectTabs', () => {
  it('renders counts and marks only the active tab selected', async () => {
    const view = render();
    expect(view.findAll('[role="tab"]').map((tab) => tab.text())).toEqual([
      'All 7',
      'Favorites 2',
      'Recent 3',
    ]);
    expect(view.get('[data-tab="all"]').attributes('aria-selected')).toBe('true');
    expect(view.get('[data-tab="favorites"]').attributes('aria-selected')).toBe('false');
    await view.setProps({ modelValue: 'favorites' });
    expect(view.get('[data-tab="favorites"]').attributes('aria-selected')).toBe('true');
    expect(view.get('[data-tab="all"]').attributes('aria-selected')).toBe('false');
  });
  it('clicking a tab emits its value', async () => {
    const view = render();
    await view.get('[data-tab="favorites"]').trigger('click');
    expect(view.emitted('update:modelValue')).toEqual([['favorites']]);
  });
  it('arrow keys emit neighboring tabs, wrap and move focus', async () => {
    const view = render();
    await view.get('[role="tablist"]').trigger('keydown', { key: 'ArrowRight' });
    expect(document.activeElement).toBe(view.get('[data-tab="favorites"]').element);
    await view.setProps({ modelValue: 'favorites' });
    await view.get('[role="tablist"]').trigger('keydown', { key: 'ArrowLeft' });
    await view.setProps({ modelValue: 'all' });
    await view.get('[role="tablist"]').trigger('keydown', { key: 'ArrowLeft' });
    expect(view.emitted('update:modelValue')).toEqual([['favorites'], ['all'], ['recent']]);
    expect(document.activeElement).toBe(view.get('[data-tab="recent"]').element);
  });
});
