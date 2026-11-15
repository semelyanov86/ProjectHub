import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { backend, DirectoryState, GitState, Target } from '@/services/backend';
import type {
  DirectoryDraft,
  DropResult,
  Project,
  ProjectInspection,
  ToolStatus,
} from '@/services/backend';
import { createDefaultSettings } from '@/stores/app';
import { DOUBLE_CLICK_DELAY_MS } from '@/lib/projects';
import App from './App.vue';
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
  GitState: {
    StateRepo: 'repo',
    StateNotRepo: 'not_repo',
    StateUnavailable: 'unavailable',
    StateError: 'error',
  },
  DirectoryState: {
    DirectoryPresent: 'present',
    DirectoryMissing: 'missing',
    DirectoryInaccessible: 'inaccessible',
    DirectoryNotDirectory: 'not_directory',
  },
  ImportMode: { ImportMerge: 'merge', ImportReplace: 'replace' },
  backend: {
    getAppInfo: vi.fn(),
    getSettings: vi.fn(),
    updateSettings: vi.fn(),
    listProjects: vi.fn(),
    chooseDirectory: vi.fn(),
    addProject: vi.fn(),
    updateProject: vi.fn(),
    removeProject: vi.fn(),
    setFavorite: vi.fn(),
    listTools: vi.fn(),
    terminalPresets: vi.fn(),
    openProject: vi.fn(),
    openProjectDefault: vi.fn(),
    inspectProjects: vi.fn(),
    prepareImport: vi.fn(),
    applyImport: vi.fn(),
    discardImport: vi.fn(),
    exportProjects: vi.fn(),
    chooseExecutable: vi.fn(),
    copyText: vi.fn(),
    onProjectsDropped: vi.fn(),
    onWindowFocus: vi.fn(),
  },
}));
function project(id = 'alpha', overrides: Partial<Project> = {}): Project {
  return {
    id,
    name: 'Alpha App',
    description: 'Desktop manager',
    path: `/home/dev/${id}`,
    defaultEditor: '',
    favorite: false,
    createdAt: '2026-11-15T10:00:00Z',
    updatedAt: '2026-11-15T10:00:00Z',
    ...overrides,
  };
}
function inspection(overrides: Partial<ProjectInspection> = {}): ProjectInspection {
  return {
    projectId: 'alpha',
    path: '/home/dev/alpha',
    directory: DirectoryState.DirectoryPresent,
    checkedAt: '2026-11-15T10:00:00Z',
    git: {
      state: GitState.StateRepo,
      message: '',
      repo: {
        branch: 'main',
        headSha: 'abcdef1234567890',
        detached: false,
        unborn: false,
        isDirty: true,
        dirtyUncertain: false,
        upstream: 'origin/main',
        ahead: 0,
        behind: 0,
      },
    },
    ...overrides,
  };
}
function toolsFixture(): ToolStatus[] {
  return [
    [Target.TargetSublime, 'Sublime Text'],
    [Target.TargetZed, 'Zed'],
    [Target.TargetGhostty, 'Ghostty'],
    [Target.TargetTerminal, 'Kitty'],
    [Target.TargetFileManager, 'File Manager'],
  ].map(([target, name]) => ({
    target: target as Target,
    name: name!,
    available: true,
    executable: `/usr/bin/${target}`,
    reason: '',
  }));
}
function menuItem(label: string): HTMLButtonElement {
  const item = Array.from(
    document.body.querySelectorAll<HTMLButtonElement>('[role="menuitem"]'),
  ).find((item) => item.textContent?.includes(label));
  expect(item, `menu item ${label}`).toBeDefined();
  return item!;
}

let pinia: ReturnType<typeof createPinia>;
let wrapper: VueWrapper | undefined;
let onDrop: (result: DropResult) => void;
let onFocus: () => void;
const unsubscribeDrop = vi.fn();
const unsubscribeFocus = vi.fn();
function draft(): DirectoryDraft {
  return {
    cancelled: false,
    path: '/home/dev/new',
    suggestedName: 'New Project',
    duplicateId: '',
    duplicateName: '',
    git: inspection().git,
  };
}
beforeEach(() => {
  vi.resetAllMocks();
  pinia = createPinia();
  setActivePinia(pinia);
  vi.mocked(backend.getAppInfo).mockResolvedValue({
    name: 'ProjectHub',
    version: '0.1.0',
    configDir: '/home/dev/.config/projecthub',
    homeDir: '/home/dev',
    os: 'linux',
    arch: 'amd64',
  });
  vi.mocked(backend.getSettings).mockResolvedValue(createDefaultSettings());
  vi.mocked(backend.updateSettings).mockImplementation(async (settings) => settings);
  vi.mocked(backend.listProjects).mockResolvedValue([]);
  vi.mocked(backend.listTools).mockResolvedValue(toolsFixture());
  vi.mocked(backend.terminalPresets).mockResolvedValue([]);
  vi.mocked(backend.inspectProjects).mockResolvedValue([]);
  vi.mocked(backend.chooseDirectory).mockResolvedValue(draft());
  vi.mocked(backend.discardImport).mockResolvedValue();
  vi.mocked(backend.onProjectsDropped).mockImplementation((callback) => {
    onDrop = callback;
    return unsubscribeDrop;
  });
  vi.mocked(backend.onWindowFocus).mockImplementation((callback) => {
    onFocus = callback;
    return unsubscribeFocus;
  });
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  disposePinia(pinia);
  document.body.innerHTML = '';
  vi.useRealTimers();
});
async function render() {
  const view = mount(App, { attachTo: document.body, global: { plugins: [pinia] } });
  wrapper = view;
  await flushPromises();
  return view;
}
const catalog = () => [
  project('beta', {
    name: 'Beta',
    description: 'A compiler',
    path: '/work/backend',
    lastOpened: '2026-11-14T12:00:00Z',
  }),
  project(),
  project('zulu', { name: 'Zulu', favorite: true }),
];
function rowIds() {
  return wrapper!.findAll('[role="option"]').map((row) => row.attributes('data-project-id'));
}
describe('App', () => {
  it('renders header, toolbar, tabs and the initial empty state', async () => {
    const view = await render();
    expect(view.get('h1').text()).toBe('ProjectHub');
    expect(view.find('input[aria-label="Search projects"]').exists()).toBe(true);
    expect(view.findAll('[role="tab"]')).toHaveLength(3);
    expect(view.text()).toContain('No projects yet');
  });
  it('loads projects sorted by name with favorites first', async () => {
    vi.mocked(backend.listProjects).mockResolvedValue(catalog());
    await render();
    expect(rowIds()).toEqual(['zulu', 'alpha', 'beta']);
  });
  it.each([
    ['Alpha', 'alpha'],
    ['compiler', 'beta'],
    ['/work/backend', 'beta'],
  ])('searches %s across name, description and path', async (query, id) => {
    vi.mocked(backend.listProjects).mockResolvedValue(catalog());
    const view = await render();
    await view.get('input').setValue(query);
    expect(rowIds()).toEqual([id]);
  });
  it('Favorites shows favorites and Recent shows only opened projects', async () => {
    vi.mocked(backend.listProjects).mockResolvedValue(catalog());
    const view = await render();
    await view.get('[data-tab="favorites"]').trigger('click');
    expect(rowIds()).toEqual(['zulu']);
    await view.get('[data-tab="recent"]').trigger('click');
    expect(rowIds()).toEqual(['beta']);
  });
  it('Ctrl+F focuses the search input', async () => {
    const view = await render();
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'f', ctrlKey: true, bubbles: true }));
    expect(document.activeElement).toBe(view.get('input').element);
  });
  it('Settings opens the teleported Preferences dialog', async () => {
    const view = await render();
    await view.get('[aria-label="Settings"]').trigger('click');
    await flushPromises();
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain('Preferences');
    expect(view.find('[role="dialog"]').exists()).toBe(false);
  });
  it('opening Settings cancels the delayed project menu and keeps focus in Preferences', async () => {
    vi.useFakeTimers();
    vi.mocked(backend.listProjects).mockResolvedValue([project()]);
    const view = await render();
    await view.get('[data-project-id="alpha"]').trigger('click');
    await view.get('[aria-label="Settings"]').trigger('click');
    await flushPromises();
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
    const dialogs = document.body.querySelectorAll('[role="dialog"]');
    expect(dialogs).toHaveLength(1);
    expect(dialogs[0]!.textContent).toContain('Preferences');
    expect(dialogs[0]!.contains(document.activeElement)).toBe(true);
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
  });
  it('Add chooses a directory then prefills the Add Project dialog', async () => {
    const view = await render();
    await view.get('[aria-label="Add project"]').trigger('click');
    await flushPromises();
    expect(backend.chooseDirectory).toHaveBeenCalledOnce();
    const dialog = document.body.querySelector('[role="dialog"]')!;
    expect(dialog.textContent).toContain('Add Project');
    expect(Array.from(dialog.querySelectorAll('input')).map((input) => input.value)).toEqual(
      expect.arrayContaining(['New Project', '/home/dev/new']),
    );
  });
  it('cancelled directory picker opens no dialog', async () => {
    vi.mocked(backend.chooseDirectory).mockResolvedValue({ ...draft(), cancelled: true });
    const view = await render();
    await view.get('[aria-label="Add project"]').trigger('click');
    await flushPromises();
    expect(document.body.querySelector('[role="dialog"]')).toBeNull();
  });
  it('a dropped draft opens Add Project', async () => {
    await render();
    onDrop({ draft: draft() });
    await flushPromises();
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain('Add Project');
    expect(backend.chooseDirectory).not.toHaveBeenCalled();
  });
  it('a dropped report reloads projects and displays Added N projects with null skipped entries', async () => {
    await render();
    vi.mocked(backend.listProjects).mockResolvedValue([
      project(),
      project('beta', { name: 'Beta' }),
    ]);
    onDrop({ report: { added: [project(), project('beta', { name: 'Beta' })], skipped: null } });
    await flushPromises();
    expect(backend.listProjects).toHaveBeenCalledTimes(2);
    expect(rowIds()).toEqual(['alpha', 'beta']);
    expect(document.body.textContent).toContain('Added 2 projects');
  });
  it('a drop error displays an error toast', async () => {
    await render();
    onDrop({ error: 'Permission denied' });
    await flushPromises();
    expect(document.body.textContent).toContain('Could not add dropped folder');
    expect(document.body.textContent).toContain('Permission denied');
  });
  it('window focus inspects only after Git status becomes stale', async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-11-15T10:00:00Z'));
    vi.mocked(backend.listProjects).mockResolvedValue([project()]);
    await render();
    expect(backend.inspectProjects).toHaveBeenCalledTimes(1);
    onFocus();
    await flushPromises();
    expect(backend.inspectProjects).toHaveBeenCalledTimes(1);
    vi.setSystemTime(new Date('2026-11-15T10:01:01Z'));
    onFocus();
    await flushPromises();
    expect(backend.inspectProjects).toHaveBeenCalledTimes(2);
    expect(backend.inspectProjects).toHaveBeenLastCalledWith(['alpha']);
  });
  it('Git inspections populate branch badges', async () => {
    vi.mocked(backend.listProjects).mockResolvedValue([project()]);
    vi.mocked(backend.inspectProjects).mockResolvedValue([inspection()]);
    const view = await render();
    expect(view.get('[data-testid="git-branch"]').text()).toBe('main');
  });
  it.each([
    ['Delete', 'Remove'],
    ['Delete', 'Cancel'],
    ['context menu', 'Remove'],
    ['context menu', 'Cancel'],
  ])('%s opens removal confirmation and %s handles the project', async (source, action) => {
    vi.mocked(backend.listProjects).mockResolvedValue([project()]);
    vi.mocked(backend.removeProject).mockResolvedValue();
    const view = await render();
    const row = view.get('[data-project-id="alpha"]');
    if (source === 'Delete') {
      await row.trigger('keydown', { key: 'Delete' });
    } else {
      await row.trigger('contextmenu');
      menuItem('Remove from ProjectHub…').click();
    }
    await flushPromises();
    const dialog = document.body.querySelector('[role="dialog"]')!;
    expect(dialog.textContent).toContain('Remove project?');
    expect(dialog.textContent).toContain('Alpha App');
    expect(dialog.textContent).toContain('/home/dev/alpha');
    expect(backend.removeProject).not.toHaveBeenCalled();
    expect(rowIds()).toEqual(['alpha']);
    Array.from(dialog.querySelectorAll('button'))
      .find((button) => button.textContent?.trim() === action)!
      .click();
    await flushPromises();
    expect(document.body.querySelector('[role="dialog"]')).toBeNull();
    if (action === 'Remove') {
      expect(backend.removeProject).toHaveBeenCalledExactlyOnceWith('alpha');
      expect(rowIds()).toEqual([]);
    } else {
      expect(backend.removeProject).not.toHaveBeenCalled();
      expect(rowIds()).toEqual(['alpha']);
    }
  });
  it('unmount unsubscribes drop and focus listeners', async () => {
    const view = await render();
    view.unmount();
    wrapper = undefined;
    expect(unsubscribeDrop).toHaveBeenCalledOnce();
    expect(unsubscribeFocus).toHaveBeenCalledOnce();
  });
  it('export menu exports projects and shows success', async () => {
    vi.mocked(backend.exportProjects).mockResolvedValue({
      cancelled: false,
      count: 3,
      path: '/tmp/projects.json',
    });
    const view = await render();
    await view.get('[aria-label="Import or export"]').trigger('click');
    menuItem('Export projects').click();
    await flushPromises();
    expect(backend.exportProjects).toHaveBeenCalledOnce();
    expect(document.body.textContent).toContain('Exported 3 projects');
  });
  it('import previews the file and opens the Import dialog with nullable skipped entries', async () => {
    vi.mocked(backend.prepareImport).mockResolvedValue({
      cancelled: false,
      token: 'preview-token',
      fileName: 'projects.json',
      total: 3,
      eligible: 3,
      new: 2,
      duplicates: 1,
      current: 1,
      skipped: null,
    });
    const view = await render();
    await view.get('[aria-label="Import or export"]').trigger('click');
    menuItem('Import projects').click();
    await flushPromises();
    expect(backend.prepareImport).toHaveBeenCalledOnce();
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain('Import');
    expect(document.body.querySelector('[role="dialog"]')?.textContent).toContain(
      '3 projects in file',
    );
  });
});
