import { describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import { ThemeMode } from '@/services/backend';
import type { Settings } from '@/services/backend';
import SettingsView from './SettingsView.vue';

vi.mock('@/services/backend', () => ({
  ThemeMode: {
    $zero: '',
    ThemeSystem: 'system',
    ThemeLight: 'light',
    ThemeDark: 'dark',
  },
}));

function baseSettings(): Settings {
  return {
    schemaVersion: 1,
    defaultEditor: 'vscode',
    preferredTerminal: 'ghostty',
    preferredFileManager: 'auto',
    theme: ThemeMode.ThemeDark,
    window: {
      width: 440,
      height: 680,
      rememberSize: true,
      alwaysOnTop: false,
    },
  };
}

describe('SettingsView.vue', () => {
  it('shows Saved notice only when saveSettings resolves true', async () => {
    const saveSettings = vi.fn().mockResolvedValue(true);
    const wrapper = mount(SettingsView, {
      props: {
        settings: baseSettings(),
        configDir: '/home/dev/.config/projecthub',
        saving: false,
        canSave: true,
        saveSettings,
      },
    });

    await wrapper.get('form').trigger('submit');
    await flushPromises();

    expect(saveSettings).toHaveBeenCalledTimes(1);
    expect(wrapper.text()).toContain('Saved');
  });

  it('does not show Saved notice when saveSettings fails', async () => {
    const saveSettings = vi.fn().mockResolvedValue(false);
    const wrapper = mount(SettingsView, {
      props: {
        settings: baseSettings(),
        configDir: '/home/dev/.config/projecthub',
        saving: false,
        canSave: true,
        saveSettings,
      },
    });

    await wrapper.get('form').trigger('submit');
    await flushPromises();

    expect(saveSettings).toHaveBeenCalledTimes(1);
    expect(wrapper.text()).not.toContain('Saved');
  });

  it('disables submit button when canSave is false', () => {
    const wrapper = mount(SettingsView, {
      props: {
        settings: baseSettings(),
        configDir: '/home/dev/.config/projecthub',
        saving: false,
        canSave: false,
        saveSettings: vi.fn(),
      },
    });

    const button = wrapper.get('button[type="submit"]');
    expect((button.element as HTMLButtonElement).disabled).toBe(true);
  });

  it('preserves unsaved form fields when external settings update', async () => {
    const initial = baseSettings();
    const wrapper = mount(SettingsView, {
      props: {
        settings: initial,
        configDir: '/home/dev/.config/projecthub',
        saving: false,
        canSave: true,
        saveSettings: vi.fn().mockResolvedValue(true),
      },
    });

    const editorInput = wrapper.get('input[type="text"]');
    await editorInput.setValue('zed');

    await wrapper.setProps({
      settings: {
        ...initial,
        theme: ThemeMode.ThemeLight,
        window: { ...initial.window },
      },
    });

    expect((editorInput.element as HTMLInputElement).value).toBe('zed');
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe(ThemeMode.ThemeLight);
  });
});
