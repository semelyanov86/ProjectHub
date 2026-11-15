import { afterEach, describe, expect, it, vi } from 'vitest';
import { mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { ProjectSort } from '@/services/backend';
import ProjectToolbar from './ProjectToolbar.vue';
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

let wrapper: VueWrapper | undefined;
function render(searchQuery = '') {
  const view = mount(ProjectToolbar, {
    attachTo: document.body,
    props: { searchQuery, sort: ProjectSort.ProjectSortName, refreshing: false },
  });
  wrapper = view;
  return view;
}
afterEach(() => {
  wrapper?.unmount();
  document.body.innerHTML = '';
});
describe('ProjectToolbar', () => {
  it('typing updates search, Escape clears it and ArrowDown requests list focus', async () => {
    const view = render('alpha');
    const input = view.get('input');
    await input.setValue('zed');
    await input.trigger('keydown', { key: 'Escape' });
    await input.trigger('keydown', { key: 'ArrowDown' });
    expect(view.emitted('update:searchQuery')).toEqual([['zed'], ['']]);
    expect(view.emitted('searchArrowDown')).toEqual([[]]);
  });
  it('toggles sort between name and last opened', async () => {
    const view = render();
    await view.get('[aria-label="Sort by last opened"]').trigger('click');
    await view.setProps({ sort: ProjectSort.ProjectSortLastOpened });
    await view.get('[aria-label="Sort by name"]').trigger('click');
    expect(view.emitted('update:sort')).toEqual([
      [ProjectSort.ProjectSortLastOpened],
      [ProjectSort.ProjectSortName],
    ]);
  });
  it('refresh emits when enabled and is disabled while refreshing; Add emits add', async () => {
    const view = render();
    const refresh = view.get('[aria-label="Refresh Git status"]');
    await refresh.trigger('click');
    await view.setProps({ refreshing: true });
    expect((refresh.element as HTMLButtonElement).disabled).toBe(true);
    await refresh.trigger('click');
    expect(view.emitted('refresh')).toEqual([[]]);
    await view.get('[aria-label="Add project"]').trigger('click');
    expect(view.emitted('add')).toEqual([[]]);
  });
  it.each(['import', 'export'] as const)('menu emits %s', async (action) => {
    const view = render();
    await view.get('[aria-label="Import or export"]').trigger('click');
    const items = Array.from(
      document.body.querySelectorAll<HTMLButtonElement>('[role="menuitem"]'),
    );
    expect(items.map((item) => item.textContent)).toEqual(
      expect.arrayContaining([
        expect.stringContaining('Import projects'),
        expect.stringContaining('Export projects'),
      ]),
    );
    items.find((item) => item.textContent?.toLowerCase().includes(action))!.click();
    expect(view.emitted(action)).toEqual([[]]);
  });
  it('focusSearch focuses the search input', () => {
    const view = render();
    view.vm.focusSearch();
    expect(document.activeElement).toBe(view.get('input').element);
  });
});
