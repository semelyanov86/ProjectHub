import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { DirectoryState, GitState } from '@/services/backend';
import type { Project, ProjectInspection } from '@/services/backend';
import ProjectRow from './ProjectRow.vue';
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

let wrapper: VueWrapper | undefined;
function render(overrides: Partial<InstanceType<typeof ProjectRow>['$props']> = {}) {
  wrapper = mount(ProjectRow, {
    attachTo: document.body,
    props: {
      project: project(),
      selected: false,
      focusable: false,
      missing: false,
      showGitDirty: true,
      homeDir: '/home/dev',
      inspection: inspection(),
      ...overrides,
    },
  });
  return wrapper;
}
afterEach(() => {
  wrapper?.unmount();
  document.body.innerHTML = '';
});
describe('ProjectRow', () => {
  it('uses focusable for its tab stop independently of selection', async () => {
    const view = render({ focusable: true, selected: false });
    expect(view.attributes('tabindex')).toBe('0');
    expect(view.attributes('aria-selected')).toBe('false');
    await view.setProps({ focusable: false, selected: true });
    expect(view.attributes('tabindex')).toBe('-1');
    expect(view.attributes('aria-selected')).toBe('true');
  });
  it('renders initials, name, description, abbreviated path with full title, star and branch', () => {
    const view = render({ project: project('alpha', { favorite: true }) });
    expect(view.text()).toContain('AA');
    expect(view.text()).toContain('Alpha App');
    expect(view.text()).toContain('Desktop manager');
    expect(view.get('[title="/home/dev/alpha"]').text()).toBe('~/alpha');
    expect(view.find('[aria-label="Favorite"]').exists()).toBe(true);
    expect(view.get('[data-testid="git-branch"]').text()).toBe('main');
    expect(view.find('[data-testid="git-error"]').exists()).toBe(false);
  });
  it('does not abbreviate a directory that only shares the home prefix', () => {
    const view = render({ project: project('alpha', { path: '/home/developer/app' }) });
    expect(view.get('[title="/home/developer/app"]').text()).toBe('/home/developer/app');
  });
  it.each([
    [true, true, true],
    [false, true, false],
    [true, false, false],
  ])('dirty setting %s, dirty repo %s renders dot %s', (showGitDirty, isDirty, expected) => {
    const state = inspection();
    state.git.repo.isDirty = isDirty;
    expect(
      render({ showGitDirty, inspection: state }).find('[data-testid="git-dirty"]').exists(),
    ).toBe(expected);
  });
  it('renders detached HEAD as a short SHA', () => {
    const state = inspection();
    state.git.repo.detached = true;
    expect(render({ inspection: state }).get('[data-testid="git-branch"]').text()).toBe('abcdef1');
  });
  it.each(['git status timed out after 3s', ''])('shows a Git error with message %j', (message) => {
    const state = inspection();
    state.git.state = GitState.StateError;
    state.git.message = message;
    const view = render({ inspection: state });
    const badge = view.get('[data-testid="git-error"]');
    const detail = message || 'Git status could not be read';
    expect(badge.text()).toBe('Git error');
    expect(badge.attributes('title')).toBe(detail);
    expect(badge.attributes('aria-label')).toBe(`Git status error: ${detail}`);
    expect(badge.classes()).toContain('bg-[var(--warning-soft)]');
    expect(badge.classes()).toContain('text-[var(--warning)]');
    expect(view.find('[data-testid="git-branch"]').exists()).toBe(false);
  });
  it.each([GitState.StateNotRepo, GitState.StateUnavailable])(
    'does not render a Git badge for %s',
    (gitState) => {
      const state = inspection();
      state.git.state = gitState;
      const view = render({ inspection: state });
      expect(view.find('[data-testid="git-error"]').exists()).toBe(false);
      expect(view.find('[data-testid="git-branch"]').exists()).toBe(false);
    },
  );
  it('shows uncertain changes as a hollow ring and explains the uncertainty in both tooltips', () => {
    const state = inspection();
    state.git.repo.dirtyUncertain = true;
    const view = render({ inspection: state });
    const ring = view.get('[data-testid="git-dirty-uncertain"]');
    const message = 'Changes may be inaccurate: repository filters were disabled for safety';
    expect(ring.classes()).toEqual(
      expect.arrayContaining(['border', 'border-[var(--warning)]', 'bg-transparent']),
    );
    expect(ring.attributes('title')).toBe(message);
    expect(ring.attributes('aria-label')).toBe(message);
    expect(view.find('[data-testid="git-dirty"]').exists()).toBe(false);
    expect(view.get('[data-testid="git-branch"]').attributes('title')).toContain(
      'uncommitted changes (may be inaccurate: repository filters were disabled for safety)',
    );
  });
  it('keeps the filled dot and normal tooltip for certain changes', () => {
    const view = render();
    expect(view.get('[data-testid="git-dirty"]').classes()).toContain('bg-[var(--warning)]');
    expect(view.find('[data-testid="git-dirty-uncertain"]').exists()).toBe(false);
    expect(view.get('[data-testid="git-branch"]').attributes('title')).toBe(
      'Branch main · uncommitted changes',
    );
  });
  it.each([
    [false, true],
    [true, false],
  ])('hides the uncertain ring with showGitDirty=%s and isDirty=%s', (showGitDirty, isDirty) => {
    const state = inspection();
    state.git.repo.isDirty = isDirty;
    state.git.repo.dirtyUncertain = true;
    const view = render({ showGitDirty, inspection: state });
    expect(view.find('[data-testid="git-dirty-uncertain"]').exists()).toBe(false);
    expect(view.find('[data-testid="git-dirty"]').exists()).toBe(false);
  });
  it('renders Missing for a missing directory', () => {
    expect(render({ missing: true }).text()).toContain('Missing');
  });
  it('favorite emits toggleFavorite without activating the row', async () => {
    const view = render();
    await view.get('[aria-label="Add to favorites"]').trigger('click');
    expect(view.emitted('toggleFavorite')).toEqual([[]]);
    expect(view.emitted('activate')).toBeUndefined();
  });
  it('more actions emits menuButton without activating the row', async () => {
    const view = render();
    await view.get('[aria-label="More actions"]').trigger('click');
    expect(view.emitted('menuButton')).toHaveLength(1);
    expect(view.emitted('activate')).toBeUndefined();
  });
  it('emits activation, default launch and context menu events', async () => {
    const view = render();
    await view.trigger('click');
    await view.trigger('dblclick');
    await view.trigger('contextmenu');
    expect(view.emitted('activate')).toHaveLength(1);
    expect(view.emitted('openDefault')).toEqual([[]]);
    expect(view.emitted('contextMenu')).toHaveLength(1);
  });
});
