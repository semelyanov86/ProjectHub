import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { backend, Target } from '@/services/backend';
import type { TerminalPreset, ToolStatus } from '@/services/backend';
import { useToolsStore } from './tools';
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

const tool: ToolStatus = {
  target: Target.TargetZed,
  name: 'Zed',
  available: true,
  executable: '/bin/zed',
  reason: '',
};
const preset: TerminalPreset = {
  id: 'kitty',
  name: 'Kitty',
  executable: 'kitty',
  args: ['--directory', '{path}'],
};
describe('tools store', () => {
  it('refreshes tools, loaded and timestamp and supports lookup', async () => {
    vi.mocked(backend.listTools).mockResolvedValue([tool]);
    const store = useToolsStore();
    expect(store.loaded).toBe(false);
    expect(store.refreshedAt).toBe(0);
    await store.refresh();
    expect(store.tools).toEqual([tool]);
    expect(store.loaded).toBe(true);
    expect(store.refreshedAt).toBe(Date.now());
    expect(store.error).toBeNull();
    expect(store.byTarget(Target.TargetZed)).toEqual(tool);
    expect(store.byTarget(Target.TargetSublime)).toBeUndefined();
  });
  it('preserves old tools on failure and clears error after recovery', async () => {
    const store = useToolsStore();
    vi.mocked(backend.listTools)
      .mockResolvedValueOnce([tool])
      .mockRejectedValueOnce(new Error('lookup failed'))
      .mockResolvedValueOnce([]);
    await store.refresh();
    await store.refresh();
    expect(store.tools).toEqual([tool]);
    expect(store.loaded).toBe(true);
    expect(store.error).toBe('lookup failed');
    await store.refresh();
    expect(store.tools).toEqual([]);
    expect(store.error).toBeNull();
  });
  it('loads terminal presets only once', async () => {
    vi.mocked(backend.terminalPresets).mockResolvedValue([preset]);
    const store = useToolsStore();
    await store.loadPresets();
    await store.loadPresets();
    expect(store.presets).toEqual([preset]);
    expect(backend.terminalPresets).toHaveBeenCalledTimes(1);
  });
  it('caches a successful empty preset list', async () => {
    vi.mocked(backend.terminalPresets).mockResolvedValue([]);
    const store = useToolsStore();
    await store.loadPresets();
    await store.loadPresets();
    expect(store.presets).toEqual([]);
    expect(backend.terminalPresets).toHaveBeenCalledTimes(1);
  });

  it('reports preset failures and allows retry', async () => {
    vi.mocked(backend.terminalPresets)
      .mockRejectedValueOnce(new Error('presets failed'))
      .mockResolvedValueOnce([preset]);
    const store = useToolsStore();
    await store.loadPresets();
    expect(store.error).toBe('presets failed');
    expect(store.presets).toEqual([]);
    await store.loadPresets();
    expect(store.presets).toEqual([preset]);
  });
});
