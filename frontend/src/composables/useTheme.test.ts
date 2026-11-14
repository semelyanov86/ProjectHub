import { beforeEach, describe, expect, it, vi } from 'vitest';
import { defineComponent, nextTick, ref } from 'vue';
import { mount } from '@vue/test-utils';
import { ThemeMode } from '@/services/backend';
import { useTheme } from './useTheme';

vi.mock('@/services/backend', () => ({
  ThemeMode: {
    $zero: '',
    ThemeSystem: 'system',
    ThemeLight: 'light',
    ThemeDark: 'dark',
  },
}));

describe('useTheme', () => {
  let listeners: Array<() => void> = [];
  let systemDark = false;

  beforeEach(() => {
    listeners = [];
    systemDark = false;
    document.documentElement.removeAttribute('data-theme');

    vi.stubGlobal(
      'matchMedia',
      vi.fn().mockImplementation(() => ({
        get matches() {
          return systemDark;
        },
        media: '(prefers-color-scheme: dark)',
        addEventListener: (_event: string, cb: () => void) => {
          listeners.push(cb);
        },
        removeEventListener: (_event: string, cb: () => void) => {
          listeners = listeners.filter((item) => item !== cb);
        },
      })),
    );
  });

  it('applies light, dark, and reactive system themes to documentElement data-theme', async () => {
    const mode = ref<ThemeMode>(ThemeMode.ThemeLight);

    const Harness = defineComponent({
      setup() {
        return useTheme(mode);
      },
      template: '<div />',
    });

    const wrapper = mount(Harness);
    await nextTick();
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');

    mode.value = ThemeMode.ThemeDark;
    await nextTick();
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');

    mode.value = ThemeMode.ThemeSystem;
    await nextTick();
    expect(document.documentElement.getAttribute('data-theme')).toBe('light');

    systemDark = true;
    listeners.forEach((cb) => cb());
    await nextTick();
    expect(document.documentElement.getAttribute('data-theme')).toBe('dark');

    wrapper.unmount();
  });
});
