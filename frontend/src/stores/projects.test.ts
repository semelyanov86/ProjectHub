import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { flushPromises } from '@vue/test-utils';
import { backend, ProjectSort, Target } from '@/services/backend';
import type { OpenResult, Project, ProjectInput } from '@/services/backend';
import { useAppStore } from './app';
import { useProjectsStore } from './projects';
import { useNotificationsStore } from './notifications';
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

const input: ProjectInput = {
  name: 'New',
  description: '',
  path: '/code/new',
  favorite: false,
  defaultEditor: '',
};
describe('projects store', () => {
  it('loads projects and clears vanished selection', async () => {
    const store = useProjectsStore();
    store.projects = [project('gone')];
    store.select('gone');
    vi.mocked(backend.listProjects).mockResolvedValue([project('new')]);
    expect(await store.load()).toBe(true);
    expect(store.loaded).toBe(true);
    expect(store.loading).toBe(false);
    expect(store.loadError).toBeNull();
    expect(store.projects).toEqual([project('new')]);
    expect(store.selectedId).toBeNull();
  });
  it('ignores a stale refresh that resolves after a newer reload', async () => {
    const store = useProjectsStore();
    let resolveStale!: (projects: Project[]) => void;
    vi.mocked(backend.listProjects)
      .mockImplementationOnce(() => new Promise((resolve) => (resolveStale = resolve)))
      .mockResolvedValueOnce([project('a'), project('dropped')]);

    const stale = store.load();
    const fresh = store.load();
    await expect(fresh).resolves.toBe(true);
    expect(store.projects.map((p) => p.id)).toEqual(['a', 'dropped']);

    resolveStale([project('a')]);
    await expect(stale).resolves.toBe(true);
    expect(store.projects.map((p) => p.id)).toEqual(['a', 'dropped']);
    expect(store.loading).toBe(false);
  });

  it('does not report errors of a superseded load', async () => {
    const store = useProjectsStore();
    let rejectStale!: (err: Error) => void;
    vi.mocked(backend.listProjects)
      .mockImplementationOnce(() => new Promise((_, reject) => (rejectStale = reject)))
      .mockResolvedValueOnce([project('a')]);

    const stale = store.load();
    await store.load();
    rejectStale(new Error('old failure'));
    await expect(stale).resolves.toBe(true);
    expect(store.loadError).toBeNull();
    expect(store.projects.map((p) => p.id)).toEqual(['a']);
  });

  it('reports load failure and preserves catalog', async () => {
    const store = useProjectsStore();
    store.projects = [project('old')];
    vi.mocked(backend.listProjects).mockRejectedValue(new Error('corrupt catalog'));
    expect(await store.load()).toBe(false);
    expect(store.loadError).toBe('corrupt catalog');
    expect(store.loaded).toBe(false);
    expect(store.loading).toBe(false);
    expect(store.projects).toEqual([project('old')]);
  });

  it.each(['add', 'update', 'remove', 'setFavorite', 'open'] as const)(
    'retries a stale catalog load after a successful %s mutation',
    async (operation) => {
      const store = useProjectsStore();
      const original = project('a');
      store.projects = [original];
      let resolveList!: (projects: Project[]) => void;
      const changed = project('a', {
        ...(operation === 'update' ? { name: 'Changed' } : {}),
        ...(operation === 'setFavorite' ? { favorite: true } : {}),
        ...(operation === 'open' ? { lastOpened: '2026-11-15' } : {}),
      });
      const authoritative =
        operation === 'add' ? [original, project('new')] : operation === 'remove' ? [] : [changed];
      vi.mocked(backend.listProjects)
        .mockReturnValueOnce(
          new Promise((resolve) => {
            resolveList = resolve;
          }),
        )
        .mockResolvedValue(authoritative);
      const loading = store.load();
      if (operation === 'add') {
        vi.mocked(backend.addProject).mockResolvedValue(project('new'));
        await store.add(input);
      } else if (operation === 'update') {
        vi.mocked(backend.updateProject).mockResolvedValue(changed);
        await store.update('a', input);
      } else if (operation === 'remove') {
        vi.mocked(backend.removeProject).mockResolvedValue();
        await store.remove('a');
      } else if (operation === 'setFavorite') {
        vi.mocked(backend.setFavorite).mockResolvedValue(changed);
        await store.setFavorite('a', true);
      } else {
        vi.mocked(backend.openProjectDefault).mockResolvedValue({ project: changed, warning: '' });
        await store.open('a');
      }
      resolveList([original]);
      expect(await loading).toBe(true);
      expect(store.projects).toEqual(authoritative);
      expect(backend.listProjects).toHaveBeenCalledTimes(2);
      expect(store.loading).toBe(false);
      expect(store.loadError).toBeNull();
    },
  );
  it('derives visible projects from tab, search and settings, and counts the catalog', () => {
    const store = useProjectsStore();
    store.projects = [
      project('a', { favorite: true }),
      project('b', { lastOpened: '2026-11-01' }),
      project('c', { lastOpened: '2026-11-02' }),
    ];
    expect(store.counts).toEqual({ all: 3, favorites: 1, recent: 2 });
    expect(store.visible.map((p) => p.id)).toEqual(['a', 'b', 'c']);
    useAppStore().settings.projectSort = ProjectSort.ProjectSortLastOpened;
    expect(store.visible.map((p) => p.id)).toEqual(['a', 'c', 'b']);
    store.tab = 'favorites';
    expect(store.visible.map((p) => p.id)).toEqual(['a']);
    store.tab = 'recent';
    expect(store.visible.map((p) => p.id)).toEqual(['c', 'b']);
    store.searchQuery = 'b';
    expect(store.visible.map((p) => p.id)).toEqual(['b']);
  });
  it('adds and selects the returned project, upserting existing ids', async () => {
    const store = useProjectsStore();
    vi.mocked(backend.addProject).mockResolvedValue(project('new'));
    expect(await store.add(input)).toEqual(project('new'));
    expect(backend.addProject).toHaveBeenCalledWith(input);
    expect(store.selectedId).toBe('new');
    await store.add(input);
    expect(store.projects).toHaveLength(1);
  });
  it('updates a project in place', async () => {
    const store = useProjectsStore();
    store.projects = [project('a'), project('b')];
    const updated = project('a', { name: 'Changed' });
    vi.mocked(backend.updateProject).mockResolvedValue(updated);
    expect(await store.update('a', input)).toEqual(updated);
    expect(backend.updateProject).toHaveBeenCalledWith('a', input);
    expect(store.projects).toEqual([updated, project('b')]);
    expect(store.byId('a')).toEqual(updated);
  });
  it.each(['add', 'update'] as const)(
    'propagates %s validation errors without notifications',
    async (operation) => {
      const store = useProjectsStore();
      const error = new Error('invalid path');
      vi.mocked(backend.addProject).mockRejectedValue(error);
      vi.mocked(backend.updateProject).mockRejectedValue(error);
      await expect(operation === 'add' ? store.add(input) : store.update('a', input)).rejects.toBe(
        error,
      );
      expect(store.projects).toEqual([]);
      expect(useNotificationsStore().items).toEqual([]);
    },
  );
  it('removes projects, clears selection and reports success', async () => {
    const store = useProjectsStore();
    store.projects = [project('a'), project('b')];
    store.select('a');
    vi.mocked(backend.removeProject).mockResolvedValue();
    expect(await store.remove('a')).toBe(true);
    expect(backend.removeProject).toHaveBeenCalledWith('a');
    expect(store.projects).toEqual([project('b')]);
    expect(store.selectedId).toBeNull();
    expect(useNotificationsStore().items[0]).toMatchObject({
      tone: 'success',
      title: 'Project removed',
    });
  });
  it('preserves catalog and selection on remove failure', async () => {
    const store = useProjectsStore();
    store.projects = [project('a')];
    store.select('a');
    vi.mocked(backend.removeProject).mockRejectedValue(new Error('disk full'));
    expect(await store.remove('a')).toBe(false);
    expect(store.projects).toEqual([project('a')]);
    expect(store.selectedId).toBe('a');
    expect(useNotificationsStore().items[0]).toMatchObject({ tone: 'error', detail: 'disk full' });
  });
  it.each([true, false])('toggles favorite from %s and upserts the result', async (favorite) => {
    const store = useProjectsStore();
    store.projects = [project('a', { favorite })];
    vi.mocked(backend.setFavorite).mockResolvedValue(project('a', { favorite: !favorite }));
    expect(await store.toggleFavorite('a')).toBe(true);
    expect(backend.setFavorite).toHaveBeenCalledWith('a', !favorite);
    expect(store.byId('a')?.favorite).toBe(!favorite);
    expect(store.projects).toHaveLength(1);
  });
  it.each([undefined, Target.TargetZed])(
    'opens with target %s and upserts timestamps',
    async (target) => {
      const store = useProjectsStore();
      store.projects = [project('a')];
      const opened = project('a', { lastOpened: '2026-11-15' });
      vi.mocked(backend.openProjectDefault).mockResolvedValue({ project: opened, warning: '' });
      vi.mocked(backend.openProject).mockResolvedValue({ project: opened, warning: '' });
      expect(await store.open('a', target)).toBe(true);
      if (target) {
        expect(backend.openProject).toHaveBeenCalledWith('a', target);
        expect(backend.openProjectDefault).not.toHaveBeenCalled();
      } else {
        expect(backend.openProjectDefault).toHaveBeenCalledWith('a');
        expect(backend.openProject).not.toHaveBeenCalled();
      }
      expect(store.projects).toEqual([opened]);
    },
  );
  it('reports launch warnings', async () => {
    const store = useProjectsStore();
    vi.mocked(backend.listProjects).mockResolvedValue([project('a')]);
    await store.load();
    vi.mocked(backend.openProjectDefault).mockResolvedValue({
      project: project('a'),
      warning: 'Timestamp not saved',
    });
    expect(await store.open('a')).toBe(true);
    await flushPromises();
    expect(backend.listProjects).toHaveBeenCalledTimes(2);
    expect(useNotificationsStore().items[0]).toMatchObject({
      tone: 'warning',
      detail: 'Timestamp not saved',
    });
  });

  it.each([undefined, Target.TargetZed])(
    'ignores a launch result for a project removed while target %s was opening',
    async (target) => {
      const store = useProjectsStore();
      store.projects = [project('a'), project('b')];
      let resolveOpen!: (result: OpenResult) => void;
      const pending = new Promise<OpenResult>((resolve) => {
        resolveOpen = resolve;
      });
      vi.mocked(backend.openProject).mockReturnValue(pending);
      vi.mocked(backend.openProjectDefault).mockReturnValue(pending);
      const opening = store.open('a', target);
      vi.mocked(backend.removeProject).mockResolvedValue();
      await store.remove('a');
      resolveOpen({ project: project('a', { lastOpened: '2026-11-15' }), warning: '' });
      expect(await opening).toBe(true);
      expect(store.projects).toEqual([project('b')]);
    },
  );

  it.each([undefined, Target.TargetZed])(
    'ignores a launch result for a project moved while target %s was opening',
    async (target) => {
      const store = useProjectsStore();
      store.projects = [project('a'), project('b')];
      let resolveOpen!: (result: OpenResult) => void;
      const pending = new Promise<OpenResult>((resolve) => {
        resolveOpen = resolve;
      });
      vi.mocked(backend.openProject).mockReturnValue(pending);
      vi.mocked(backend.openProjectDefault).mockReturnValue(pending);
      const opening = store.open('a', target);
      const moved = project('a', { path: '/code/moved' });
      vi.mocked(backend.updateProject).mockResolvedValue(moved);
      await store.update('a', { ...input, path: moved.path });
      resolveOpen({ project: project('a', { lastOpened: '2026-11-15' }), warning: '' });
      expect(await opening).toBe(true);
      expect(store.projects).toEqual([moved, project('b')]);
    },
  );
  it('returns false and reports launch rejection using the project name', async () => {
    const store = useProjectsStore();
    store.projects = [project('a', { name: 'Alpha' })];
    vi.mocked(backend.openProjectDefault).mockRejectedValue(new Error('not installed'));
    expect(await store.open('a')).toBe(false);
    expect(store.projects).toEqual([project('a', { name: 'Alpha' })]);
    expect(useNotificationsStore().items[0]).toMatchObject({
      tone: 'error',
      title: 'Could not open Alpha',
      detail: 'not installed',
    });
  });
});
