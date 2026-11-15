import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushPromises, mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { nextTick } from 'vue';
import { backend, DirectoryState, GitState, Target, TerminalMode } from '@/services/backend';
import type { Project, ProjectInspection, ToolStatus } from '@/services/backend';
import { createDefaultSettings, useAppStore } from '@/stores/app';
import { useProjectsStore } from '@/stores/projects';
import { useGitStore } from '@/stores/git';
import { useToolsStore } from '@/stores/tools';
import { useNotificationsStore } from '@/stores/notifications';
import { DOUBLE_CLICK_DELAY_MS } from '@/lib/projects';
import ProjectsView from './ProjectsView.vue';
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
beforeEach(() => {
  vi.resetAllMocks();
  vi.useFakeTimers();
  pinia = createPinia();
  setActivePinia(pinia);
  useAppStore().settings = createDefaultSettings();
  useProjectsStore().projects = [
    project(),
    project('beta', { name: 'Beta' }),
    project('gamma', { name: 'Gamma' }),
  ];
  useProjectsStore().loaded = true;
  useToolsStore().tools = toolsFixture();
  useToolsStore().loaded = true;
  vi.mocked(backend.openProject).mockImplementation(async () => ({
    project: project(),
    warning: '',
  }));
  vi.mocked(backend.openProjectDefault).mockImplementation(async () => ({
    project: project(),
    warning: '',
  }));
  vi.mocked(backend.copyText).mockResolvedValue();
  vi.mocked(backend.listProjects).mockResolvedValue([]);
});
afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  disposePinia(pinia);
  document.body.innerHTML = '';
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
});
function render() {
  const view = mount(ProjectsView, { attachTo: document.body, global: { plugins: [pinia] } });
  wrapper = view;
  return view;
}
function row() {
  return wrapper!.get('[data-project-id="alpha"]');
}
async function openWith() {
  await row().trigger('click');
  await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
}
describe('ProjectsView', () => {
  it('places the delayed click menu below the row and launches on a slow double-click', async () => {
    vi.stubGlobal('innerHeight', 650);
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (
      this: HTMLElement,
    ) {
      return this.matches('[data-project-id]')
        ? new DOMRect(10, 100, 400, 56)
        : new DOMRect(0, 0, 220, 160);
    });
    render();
    const label = row().get('span.truncate');
    await label.trigger('pointerdown', { clientX: 40, clientY: 120 });
    await label.trigger('click', { detail: 1, clientX: 40, clientY: 120 });
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
    const menu = document.body.querySelector<HTMLElement>('[role="menu"]');
    expect(menu).not.toBeNull();
    expect(Number.parseFloat(menu!.style.top)).toBeGreaterThanOrEqual(156);
    expect(menu!.style.top).toBe('158px');
    await vi.advanceTimersByTimeAsync(100);
    await label.trigger('pointerdown', { clientX: 43, clientY: 123 });
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
    await label.trigger('click', { detail: 2, clientX: 43, clientY: 123 });
    await label.trigger('dblclick', { detail: 2, clientX: 43, clientY: 123 });
    await flushPromises();
    expect(backend.openProjectDefault).toHaveBeenCalledExactlyOnceWith('alpha');
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
  });

  it.each(['Enter', ' ', 'F10'])('anchors the %s keyboard menu below its row', async (key) => {
    vi.stubGlobal('innerHeight', 650);
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (
      this: HTMLElement,
    ) {
      return this.matches('[data-project-id]')
        ? new DOMRect(10, 100, 400, 56)
        : new DOMRect(0, 0, 220, 160);
    });
    render();
    await row().trigger('keydown', { key, shiftKey: key === 'F10' });
    await nextTick();
    expect(document.body.querySelector<HTMLElement>('[role="menu"]')?.style.top).toBe('158px');
  });

  it('places a clicked-row menu entirely above its row when it cannot fit below', async () => {
    vi.stubGlobal('innerHeight', 650);
    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function (
      this: HTMLElement,
    ) {
      return this.matches('[data-project-id]')
        ? new DOMRect(10, 550, 400, 56)
        : new DOMRect(0, 0, 220, 160);
    });
    render();
    await row().trigger('click', { detail: 1, clientX: 40, clientY: 570 });
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
    expect(document.body.querySelector<HTMLElement>('[role="menu"]')?.style.top).toBe('388px');
  });
  it('delays a single-click menu by DOUBLE_CLICK_DELAY_MS and lists all launch targets', async () => {
    render();
    await row().trigger('click');
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS - 1);
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
    await vi.advanceTimersByTimeAsync(1);
    for (const name of ['Sublime Text', 'Zed', 'Ghostty', 'File Manager'])
      expect(menuItem(name)).toBeDefined();
  });
  it('opens the menu immediately when double-click launch is off', async () => {
    useAppStore().settings.doubleClickOpensEditor = false;
    render();
    await row().trigger('click');
    expect(document.body.querySelector('[aria-label="Open with"]')).not.toBeNull();
  });
  it('cancels a pending menu when suspended and does not reopen it after resuming', async () => {
    const view = render();
    await row().trigger('click');
    await view.setProps({ suspended: true });
    await view.setProps({ suspended: false });
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
  });
  it('closes an open menu when suspended', async () => {
    const view = render();
    await openWith();
    expect(document.body.querySelector('[role="menu"]')).not.toBeNull();
    await view.setProps({ suspended: true });
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
  });
  it.each(['click', 'contextmenu', 'menu-button', 'Enter', ' ', 'ContextMenu', 'F10'])(
    'does not open a menu from %s while suspended',
    async (source) => {
      const view = render();
      await view.setProps({ suspended: true });
      if (source === 'menu-button') {
        await row().get('[aria-label="More actions"]').trigger('click');
      } else if (source === 'click' || source === 'contextmenu') {
        await row().trigger(source);
      } else {
        await row().trigger('keydown', { key: source, shiftKey: source === 'F10' });
      }
      await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
      expect(document.body.querySelector('[role="menu"]')).toBeNull();
    },
  );
  it('cancels a pending menu on pointerdown outside the clicked row even if propagation stops', async () => {
    render();
    await row().trigger('click');
    document.body.addEventListener('pointerdown', (event) => event.stopPropagation(), {
      once: true,
    });
    document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
  });
  it('keeps the pending menu on pointerdown inside the row, then cancels on an outside pointerdown', async () => {
    render();
    await row().trigger('click');
    await row().get('[aria-label="Add to favorites"]').trigger('pointerdown');
    document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
  });
  it('preserves the single-click timer on pointerdown inside the clicked row', async () => {
    render();
    await row().trigger('click');
    await row().get('[aria-label="More actions"]').trigger('pointerdown');
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
    expect(document.body.querySelector('[role="menu"][aria-label="Open with"]')).not.toBeNull();
  });
  it.each(['timer', 'outside', 'suspend', 'unmount'])(
    'removes the pending pointerdown listener after %s',
    async (action) => {
      const addListener = vi.spyOn(document, 'addEventListener');
      const removeListener = vi.spyOn(document, 'removeEventListener');
      try {
        const view = render();
        await row().trigger('click');
        const listener = addListener.mock.calls.find(([type]) => type === 'pointerdown')![1];
        removeListener.mockClear();
        if (action === 'timer') await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
        else if (action === 'outside')
          document.body.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
        else if (action === 'suspend') await view.setProps({ suspended: true });
        else {
          view.unmount();
          wrapper = undefined;
        }
        expect(removeListener).toHaveBeenCalledWith('pointerdown', listener, true);
      } finally {
        addListener.mockRestore();
        removeListener.mockRestore();
      }
    },
  );
  it('shows the unavailable reason and prevents launch from a disabled item', async () => {
    useToolsStore().tools[0]!.available = false;
    useToolsStore().tools[0]!.reason = 'Sublime is not installed';
    render();
    await openWith();
    const item = menuItem('Sublime Text');
    expect(item.getAttribute('aria-disabled')).toBe('true');
    expect(item.textContent).toContain('Sublime is not installed');
    item.click();
    await nextTick();
    expect(backend.openProject).not.toHaveBeenCalled();
    expect(backend.openProjectDefault).not.toHaveBeenCalled();
  });
  it('selecting Zed launches that target', async () => {
    render();
    await openWith();
    menuItem('Zed').click();
    await flushPromises();
    expect(backend.openProject).toHaveBeenCalledWith('alpha', 'zed');
  });
  it('a real click/click/dblclick sequence cancels the menu and launches the default editor', async () => {
    render();
    await row().trigger('pointerdown');
    await row().trigger('click', { detail: 1 });
    await row().trigger('pointerdown');
    await row().trigger('click', { detail: 2 });
    await row().trigger('dblclick');
    await vi.advanceTimersByTimeAsync(DOUBLE_CLICK_DELAY_MS);
    expect(backend.openProjectDefault).toHaveBeenCalledExactlyOnceWith('alpha');
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
  });
  it('double-click does not launch when the setting is off', async () => {
    useAppStore().settings.doubleClickOpensEditor = false;
    render();
    await row().trigger('dblclick');
    expect(backend.openProjectDefault).not.toHaveBeenCalled();
  });
  it('uses the custom terminal tool name instead of Ghostty', async () => {
    useAppStore().settings.preferredTerminal = TerminalMode.TerminalModeCustom;
    render();
    await openWith();
    expect(menuItem('Kitty')).toBeDefined();
    expect(document.body.querySelector('[role="menu"]')?.textContent).not.toContain('Ghostty');
  });
  it('missing directories show a badge, disable all launch items and offer editing', async () => {
    useGitStore().inspections = {
      alpha: inspection({ directory: DirectoryState.DirectoryMissing }),
    };
    const view = render();
    expect(row().text()).toContain('Missing');
    await openWith();
    for (const name of ['Sublime Text', 'Zed', 'Ghostty', 'File Manager']) {
      const item = menuItem(name);
      expect(item.getAttribute('aria-disabled')).toBe('true');
      item.click();
    }
    expect(backend.openProject).not.toHaveBeenCalled();
    menuItem('Edit project…').click();
    await nextTick();
    expect(view.emitted('edit')).toEqual([['alpha']]);
  });
  it('right-click offers edit, favorites, copy and remove; remove emits the project id', async () => {
    const view = render();
    await row().trigger('contextmenu');
    for (const label of ['Edit…', 'Add to favorites', 'Copy path', 'Remove from ProjectHub…'])
      expect(menuItem(label)).toBeDefined();
    menuItem('Remove from ProjectHub…').click();
    expect(view.emitted('remove')).toEqual([['alpha']]);
  });
  it('focuses the first enabled Open-with item after switching from the context menu', async () => {
    useToolsStore().tools[0]!.available = false;
    render();
    await row().trigger('contextmenu');
    const openWithItem = menuItem('Open with…');
    openWithItem.focus();
    openWithItem.click();
    await flushPromises();
    const menu = document.body.querySelector('[role="menu"][aria-label="Open with"]')!;
    expect(menu).not.toBeNull();
    expect(document.activeElement).toBe(menuItem('Zed'));
    menu.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true }));
    expect(document.activeElement).toBe(menuItem('Ghostty'));
  });
  it('Refresh status inspects only the context-menu project', async () => {
    vi.mocked(backend.inspectProjects).mockResolvedValue([inspection()]);
    render();
    await row().trigger('contextmenu');
    menuItem('Refresh status').click();
    await flushPromises();
    expect(backend.inspectProjects).toHaveBeenCalledExactlyOnceWith(['alpha']);
    expect(useGitStore().get('alpha')).toEqual(inspection());
  });
  it('makes only the first visible row tabbable when there is no selection', () => {
    const view = render();
    expect(useProjectsStore().selectedId).toBeNull();
    expect(
      view.findAll('[role="option"][tabindex="0"]').map((row) => row.attributes('data-project-id')),
    ).toEqual(['alpha']);
    expect(row().attributes('aria-selected')).toBe('false');
  });
  it('selects a row when it receives keyboard focus', async () => {
    render();
    (row().element as HTMLElement).focus();
    await nextTick();
    expect(useProjectsStore().selectedId).toBe('alpha');
    expect(row().attributes('aria-selected')).toBe('true');
  });
  it('selects Alpha when its favorite button is clicked and Delete targets Alpha', async () => {
    const store = useProjectsStore();
    store.select('beta');
    vi.mocked(backend.setFavorite).mockResolvedValue(project('alpha', { favorite: true }));
    const view = render();
    const favorite = row().get('button[title="Add to favorites"]');
    await favorite.trigger('click');
    expect(store.selectedId).toBe('alpha');
    expect(row().attributes('aria-selected')).toBe('true');
    (favorite.element as HTMLButtonElement).focus();
    await favorite.trigger('keydown', { key: 'Delete' });
    expect(view.emitted('remove')).toEqual([['alpha']]);
  });
  it.each(['Enter', ' '])('leaves %s on a focused star to the native button', async (key) => {
    const view = render();
    const favorite = row().get('[aria-label="Add to favorites"]');
    (favorite.element as HTMLButtonElement).focus();
    const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true });
    favorite.element.dispatchEvent(event);
    await nextTick();
    expect(event.defaultPrevented).toBe(false);
    expect(document.body.querySelector('[role="menu"]')).toBeNull();
    expect(view.emitted('edit')).toBeUndefined();
    expect(backend.openProjectDefault).not.toHaveBeenCalled();
  });
  it('focus inside Alpha selects it and F2 on its More actions button edits Alpha', async () => {
    const store = useProjectsStore();
    store.select('beta');
    const view = render();
    const button = row().get('[aria-label="More actions"]');
    (button.element as HTMLButtonElement).focus();
    await nextTick();
    expect(store.selectedId).toBe('alpha');
    expect(row().attributes('aria-selected')).toBe('true');
    await button.trigger('keydown', { key: 'F2' });
    expect(view.emitted('edit')).toEqual([['alpha']]);
  });
  it('resolves the keyboard target from its row even when another project is selected', async () => {
    const store = useProjectsStore();
    store.select('beta');
    const view = render();
    await row().get('[aria-label="More actions"]').trigger('keydown', { key: 'F2' });
    expect(view.emitted('edit')).toEqual([['alpha']]);
    expect(store.selectedId).toBe('alpha');
  });
  it('Ctrl+Enter on a row button still launches that row project', async () => {
    useProjectsStore().select('beta');
    render();
    await row().get('[aria-label="Add to favorites"]').trigger('keydown', {
      key: 'Enter',
      ctrlKey: true,
    });
    expect(backend.openProjectDefault).toHaveBeenCalledExactlyOnceWith('alpha');
  });
  it('makes the first visible row tabbable when a tab hides the selected project', async () => {
    const store = useProjectsStore();
    store.projects[1]!.favorite = true;
    store.select('alpha');
    const view = render();
    store.tab = 'favorites';
    await nextTick();
    expect(store.selectedId).toBe('alpha');
    expect(
      view.findAll('[role="option"][tabindex="0"]').map((row) => row.attributes('data-project-id')),
    ).toEqual(['beta']);
  });
  it.each([null, 'alpha'])(
    'F2 selects and edits the first visible row with selection %s',
    async (selectedId) => {
      const store = useProjectsStore();
      store.projects[1]!.favorite = true;
      store.select(selectedId);
      store.tab = 'favorites';
      const view = render();
      await view.get('[role="listbox"]').trigger('keydown', { key: 'F2' });
      expect(view.emitted('edit')).toEqual([['beta']]);
      expect(store.selectedId).toBe('beta');
    },
  );
  it('Copy path uses the backend clipboard and reports success', async () => {
    render();
    await row().trigger('contextmenu');
    menuItem('Copy path').click();
    await flushPromises();
    expect(backend.copyText).toHaveBeenCalledWith('/home/dev/alpha');
    expect(useNotificationsStore().items).toEqual(
      expect.arrayContaining([expect.objectContaining({ tone: 'success', title: 'Path copied' })]),
    );
  });
  it('ArrowDown/Up and Home/End move selection and focus', async () => {
    const view = render();
    useProjectsStore().select('alpha');
    await nextTick();
    for (const [key, id] of [
      ['ArrowDown', 'beta'],
      ['ArrowUp', 'alpha'],
      ['End', 'gamma'],
      ['Home', 'alpha'],
    ]) {
      await view.get('[role="listbox"]').trigger('keydown', { key });
      await nextTick();
      const target = view.get(`[data-project-id="${id}"]`);
      expect(target.attributes('aria-selected')).toBe('true');
      expect(document.activeElement).toBe(target.element);
      expect(view.findAll('[aria-selected="true"]')).toHaveLength(1);
    }
  });
  it.each([
    { key: 'Enter', ctrlKey: false },
    { key: 'Enter', ctrlKey: true },
    { key: 'F2', ctrlKey: false },
    { key: 'Delete', ctrlKey: false },
    { key: 'F10', ctrlKey: false, shiftKey: true },
  ])('handles keyboard shortcut $key ctrl=$ctrlKey', async (event) => {
    const view = render();
    useProjectsStore().select('alpha');
    await view.get('[role="listbox"]').trigger('keydown', event);
    await flushPromises();
    if (event.ctrlKey) expect(backend.openProjectDefault).toHaveBeenCalledWith('alpha');
    else if (event.key === 'F2') expect(view.emitted('edit')).toEqual([['alpha']]);
    else if (event.key === 'Delete') expect(view.emitted('remove')).toEqual([['alpha']]);
    else
      expect(
        document.body.querySelector(
          `[role="menu"][aria-label="${event.key === 'F10' ? 'Project actions' : 'Open with'}"]`,
        ),
      ).not.toBeNull();
  });
  it('empty search offers Clear search and emits clearSearch', async () => {
    useProjectsStore().searchQuery = 'no match';
    const view = render();
    expect(view.text()).toContain('No projects match');
    await view.get('button').trigger('click');
    expect(view.emitted('clearSearch')).toEqual([[]]);
  });
  it('empty favorites explains how to pin a project', () => {
    useProjectsStore().tab = 'favorites';
    expect(render().text()).toContain('No favorites yet');
  });
  it('Retry reloads the catalog after an initial load error', async () => {
    const store = useProjectsStore();
    store.loaded = false;
    store.loadError = 'Cannot read catalog';
    const view = render();
    expect(view.get('[role="alert"]').text()).toContain('Cannot read catalog');
    await view.get('button').trigger('click');
    await flushPromises();
    expect(backend.listProjects).toHaveBeenCalledOnce();
  });
  it('shows reload errors even after a successful load (empty catalog)', async () => {
    const store = useProjectsStore();
    store.projects = [];
    store.loaded = true;
    store.loadError = 'corrupted configuration file';
    const view = render();
    const alert = view.get('[role="alert"]');
    expect(alert.text()).toContain('could not be reloaded');
    expect(alert.text()).toContain('corrupted configuration file');
    expect(view.text()).toContain('No projects yet');
    vi.mocked(backend.listProjects).mockResolvedValue([]);
    await alert.get('button').trigger('click');
    await flushPromises();
    expect(backend.listProjects).toHaveBeenCalledOnce();
  });
  it('keeps showing the loaded list with a reload error', () => {
    const store = useProjectsStore();
    store.loaded = true;
    store.loadError = 'permission denied';
    const view = render();
    expect(view.get('[role="alert"]').text()).toContain('permission denied');
    expect(view.find('[role="listbox"]').exists()).toBe(true);
  });
  it.each(['global', 'project', 'remembered'] as const)(
    'Default marks the effective %s editor',
    async (source) => {
      const app = useAppStore();
      app.settings.defaultEditor = 'zed';
      if (source === 'project') useProjectsStore().projects[0]!.defaultEditor = 'sublime';
      if (source === 'remembered') useProjectsStore().projects[0]!.lastUsedEditor = 'sublime';
      render();
      await openWith();
      const expected = source === 'global' ? 'Zed' : 'Sublime Text';
      expect(menuItem(expected).textContent).toContain('Default');
      expect(menuItem(source === 'global' ? 'Sublime Text' : 'Zed').textContent).not.toContain(
        'Default',
      );
    },
  );
});
