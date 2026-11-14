import { beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { createPinia, setActivePinia } from 'pinia';
import { backend, ThemeMode } from '@/services/backend';
import App from './App.vue';

vi.mock('@/services/backend', () => ({
  ThemeMode: {
    $zero: '',
    ThemeSystem: 'system',
    ThemeLight: 'light',
    ThemeDark: 'dark',
  },
  backend: {
    getAppInfo: vi.fn(),
    getSettings: vi.fn(),
    updateSettings: vi.fn(),
    listProjects: vi.fn(),
  },
}));

describe('App.vue', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();

    vi.mocked(backend.getAppInfo).mockResolvedValue({
      name: 'ProjectHub',
      version: '0.1.0',
      configDir: '/home/dev/.config/projecthub',
      os: 'linux',
      arch: 'amd64',
    });
    vi.mocked(backend.getSettings).mockResolvedValue({
      schemaVersion: 1,
      defaultEditor: 'vscode',
      preferredTerminal: 'ghostty',
      preferredFileManager: 'auto',
      theme: ThemeMode.ThemeDark,
      window: { width: 440, height: 680, rememberSize: true, alwaysOnTop: false },
    });
    vi.mocked(backend.listProjects).mockResolvedValue([]);
    vi.mocked(backend.updateSettings).mockImplementation(async (s) => s);
  });

  it('renders empty state when no projects are registered', async () => {
    const wrapper = mount(App, {
      global: { plugins: [createPinia()] },
    });
    await flushPromises();

    expect(wrapper.text()).toContain('ProjectHub');
    expect(wrapper.text()).toContain('No projects registered yet');
    expect(wrapper.text()).toContain('/home/dev/.config/projecthub');
  });

  it('switches between projects and settings views', async () => {
    const wrapper = mount(App, {
      global: { plugins: [createPinia()] },
    });
    await flushPromises();

    await wrapper.get('button[aria-label="Settings"]').trigger('click');
    expect(wrapper.text()).toContain('Preferences');

    await wrapper.get('button[aria-label="Back to projects"]').trigger('click');
    expect(wrapper.text()).toContain('No projects registered yet');
  });

  it('disables the header theme button while a save is in flight', async () => {
    let resolveSave!: (value: Awaited<ReturnType<typeof backend.getSettings>>) => void;
    vi.mocked(backend.updateSettings).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveSave = resolve;
        }),
    );

    const wrapper = mount(App, {
      global: { plugins: [createPinia()] },
    });
    await flushPromises();

    const themeButton = wrapper.get('button[aria-label^="Theme:"]');
    expect((themeButton.element as HTMLButtonElement).disabled).toBe(false);

    await themeButton.trigger('click');
    expect((themeButton.element as HTMLButtonElement).disabled).toBe(true);

    resolveSave({
      schemaVersion: 1,
      defaultEditor: 'vscode',
      preferredTerminal: 'ghostty',
      preferredFileManager: 'auto',
      theme: ThemeMode.ThemeLight,
      window: { width: 440, height: 680, rememberSize: true, alwaysOnTop: false },
    });
    await flushPromises();

    expect((themeButton.element as HTMLButtonElement).disabled).toBe(false);
  });
});
