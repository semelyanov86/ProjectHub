import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { nextTick } from 'vue';
import { mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { useNotificationsStore } from '@/stores/notifications';
import ToastHost from './ToastHost.vue';

let pinia: ReturnType<typeof createPinia>;
let wrapper: VueWrapper | undefined;

beforeEach(() => {
  pinia = createPinia();
  setActivePinia(pinia);
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  disposePinia(pinia);
  document.body.innerHTML = '';
});

describe('ToastHost', () => {
  it('renders nothing when the store is empty', () => {
    wrapper = mount(ToastHost, { attachTo: document.body });
    expect(wrapper.find('[role="region"]').exists()).toBe(false);
  });

  it('renders titles and details using alert for errors and status for other tones', () => {
    const store = useNotificationsStore();
    store.info('Opened', 'Project is ready');
    store.success('Saved');
    store.warning('Unavailable');
    store.error('Failed', 'Directory is missing');
    wrapper = mount(ToastHost, { attachTo: document.body });
    expect(wrapper.get('[role="region"]').attributes('aria-label')).toBe('Notifications');
    expect(wrapper.findAll('[role="status"]')).toHaveLength(3);
    expect(wrapper.get('[role="alert"]').text()).toContain('Failed');
    expect(wrapper.text()).toContain('Project is ready');
    expect(wrapper.get('[title="Directory is missing"]').text()).toBe('Directory is missing');
  });

  it('dismisses the selected toast and hides the region when the last toast is removed', async () => {
    const store = useNotificationsStore();
    store.info('First');
    const second = store.error('Second');
    wrapper = mount(ToastHost, { attachTo: document.body });
    await wrapper.get('[role="status"] [aria-label="Dismiss notification"]').trigger('click');
    expect(store.items.map((item) => item.id)).toEqual([second]);
    expect(wrapper.text()).not.toContain('First');
    await wrapper.get('[aria-label="Dismiss notification"]').trigger('click');
    await nextTick();
    expect(store.items).toHaveLength(0);
    expect(wrapper.find('[role="region"]').exists()).toBe(false);
  });
});
