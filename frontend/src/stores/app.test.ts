import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { backend, ThemeMode } from '@/services/backend';
import type { Settings } from '@/services/backend';
import { useAppStore } from './app';

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

const defaultAppInfo = {
  name: 'ProjectHub',
  version: '0.1.0',
  configDir: '/home/dev/.config/projecthub',
  os: 'linux',
  arch: 'amd64',
};

describe('useAppStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('initializes app info, settings, and projects', async () => {
    vi.mocked(backend.getAppInfo).mockResolvedValue(defaultAppInfo);
    vi.mocked(backend.getSettings).mockResolvedValue({
      schemaVersion: 1,
      defaultEditor: 'zed',
      preferredTerminal: 'ghostty',
      preferredFileManager: 'auto',
      theme: ThemeMode.ThemeDark,
      window: { width: 440, height: 680, rememberSize: true, alwaysOnTop: false },
    });
    vi.mocked(backend.listProjects).mockResolvedValue([
      {
        id: 'b',
        name: 'Beta',
        description: 'Second',
        path: '/code/beta',
        favorite: false,
        defaultEditor: 'vscode',
        createdAt: '2026-11-14T00:00:00Z',
        updatedAt: '2026-11-14T00:00:00Z',
      },
      {
        id: 'a',
        name: 'Alpha',
        description: 'Favorite project',
        path: '/code/alpha',
        favorite: true,
        defaultEditor: 'zed',
        createdAt: '2026-11-14T00:00:00Z',
        updatedAt: '2026-11-14T00:00:00Z',
      },
    ]);

    const store = useAppStore();
    await store.initialize();

    expect(store.error).toBeNull();
    expect(store.settingsLoaded).toBe(true);
    expect(store.appInfo?.configDir).toBe('/home/dev/.config/projecthub');
    expect(store.settings.defaultEditor).toBe('zed');
    expect(store.filteredProjects.map((p) => p.id)).toEqual(['a', 'b']);

    store.searchQuery = 'beta';
    expect(store.filteredProjects.map((p) => p.id)).toEqual(['b']);
  });

  it('preserves loaded settings when listProjects fails and keeps them on theme cycle', async () => {
    vi.mocked(backend.getAppInfo).mockResolvedValue(defaultAppInfo);
    vi.mocked(backend.getSettings).mockResolvedValue({
      schemaVersion: 1,
      defaultEditor: 'zed',
      preferredTerminal: 'kitty',
      preferredFileManager: 'auto',
      theme: ThemeMode.ThemeSystem,
      window: { width: 440, height: 680, rememberSize: true, alwaysOnTop: false },
    });
    vi.mocked(backend.listProjects).mockRejectedValue(new Error('corrupted projects.json'));
    vi.mocked(backend.updateSettings).mockImplementation(async (s) => s);

    const store = useAppStore();
    await store.initialize();

    expect(store.settingsLoaded).toBe(true);
    expect(store.settings.defaultEditor).toBe('zed');
    expect(store.settings.preferredTerminal).toBe('kitty');
    expect(store.error).toContain('corrupted projects.json');

    await store.cycleTheme();
    expect(backend.updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        defaultEditor: 'zed',
        preferredTerminal: 'kitty',
        theme: ThemeMode.ThemeDark,
      }),
    );
  });

  it('refuses to save settings or cycle theme when getSettings failed', async () => {
    vi.mocked(backend.getAppInfo).mockResolvedValue(defaultAppInfo);
    vi.mocked(backend.getSettings).mockRejectedValue(new Error('corrupted configuration file'));
    vi.mocked(backend.listProjects).mockResolvedValue([]);

    const store = useAppStore();
    await store.initialize();

    expect(store.settingsLoaded).toBe(false);
    expect(store.error).toContain('corrupted configuration file');

    await store.cycleTheme();
    const saved = await store.updateSettings(store.settings);

    expect(saved).toBe(false);
    expect(backend.updateSettings).not.toHaveBeenCalled();
  });

  it('cycles theme and persists via backend.updateSettings after successful load', async () => {
    vi.mocked(backend.getAppInfo).mockResolvedValue(defaultAppInfo);
    vi.mocked(backend.getSettings).mockResolvedValue({
      schemaVersion: 1,
      defaultEditor: 'vscode',
      preferredTerminal: 'ghostty',
      preferredFileManager: 'auto',
      theme: ThemeMode.ThemeSystem,
      window: { width: 440, height: 680, rememberSize: true, alwaysOnTop: false },
    });
    vi.mocked(backend.listProjects).mockResolvedValue([]);
    vi.mocked(backend.updateSettings).mockImplementation(async (s) => s);

    const store = useAppStore();
    await store.initialize();
    expect(store.settings.theme).toBe(ThemeMode.ThemeSystem);

    await store.cycleTheme();
    expect(store.settings.theme).toBe(ThemeMode.ThemeDark);

    await store.cycleTheme();
    expect(store.settings.theme).toBe(ThemeMode.ThemeLight);

    await store.cycleTheme();
    expect(store.settings.theme).toBe(ThemeMode.ThemeSystem);
    expect(backend.updateSettings).toHaveBeenCalledTimes(3);
  });

  it('serializes concurrent updateSettings and cycleTheme calls without dropping edits', async () => {
    vi.mocked(backend.getAppInfo).mockResolvedValue(defaultAppInfo);
    vi.mocked(backend.getSettings).mockResolvedValue({
      schemaVersion: 1,
      defaultEditor: 'vscode',
      preferredTerminal: 'ghostty',
      preferredFileManager: 'auto',
      theme: ThemeMode.ThemeSystem,
      window: { width: 440, height: 680, rememberSize: true, alwaysOnTop: false },
    });
    vi.mocked(backend.listProjects).mockResolvedValue([]);

    let resolveFirst!: (value: Settings) => void;
    const firstPending = new Promise<Settings>((resolve) => {
      resolveFirst = resolve;
    });

    vi.mocked(backend.updateSettings)
      .mockImplementationOnce(async () => firstPending)
      .mockImplementationOnce(async (s) => s);

    const store = useAppStore();
    await store.initialize();

    const savePromise = store.updateSettings({
      ...store.settings,
      defaultEditor: 'zed',
      window: { ...store.settings.window },
    });
    const cyclePromise = store.cycleTheme();
    await Promise.resolve();

    expect(store.saving).toBe(true);
    expect(backend.updateSettings).toHaveBeenCalledTimes(1);

    resolveFirst({
      ...store.settings,
      defaultEditor: 'zed',
      window: { ...store.settings.window },
    });

    const [saveResult] = await Promise.all([savePromise, cyclePromise]);
    expect(saveResult).toBe(true);
    expect(store.saving).toBe(false);
    expect(backend.updateSettings).toHaveBeenCalledTimes(2);
    expect(store.settings.defaultEditor).toBe('zed');
    expect(store.settings.theme).toBe(ThemeMode.ThemeDark);
  });
});
