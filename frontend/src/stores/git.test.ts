import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { backend, DirectoryState, GitState } from '@/services/backend';
import type { Project, ProjectInspection } from '@/services/backend';
import { useProjectsStore } from './projects';
import { useGitStore, INSPECT_BATCH_SIZE, STALE_AFTER_MS } from './git';
import { useNotificationsStore } from './notifications';
vi.mock('@/services/backend', () => ({
  ThemeMode: { ThemeSystem: 'system', ThemeDark: 'dark', ThemeLight: 'light' },
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

function project(id: string, overrides: Partial<Project> = {}): Project {
  return {
    id,
    name: id,
    description: '',
    path: `/code/${id}`,
    favorite: false,
    defaultEditor: '',
    createdAt: '2026-11-01T00:00:00Z',
    updatedAt: '2026-11-01T00:00:00Z',
    ...overrides,
  };
}

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

function inspection(id: string, overrides: Partial<ProjectInspection> = {}): ProjectInspection {
  return {
    projectId: id,
    path: `/code/${id}`,
    directory: DirectoryState.DirectoryPresent,
    checkedAt: '2026-11-15',
    git: {
      state: GitState.StateRepo,
      message: '',
      repo: {
        branch: 'main',
        headSha: '',
        detached: false,
        unborn: false,
        isDirty: false,
        dirtyUncertain: false,
        upstream: '',
        ahead: 0,
        behind: 0,
      },
    },
    ...overrides,
  };
}
describe('git store', () => {
  it('batches 30 projects as 24 + 6 with visible projects first', async () => {
    const projects = useProjectsStore();
    projects.projects = Array.from({ length: 30 }, (_, i) =>
      project(`p${i}`, { favorite: i === 29 }),
    );
    projects.tab = 'favorites';
    vi.mocked(backend.inspectProjects).mockImplementation(async (ids) =>
      ids.map((id) => inspection(id)),
    );
    const store = useGitStore();
    const now = Date.now();
    await store.refreshAll();
    expect(INSPECT_BATCH_SIZE).toBe(24);
    expect(backend.inspectProjects).toHaveBeenCalledTimes(2);
    const batches = vi.mocked(backend.inspectProjects).mock.calls.map(([ids]) => ids);
    expect(batches.map((ids) => ids.length)).toEqual([24, 6]);
    expect(batches[0]?.[0]).toBe('p29');
    expect(batches.flat()).toEqual(['p29', ...Array.from({ length: 29 }, (_, i) => `p${i}`)]);
    expect(Object.keys(store.inspections)).toHaveLength(30);
    expect(store.get('p29')).toEqual(inspection('p29'));
    expect(store.lastRefreshAt).toBe(now);
    expect(store.refreshing).toBe(false);
  });
  it.each([
    DirectoryState.DirectoryPresent,
    DirectoryState.DirectoryMissing,
    DirectoryState.DirectoryInaccessible,
    DirectoryState.DirectoryNotDirectory,
  ])('isMissing reflects %s', async (directory) => {
    useProjectsStore().projects = [project('a')];
    vi.mocked(backend.inspectProjects).mockResolvedValue([inspection('a', { directory })]);
    const store = useGitStore();
    await store.refreshProjects(['a']);
    expect(store.isMissing('a')).toBe(directory !== DirectoryState.DirectoryPresent);
    expect(store.isMissing('unknown')).toBe(false);
  });
  it('discards inspections for removed or moved projects', async () => {
    const projects = useProjectsStore();
    projects.projects = [project('removed'), project('moved'), project('kept')];
    let resolve!: (results: ProjectInspection[]) => void;
    vi.mocked(backend.inspectProjects).mockImplementation(
      () =>
        new Promise((done) => {
          resolve = done;
        }),
    );
    const store = useGitStore();
    const pending = store.refreshAll();
    projects.projects = [project('moved', { path: '/new/path' }), project('kept')];
    resolve(['removed', 'moved', 'kept'].map((id) => inspection(id)));
    await pending;
    expect(store.inspections).toEqual({ kept: inspection('kept') });
  });
  it('coalesces overlapping refreshes into one trailing full pass', async () => {
    useProjectsStore().projects = [project('a')];
    let resolve!: (results: ProjectInspection[]) => void;
    vi.mocked(backend.inspectProjects)
      .mockImplementationOnce(
        () =>
          new Promise((done) => {
            resolve = done;
          }),
      )
      .mockResolvedValue([inspection('a')]);
    const store = useGitStore();
    const first = store.refreshAll();
    const second = store.refreshAll();
    const third = store.refreshAll();
    expect(store.refreshing).toBe(true);
    expect(backend.inspectProjects).toHaveBeenCalledTimes(1);
    resolve([inspection('a')]);
    await Promise.all([first, second, third]);
    expect(backend.inspectProjects).toHaveBeenCalledTimes(2);
    expect(vi.mocked(backend.inspectProjects).mock.calls).toEqual([[['a']], [['a']]]);
    expect(store.refreshing).toBe(false);
  });
  it('reports rejection without advancing lastRefreshAt and clears refreshing', async () => {
    useProjectsStore().projects = [project('a')];
    const store = useGitStore();
    store.lastRefreshAt = 123;
    vi.mocked(backend.inspectProjects).mockRejectedValue(new Error('git timeout'));
    await store.refreshAll();
    expect(store.lastRefreshAt).toBe(123);
    expect(store.refreshing).toBe(false);
    expect(useNotificationsStore().items[0]).toMatchObject({
      tone: 'error',
      title: 'Could not refresh project status',
      detail: 'git timeout',
    });
  });
  it('refreshes only once the injected time reaches the stale threshold', async () => {
    useProjectsStore().projects = [project('a')];
    vi.mocked(backend.inspectProjects).mockResolvedValue([inspection('a')]);
    const store = useGitStore();
    store.lastRefreshAt = 1000;
    await store.refreshIfStale(1000 + STALE_AFTER_MS - 1);
    expect(backend.inspectProjects).not.toHaveBeenCalled();
    await store.refreshIfStale(1000 + STALE_AFTER_MS);
    expect(backend.inspectProjects).toHaveBeenCalledTimes(1);
  });
  it('does nothing for empty ids', async () => {
    const store = useGitStore();
    await store.refreshProjects([]);
    expect(backend.inspectProjects).not.toHaveBeenCalled();
    expect(store.lastRefreshAt).toBe(0);
  });
  it('detects unavailable Git and forgets only the requested entry', async () => {
    useProjectsStore().projects = [project('a'), project('b')];
    const unavailable = inspection('a');
    unavailable.git.state = GitState.StateUnavailable;
    vi.mocked(backend.inspectProjects).mockResolvedValue([unavailable, inspection('b')]);
    const store = useGitStore();
    expect(store.gitUnavailable).toBe(false);
    await store.refreshAll();
    expect(store.gitUnavailable).toBe(true);
    store.forget('a');
    store.forget('unknown');
    expect(store.get('a')).toBeUndefined();
    expect(store.get('b')).toEqual(inspection('b'));
    expect(store.gitUnavailable).toBe(false);
  });
});
