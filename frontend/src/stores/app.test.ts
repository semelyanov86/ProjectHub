import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { backend, ProjectSort, ThemeMode } from '@/services/backend';
import type { Settings } from '@/services/backend';
import { cloneSettings, createDefaultSettings, mergeSettings, useAppStore } from './app';
import type { SettingsPatch } from './app';
vi.mock('@/services/backend', () => ({
  ThemeMode: { ThemeSystem: 'system', ThemeDark: 'dark', ThemeLight: 'light' },
  TerminalMode: { TerminalModeGhostty: 'ghostty', TerminalModeCustom: 'custom' },
  CloseBehavior: { CloseBehaviorTray: 'tray', CloseBehaviorQuit: 'quit' },
  FileManagerMode: { FileManagerModeAuto: 'auto', FileManagerModeCustom: 'custom' },
  ProjectSort: { ProjectSortName: 'name', ProjectSortLastOpened: 'lastOpened' },
  Target: {
    TargetSublime: 'sublime',
    TargetZed: 'zed',
    TargetGhostty: 'ghostty',
    TargetTerminal: 'terminal',
    TargetFileManager: 'file_manager',
  },
  DirectoryState: {
    DirectoryPresent: 'present',
    DirectoryMissing: 'missing',
    DirectoryInaccessible: 'inaccessible',
    DirectoryNotDirectory: 'not_directory',
  },
  GitState: {
    StateRepo: 'repo',
    StateNotRepo: 'not_repo',
    StateUnavailable: 'unavailable',
    StateError: 'error',
  },
  backend: {
    getAppInfo: vi.fn(),
    getSettings: vi.fn(),
    updateSettings: vi.fn(),
    listProjects: vi.fn(),
    addProject: vi.fn(),
    updateProject: vi.fn(),
    removeProject: vi.fn(),
    setFavorite: vi.fn(),
    openProject: vi.fn(),
    openProjectDefault: vi.fn(),
    inspectProjects: vi.fn(),
    listTools: vi.fn(),
    terminalPresets: vi.fn(),
  },
}));

let pinia: ReturnType<typeof createPinia>;
beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers();
  pinia = createPinia();
  setActivePinia(pinia);
});
afterEach(() => {
  disposePinia(pinia);
  vi.useRealTimers();
});

async function initialized() {
  vi.mocked(backend.getAppInfo).mockResolvedValue({
    name: 'ProjectHub',
    version: '0.1.0',
    configDir: '/config',
    homeDir: '/home/dev',
    os: 'linux',
    arch: 'amd64',
  });
  vi.mocked(backend.getSettings).mockResolvedValue(createDefaultSettings());
  vi.mocked(backend.updateSettings).mockImplementation(async (s) => s);
  const store = useAppStore();
  await store.initialize();
  return store;
}

describe('app store', () => {
  it('merges nested settings and copies arguments without mutating either input', () => {
    const base = createDefaultSettings();
    base.customTerminal = { executable: 'kitty', args: ['--directory', '{path}'] };
    const before = cloneSettings(base);
    const patch: SettingsPatch = {
      theme: ThemeMode.ThemeDark,
      window: { alwaysOnTop: true },
      executables: { zed: '/opt/zed' },
      customTerminal: { args: ['--cwd', '{path}'] },
    };
    const merged = mergeSettings(base, patch);
    expect(merged).toEqual({
      ...before,
      theme: ThemeMode.ThemeDark,
      window: { ...before.window, alwaysOnTop: true },
      executables: { ...before.executables, zed: '/opt/zed' },
      customTerminal: { executable: 'kitty', args: ['--cwd', '{path}'] },
    });
    expect(base).toEqual(before);
    merged.window.width = 999;
    merged.executables.zed = 'changed';
    merged.customTerminal.args?.push('extra');
    expect(base).toEqual(before);
    expect(patch).toEqual({
      theme: ThemeMode.ThemeDark,
      window: { alwaysOnTop: true },
      executables: { zed: '/opt/zed' },
      customTerminal: { args: ['--cwd', '{path}'] },
    });
    const executableOnly = mergeSettings(base, { customTerminal: { executable: 'wezterm' } });
    expect(executableOnly.customTerminal).toEqual({
      executable: 'wezterm',
      args: ['--directory', '{path}'],
    });
    expect(executableOnly.customTerminal.args).not.toBe(base.customTerminal.args);
  });
  it('applies a theme patch after a pending sort save without reverting the sort', async () => {
    const store = await initialized();
    let resolveFirst!: (settings: Settings) => void;
    vi.mocked(backend.updateSettings).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFirst = resolve;
        }),
    );
    const sort = store.setProjectSort(ProjectSort.ProjectSortLastOpened);
    await Promise.resolve();
    const theme = store.patchSettings({ theme: ThemeMode.ThemeDark });
    expect(backend.updateSettings).toHaveBeenCalledTimes(1);
    resolveFirst(vi.mocked(backend.updateSettings).mock.calls[0]![0]);
    expect(await Promise.all([sort, theme])).toEqual([true, true]);
    expect(backend.updateSettings).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        projectSort: ProjectSort.ProjectSortLastOpened,
        theme: ThemeMode.ThemeDark,
      }),
    );
  });
  it('loads info and settings, cloning all nested settings', async () => {
    const store = await initialized();
    const loaded = vi.mocked(backend.getSettings).mock.results[0];
    const source = (await loaded!.value) as Settings;
    expect(store.settingsLoaded).toBe(true);
    expect(store.appInfo?.name).toBe('ProjectHub');
    expect(store.settings).toEqual(source);
    expect(store.settings.window).not.toBe(source.window);
    expect(store.settings.executables).not.toBe(source.executables);
    expect(store.settings.customTerminal.args).not.toBe(source.customTerminal.args);
    source.window.width = 999;
    expect(store.settings.window.width).toBe(440);
    expect(store.error).toBeNull();
  });
  it('refuses writes after settings fail to load', async () => {
    vi.mocked(backend.getAppInfo).mockResolvedValue({
      name: 'ProjectHub',
      version: '',
      configDir: '',
      homeDir: '/home/dev',
      os: 'linux',
      arch: 'amd64',
    });
    vi.mocked(backend.getSettings).mockRejectedValue(new Error('corrupted settings'));
    const store = useAppStore();
    await store.initialize();
    expect(store.settingsLoaded).toBe(false);
    expect(store.error).toContain('corrupted settings');
    expect(await store.updateSettings(store.settings)).toBe(false);
    await store.cycleTheme();
    expect(await store.setProjectSort(ProjectSort.ProjectSortLastOpened)).toBe(false);
    expect(backend.updateSettings).not.toHaveBeenCalled();
  });
  it('cycles system → dark → light → system with persistence', async () => {
    const store = await initialized();
    for (const theme of [ThemeMode.ThemeDark, ThemeMode.ThemeLight, ThemeMode.ThemeSystem]) {
      await store.cycleTheme();
      expect(store.settings.theme).toBe(theme);
      expect(backend.updateSettings).toHaveBeenLastCalledWith(expect.objectContaining({ theme }));
    }
    expect(backend.updateSettings).toHaveBeenCalledTimes(3);
  });
  it('serializes concurrent updateSettings and cycleTheme and snapshots the draft', async () => {
    const store = await initialized();
    let resolveFirst!: (settings: Settings) => void;
    vi.mocked(backend.updateSettings).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFirst = resolve;
        }),
    );
    const draft = cloneSettings(store.settings);
    draft.defaultEditor = 'sublime';
    const save = store.updateSettings(draft);
    const cycle = store.cycleTheme();
    draft.window.width = 999;
    await Promise.resolve();
    expect(store.saving).toBe(true);
    expect(backend.updateSettings).toHaveBeenCalledTimes(1);
    const submitted = vi.mocked(backend.updateSettings).mock.calls[0]![0];
    expect(submitted.window.width).toBe(440);
    resolveFirst(submitted);
    expect(await save).toBe(true);
    await cycle;
    expect(backend.updateSettings).toHaveBeenCalledTimes(2);
    expect(store.settings.defaultEditor).toBe('sublime');
    expect(store.settings.theme).toBe(ThemeMode.ThemeDark);
    expect(store.saving).toBe(false);
  });
  it('applies queued patches to the latest saved settings and persists sorting', async () => {
    const store = await initialized();
    let resolveFirst!: (settings: Settings) => void;
    vi.mocked(backend.updateSettings).mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          resolveFirst = resolve;
        }),
    );
    const first = store.updateSettings({
      ...cloneSettings(store.settings),
      defaultEditor: 'sublime',
    });
    const patch = store.patchSettings({ showGitDirty: false });
    const sort = store.setProjectSort(ProjectSort.ProjectSortLastOpened);
    await Promise.resolve();
    expect(backend.updateSettings).toHaveBeenCalledTimes(1);
    resolveFirst(vi.mocked(backend.updateSettings).mock.calls[0]![0]);
    expect(await Promise.all([first, patch, sort])).toEqual([true, true, true]);
    expect(backend.updateSettings).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ defaultEditor: 'sublime', showGitDirty: false }),
    );
    expect(backend.updateSettings).toHaveBeenLastCalledWith(
      expect.objectContaining({
        defaultEditor: 'sublime',
        showGitDirty: false,
        projectSort: ProjectSort.ProjectSortLastOpened,
      }),
    );
  });
  it('returns false and reports failed save without poisoning the queue', async () => {
    const store = await initialized();
    const before = cloneSettings(store.settings);
    vi.mocked(backend.updateSettings).mockRejectedValueOnce(new Error('disk full'));
    expect(await store.patchSettings({ showGitDirty: false })).toBe(false);
    expect(store.error).toBe('disk full');
    expect(store.settings).toEqual(before);
    expect(store.saving).toBe(false);
    expect(await store.patchSettings({ showGitDirty: false })).toBe(true);
    expect(store.settings.showGitDirty).toBe(false);
    expect(store.error).toBeNull();
  });
  it('cloneSettings deep copies window, executables, terminal and args and normalizes null args', () => {
    const source = createDefaultSettings();
    source.customTerminal.args = ['--directory', '{path}'];
    const copy = cloneSettings(source);
    copy.window.width = 999;
    copy.executables.zed = 'changed';
    copy.customTerminal.executable = 'changed';
    copy.customTerminal.args?.push('extra');
    expect(source.window.width).toBe(440);
    expect(source.executables.zed).toBe('');
    expect(source.customTerminal).toEqual({ executable: '', args: ['--directory', '{path}'] });
    expect(
      cloneSettings({ ...source, customTerminal: { executable: '', args: null } }).customTerminal
        .args,
    ).toEqual([]);
  });
});
