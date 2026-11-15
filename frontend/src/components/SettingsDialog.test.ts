import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { nextTick } from 'vue';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { DOMWrapper, flushPromises, mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import {
  backend,
  FileManagerMode,
  ProjectSort,
  Target,
  TerminalMode,
  ThemeMode,
} from '@/services/backend';
import type { Settings, TerminalPreset, ToolStatus } from '@/services/backend';
import { cloneSettings, createDefaultSettings, useAppStore } from '@/stores/app';
import { useToolsStore } from '@/stores/tools';
import { useNotificationsStore } from '@/stores/notifications';
import SettingsDialog from './SettingsDialog.vue';

vi.mock('@/services/backend', () => ({
  ThemeMode: { ThemeSystem: 'system', ThemeLight: 'light', ThemeDark: 'dark' },
  TerminalMode: { TerminalModeGhostty: 'ghostty', TerminalModeCustom: 'custom' },
  FileManagerMode: { FileManagerModeAuto: 'auto', FileManagerModeCustom: 'custom' },
  ProjectSort: { ProjectSortName: 'name', ProjectSortLastOpened: 'lastOpened' },
  Target: {
    TargetSublime: 'sublime',
    TargetZed: 'zed',
    TargetGhostty: 'ghostty',
    TargetTerminal: 'terminal',
    TargetFileManager: 'file_manager',
  },
  backend: {
    updateSettings: vi.fn(),
    listTools: vi.fn(),
    terminalPresets: vi.fn(),
    chooseExecutable: vi.fn(),
  },
}));

const kitty: TerminalPreset = {
  id: 'kitty',
  name: 'kitty',
  executable: 'kitty',
  args: ['--directory', '{path}'],
};
let pinia: ReturnType<typeof createPinia>;
let wrapper: VueWrapper | undefined;

function body() {
  return new DOMWrapper(document.body);
}

function control<
  T extends HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement = HTMLInputElement,
>(label: string) {
  const element = Array.from(document.body.querySelectorAll('label')).find(
    (item) => item.textContent?.trim() === label,
  )?.control;
  if (!element) throw new Error(`No labelled control: ${label}`);
  return new DOMWrapper(element as T);
}

function saveButton() {
  return body().get<HTMLButtonElement>('button[type="submit"]');
}

async function openDialog(open = true) {
  wrapper = mount(SettingsDialog, {
    props: { open },
    global: { plugins: [pinia] },
    attachTo: document.body,
  });
  await flushPromises();
  return wrapper;
}

async function submit() {
  await body().get('form').trigger('submit');
  await flushPromises();
}

function tool(target: Target, available: boolean, executable = '', reason = ''): ToolStatus {
  return { target, name: target, available, executable, reason };
}

beforeEach(() => {
  vi.resetAllMocks();
  pinia = createPinia();
  setActivePinia(pinia);
  const app = useAppStore();
  app.settings = createDefaultSettings();
  app.settingsLoaded = true;
  vi.mocked(backend.updateSettings).mockImplementation(async (settings) => cloneSettings(settings));
  vi.mocked(backend.listTools).mockImplementation(async () => [...useToolsStore().tools]);
  vi.mocked(backend.terminalPresets).mockResolvedValue([kitty]);
  vi.mocked(backend.chooseExecutable).mockResolvedValue('');
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  disposePinia(pinia);
  document.body.innerHTML = '';
});

describe('SettingsDialog', () => {
  it('saves only the changed theme and preserves sorting saved after opening', async () => {
    const app = useAppStore();
    await openDialog();
    app.settings.projectSort = ProjectSort.ProjectSortLastOpened;
    await control<HTMLSelectElement>('Theme').setValue(ThemeMode.ThemeDark);
    await submit();
    expect(backend.updateSettings).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        projectSort: ProjectSort.ProjectSortLastOpened,
        theme: ThemeMode.ThemeDark,
      }),
    );
  });

  it('closes unchanged preferences with a success notification without saving', async () => {
    const view = await openDialog();
    await submit();
    expect(backend.updateSettings).not.toHaveBeenCalled();
    expect(backend.listTools).toHaveBeenCalledTimes(1);
    expect(view.emitted('close')).toEqual([[]]);
    expect(useNotificationsStore().items).toEqual([
      expect.objectContaining({ tone: 'success', title: 'Preferences saved' }),
    ]);
  });

  it('changes alwaysOnTop while keeping the latest window size and rememberSize', async () => {
    const app = useAppStore();
    await openDialog();
    app.settings.window = { width: 700, height: 800, rememberSize: false, alwaysOnTop: false };
    await control('Always on top').setValue(true);
    await submit();
    expect(backend.updateSettings).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        window: { width: 700, height: 800, rememberSize: false, alwaysOnTop: true },
      }),
    );
  });

  it('merges changed executable and terminal args while keeping concurrent sub-field changes', async () => {
    const app = useAppStore();
    app.settings.preferredTerminal = TerminalMode.TerminalModeCustom;
    app.settings.customTerminal = { executable: 'kitty', args: ['--directory', '{path}'] };
    await openDialog();
    app.settings.executables.sublime = '/new/subl';
    app.settings.customTerminal.executable = 'wezterm';
    await control('Zed path').setValue(' /draft/zed ');
    await control<HTMLTextAreaElement>('Arguments (one per line)').setValue('--cwd\n{path}');
    await submit();
    expect(backend.updateSettings).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        executables: expect.objectContaining({ sublime: '/new/subl', zed: '/draft/zed' }),
        customTerminal: { executable: 'wezterm', args: ['--cwd', '{path}'] },
      }),
    );
  });
  it('renders only while open, with all sections and current values', async () => {
    const app = useAppStore();
    app.settings.theme = ThemeMode.ThemeDark;
    app.settings.executables.zed = '/opt/Zed App/zed';
    const view = await openDialog(false);
    expect(document.body.querySelector('[role="dialog"]')).toBeNull();
    expect(backend.listTools).not.toHaveBeenCalled();
    await view.setProps({ open: true });
    await flushPromises();

    expect(body().get('[role="dialog"]').text()).toContain('Preferences');
    expect(
      body()
        .findAll('h3')
        .map((heading) => heading.text()),
    ).toEqual([
      'Appearance',
      'Editors',
      'Terminal',
      'Applications',
      'File manager',
      'Git',
      'Window',
    ]);
    expect(control<HTMLSelectElement>('Theme').element.value).toBe('dark');
    expect(control<HTMLSelectElement>('Default editor').element.value).toBe('zed');
    expect(control('Zed path').element.value).toBe('/opt/Zed App/zed');
    expect(control('Ghostty').element.checked).toBe(true);
    expect(control('System default (gio / xdg-open)').element.checked).toBe(true);
    expect(control('Remember window size').element.checked).toBe(true);
    expect(control('Always on top').element.checked).toBe(false);
    expect(body().text()).toContain('Saved in ~/.config/projecthub');
    expect(backend.listTools).toHaveBeenCalledTimes(1);
    expect(backend.terminalPresets).toHaveBeenCalledTimes(1);
    expect(saveButton().element.form).toBe(body().get('form').element);
  });

  it('saves changed appearance, editor and behaviour, preserving every other field', async () => {
    const original = cloneSettings(useAppStore().settings);
    const view = await openDialog();
    await control<HTMLSelectElement>('Theme').setValue(ThemeMode.ThemeLight);
    await control<HTMLSelectElement>('Default editor').setValue(Target.TargetSublime);
    await control('Remember last used editor per project').setValue(false);
    await control('Double-click opens the default editor').setValue(false);
    await control('Show uncommitted-changes indicator').setValue(false);
    expect(useAppStore().settings).toEqual(original);
    await submit();

    expect(backend.updateSettings).toHaveBeenCalledExactlyOnceWith({
      ...original,
      theme: ThemeMode.ThemeLight,
      defaultEditor: Target.TargetSublime,
      rememberLastEditor: false,
      doubleClickOpensEditor: false,
      showGitDirty: false,
    });
    expect(view.emitted('close')).toHaveLength(1);
    expect(useNotificationsStore().items).toEqual([
      expect.objectContaining({ tone: 'success', title: 'Preferences saved' }),
    ]);
    expect(backend.listTools).toHaveBeenCalledTimes(2);
  });

  it('copies a preset and saves lines as discrete arguments with internal spaces preserved', async () => {
    await openDialog();
    await control('Custom terminal').setValue(true);
    await control<HTMLSelectElement>('Terminal preset').setValue('kitty');
    expect(control('Custom terminal executable').element.value).toBe('kitty');
    expect(control<HTMLTextAreaElement>('Arguments (one per line)').element.value).toBe(
      '--directory\n{path}',
    );
    await submit();
    const firstSave = vi.mocked(backend.updateSettings).mock.calls[0]![0];
    expect(firstSave.customTerminal.args).toEqual(['--directory', '{path}']);
    expect(firstSave.customTerminal.args).not.toBe(useToolsStore().presets[0]!.args);

    const args = control<HTMLTextAreaElement>('Arguments (one per line)');
    await args.setValue('a b\n\n{path}\n');
    expect(args.element.value).toBe('a b\n\n{path}\n');
    await submit();
    expect(vi.mocked(backend.updateSettings).mock.calls[1]![0].customTerminal).toEqual({
      executable: 'kitty',
      args: ['a b', '{path}'],
    });
    expect(useToolsStore().presets[0]!.args).toEqual(['--directory', '{path}']);
  });

  it.each([
    ['--title\n Build \n{path}', ['--title', ' Build ', '{path}']],
    ['--title\r\n Build \r\n{path}\r\n', ['--title', ' Build ', '{path}']],
    [' \n\n{path}', [' ', '{path}']],
  ])('preserves argument whitespace in %j', async (text, expected) => {
    useAppStore().settings.preferredTerminal = TerminalMode.TerminalModeCustom;
    useAppStore().settings.customTerminal.executable = 'kitty';
    await openDialog();
    await control<HTMLTextAreaElement>('Arguments (one per line)').setValue(text);
    await submit();
    expect(vi.mocked(backend.updateSettings).mock.calls[0]![0].customTerminal.args).toEqual(
      expected,
    );
  });

  it('trims executable fields on save while preserving arguments', async () => {
    useAppStore().settings.preferredTerminal = TerminalMode.TerminalModeCustom;
    useAppStore().settings.preferredFileManager = FileManagerMode.FileManagerModeCustom;
    await openDialog();
    for (const [label, executable] of [
      ['Sublime Text path', '/usr/bin/subl'],
      ['Zed path', '/usr/bin/zed'],
      ['Ghostty executable', '/usr/bin/ghostty'],
      ['File manager executable', '/usr/bin/nautilus'],
      ['Custom terminal executable', '/opt/Terminal App/kitty'],
    ]) {
      await control(label!).setValue(` ${executable} `);
    }
    await control<HTMLTextAreaElement>('Arguments (one per line)').setValue(' Build ');
    await submit();
    expect(backend.updateSettings).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        executables: {
          sublime: '/usr/bin/subl',
          zed: '/usr/bin/zed',
          ghostty: '/usr/bin/ghostty',
          fileManager: '/usr/bin/nautilus',
        },
        customTerminal: { executable: '/opt/Terminal App/kitty', args: [' Build '] },
      }),
    );
  });

  it('reinitialises the draft and baseline when settings finish loading without saving unchanged values', async () => {
    const app = useAppStore();
    app.settingsLoaded = false;
    await openDialog();
    expect(saveButton().element.disabled).toBe(true);
    await control('Custom terminal').setValue(true);
    await control<HTMLSelectElement>('Terminal preset').setValue('kitty');
    const loaded = cloneSettings({
      ...app.settings,
      theme: ThemeMode.ThemeDark,
      defaultEditor: Target.TargetSublime,
      executables: { ...app.settings.executables, zed: '/opt/zed/bin/zed' },
      preferredTerminal: TerminalMode.TerminalModeCustom,
      customTerminal: { executable: 'wezterm', args: ['start', '--cwd', '{path}'] },
    });
    app.settings = loaded;
    app.settingsLoaded = true;
    await nextTick();
    expect(control<HTMLSelectElement>('Theme').element.value).toBe('dark');
    expect(control<HTMLSelectElement>('Default editor').element.value).toBe('sublime');
    expect(control('Zed path').element.value).toBe('/opt/zed/bin/zed');
    expect(control('Custom terminal executable').element.value).toBe('wezterm');
    expect(control<HTMLTextAreaElement>('Arguments (one per line)').element.value).toBe(
      'start\n--cwd\n{path}',
    );
    expect(control<HTMLSelectElement>('Terminal preset').element.value).toBe('');
    expect(saveButton().element.disabled).toBe(false);
    await submit();
    expect(backend.updateSettings).not.toHaveBeenCalled();
    expect(app.settings).toEqual(loaded);
  });

  it('rejects an empty custom terminal executable and ignores hidden custom validation', async () => {
    await openDialog();
    expect(body().find('textarea').exists()).toBe(false);
    await control('Custom terminal').setValue(true);
    await control('Custom terminal executable').setValue('   ');
    expect(saveButton().element.disabled).toBe(true);
    expect(body().text()).toContain('Enter a custom terminal executable.');
    await submit();
    expect(backend.updateSettings).not.toHaveBeenCalled();
    await control('Ghostty').setValue(true);
    expect(saveButton().element.disabled).toBe(false);
  });

  it('rejects an empty custom file manager executable and saves a valid selection', async () => {
    await openDialog();
    await control('Custom').setValue(true);
    await control('File manager executable').setValue(' ');
    expect(saveButton().element.disabled).toBe(true);
    expect(body().text()).toContain('Enter a custom file manager executable.');
    await submit();
    expect(backend.updateSettings).not.toHaveBeenCalled();
    await control('File manager executable').setValue('/usr/bin/nautilus');
    expect(saveButton().element.disabled).toBe(false);
    await submit();
    expect(backend.updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        preferredFileManager: FileManagerMode.FileManagerModeCustom,
        executables: expect.objectContaining({ fileManager: '/usr/bin/nautilus' }),
      }),
    );
  });

  it.each([
    'Ghostty executable',
    'Custom terminal executable',
    'Sublime Text path',
    'Zed path',
    'File manager executable',
  ])('browses for %s and preserves it on picker cancellation', async (label) => {
    useAppStore().settings.preferredTerminal = TerminalMode.TerminalModeCustom;
    useAppStore().settings.preferredFileManager = FileManagerMode.FileManagerModeCustom;
    vi.mocked(backend.chooseExecutable)
      .mockResolvedValueOnce('/opt/Tool App/bin/tool')
      .mockResolvedValueOnce('');
    await openDialog();
    const browse = body().get(`[aria-label="Browse ${label}"]`);
    await browse.trigger('click');
    await flushPromises();
    expect(control(label).element.value).toBe('/opt/Tool App/bin/tool');
    await browse.trigger('click');
    await flushPromises();
    expect(control(label).element.value).toBe('/opt/Tool App/bin/tool');
    expect(backend.chooseExecutable).toHaveBeenCalledTimes(2);
  });

  it('notifies about picker errors', async () => {
    vi.mocked(backend.chooseExecutable).mockRejectedValue(new Error('Picker unavailable'));
    await openDialog();
    await body().get('[aria-label="Browse Zed path"]').trigger('click');
    await flushPromises();
    expect(useNotificationsStore().items).toEqual([
      expect.objectContaining({
        tone: 'error',
        title: 'Could not open file picker',
        detail: 'Picker unavailable',
      }),
    ]);
  });

  it('keeps the dialog open and shows the backend save error', async () => {
    vi.mocked(backend.updateSettings).mockRejectedValue(new Error('Settings are read-only'));
    const view = await openDialog();
    await control<HTMLSelectElement>('Theme').setValue(ThemeMode.ThemeDark);
    await submit();
    expect(view.emitted('close')).toBeUndefined();
    expect(body().get('[role="alert"]').text()).toBe('Settings are read-only');
    expect(useNotificationsStore().items).toEqual([]);
    expect(saveButton().element.disabled).toBe(false);
  });

  it('disables saving when settings could not be loaded, including form submission', async () => {
    useAppStore().settingsLoaded = false;
    await openDialog();
    expect(saveButton().element.disabled).toBe(true);
    expect(body().get('[role="alert"]').text()).toBe(
      'Preferences could not be loaded, so they cannot be saved. Fix or remove settings.json and restart.',
    );
    await submit();
    expect(backend.updateSettings).not.toHaveBeenCalled();
  });

  it('shows tool availability and replaces stale status after edits', async () => {
    useAppStore().settings.preferredTerminal = TerminalMode.TerminalModeCustom;
    useAppStore().settings.customTerminal.executable = 'kitty';
    useAppStore().settings.preferredFileManager = FileManagerMode.FileManagerModeCustom;
    useAppStore().settings.executables.fileManager = 'nautilus';
    useToolsStore().tools = [
      tool(Target.TargetSublime, true, '/usr/bin/subl'),
      tool(Target.TargetZed, false, '', 'Zed was not found'),
      tool(Target.TargetGhostty, true, '/usr/bin/ghostty'),
      tool(Target.TargetTerminal, true, '/usr/bin/kitty'),
      tool(Target.TargetFileManager, true, '/usr/bin/nautilus'),
    ];
    await openDialog();
    expect(body().text()).toContain('Found: /usr/bin/subl');
    expect(body().text()).toContain('Zed was not found');
    expect(body().text()).toContain('Found: /usr/bin/ghostty');
    expect(body().text()).toContain('Found: /usr/bin/kitty');
    expect(body().text()).toContain('Found: /usr/bin/nautilus');
    expect(body().get('p[title="/usr/bin/subl"]').text()).toBe('Found: /usr/bin/subl');
    expect(control<HTMLSelectElement>('Default editor').text()).toContain('Zed (not found)');
    await control('Sublime Text path').setValue('/opt/subl');
    expect(body().text()).not.toContain('Found: /usr/bin/subl');
    expect(body().text()).toContain('Save to re-check');
  });

  it('shows no status before tool information is available', async () => {
    vi.mocked(backend.listTools).mockReturnValue(new Promise(() => {}));
    await openDialog();
    expect(body().text()).not.toContain('Found:');
    expect(body().text()).not.toContain('Save to re-check');
    expect(control<HTMLSelectElement>('Default editor').text()).not.toContain('(not found)');
  });

  it('keeps a deep independent draft while open, and resets it on reopening', async () => {
    const app = useAppStore();
    app.settings.customTerminal = { executable: 'kitty', args: ['--directory', '{path}'] };
    app.settings.preferredTerminal = TerminalMode.TerminalModeCustom;
    const original = cloneSettings(app.settings);
    const view = await openDialog();
    await control('Always on top').setValue(true);
    await control('Zed path').setValue('/draft/zed');
    await control<HTMLTextAreaElement>('Arguments (one per line)').setValue('draft argument');
    expect(app.settings).toEqual(original);
    app.settings = {
      ...original,
      theme: ThemeMode.ThemeDark,
      executables: { ...original.executables, zed: '/saved/zed' },
      window: { ...original.window, width: 700 },
      customTerminal: { executable: 'new-kitty', args: ['saved argument'] },
    };
    await nextTick();
    expect(control<HTMLSelectElement>('Theme').element.value).toBe('system');
    expect(control('Zed path').element.value).toBe('/draft/zed');
    expect(control('Always on top').element.checked).toBe(true);
    expect(control('Custom terminal executable').element.value).toBe('kitty');
    expect(control<HTMLTextAreaElement>('Arguments (one per line)').element.value).toBe(
      'draft argument',
    );
    await view.setProps({ open: false });
    await view.setProps({ open: true });
    await flushPromises();
    expect(control('Zed path').element.value).toBe('/saved/zed');
    expect(control('Always on top').element.checked).toBe(false);
    expect(control<HTMLTextAreaElement>('Arguments (one per line)').element.value).toBe(
      'saved argument',
    );
  });

  it('prevents duplicate submissions and closing during a save', async () => {
    let finishSave!: (settings: Settings) => void;
    vi.mocked(backend.updateSettings).mockImplementation(
      () =>
        new Promise((resolve) => {
          finishSave = resolve;
        }),
    );
    const view = await openDialog();
    await control<HTMLSelectElement>('Theme').setValue(ThemeMode.ThemeDark);
    await body().get('form').trigger('submit');
    await flushPromises();
    expect(saveButton().element.disabled).toBe(true);
    expect(body().get<HTMLButtonElement>('[aria-label="Close"]').element.disabled).toBe(true);
    await body().get('form').trigger('submit');
    expect(backend.updateSettings).toHaveBeenCalledTimes(1);
    expect(view.emitted('close')).toBeUndefined();
    finishSave(cloneSettings(useAppStore().settings));
    await flushPromises();
    expect(view.emitted('close')).toHaveLength(1);
  });

  it('cancels without persisting local edits', async () => {
    const view = await openDialog();
    await control('Zed path').setValue('/draft/zed');
    const cancel = body()
      .findAll('button')
      .find((button) => button.text() === 'Cancel')!;
    await cancel.trigger('click');
    expect(view.emitted('close')).toHaveLength(1);
    expect(backend.updateSettings).not.toHaveBeenCalled();
    expect(useAppStore().settings.executables.zed).toBe('');
  });
});
