import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { DOMWrapper, flushPromises, mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { backend, GitState } from '@/services/backend';
import type { DirectoryDraft, Project } from '@/services/backend';
import { useAppStore } from '@/stores/app';
import ProjectDialog from './ProjectDialog.vue';

vi.mock('@/services/backend', () => ({
  Target: {
    TargetSublime: 'sublime',
    TargetZed: 'zed',
    TargetGhostty: 'ghostty',
    TargetTerminal: 'terminal',
    TargetFileManager: 'file_manager',
  },
  ThemeMode: { ThemeSystem: 'system', ThemeDark: 'dark', ThemeLight: 'light' },
  TerminalMode: { TerminalModeGhostty: 'ghostty', TerminalModeCustom: 'custom' },
  CloseBehavior: { CloseBehaviorTray: 'tray', CloseBehaviorQuit: 'quit' },
  FileManagerMode: { FileManagerModeAuto: 'auto', FileManagerModeCustom: 'custom' },
  ProjectSort: { ProjectSortName: 'name', ProjectSortLastOpened: 'lastOpened' },
  GitState: {
    StateRepo: 'repo',
    StateNotRepo: 'not_repo',
    StateUnavailable: 'unavailable',
    StateError: 'error',
  },
  backend: { addProject: vi.fn(), updateProject: vi.fn(), chooseDirectory: vi.fn() },
}));

let wrapper: VueWrapper | undefined;
let pinia: ReturnType<typeof createPinia>;

function project(overrides: Partial<Project> = {}): Project {
  return {
    id: 'project-1',
    name: 'ProjectHub',
    description: 'Desktop projects',
    path: '/projects/projecthub',
    defaultEditor: 'sublime',
    favorite: true,
    createdAt: '2026-11-15T10:00:00Z',
    updatedAt: '2026-11-15T10:00:00Z',
    ...overrides,
  };
}

function draft(overrides: Partial<DirectoryDraft> = {}): DirectoryDraft {
  return {
    cancelled: false,
    path: '/projects/projecthub',
    suggestedName: 'ProjectHub',
    duplicateId: '',
    duplicateName: '',
    git: {
      state: GitState.StateRepo,
      message: '',
      repo: {
        branch: 'main',
        headSha: '1234567890abcdef',
        detached: false,
        unborn: false,
        isDirty: true,
        dirtyUncertain: false,
        upstream: '',
        ahead: 0,
        behind: 0,
      },
    },
    ...overrides,
  };
}

async function openDialog(
  props: {
    open?: boolean;
    mode?: 'add' | 'edit';
    draft?: DirectoryDraft | null;
    project?: Project | null;
  } = {},
) {
  wrapper = mount(ProjectDialog, {
    attachTo: document.body,
    props: { open: true, mode: 'add', ...props },
    global: { plugins: [pinia] },
  });
  await flushPromises();
  return wrapper;
}

function input(field: string) {
  return new DOMWrapper(document.body.querySelector<HTMLInputElement>(`input[id$="-${field}"]`)!);
}

function button(label: string) {
  return Array.from(document.body.querySelectorAll('button')).find(
    (element) => element.textContent?.trim() === label,
  )!;
}

async function click(label: string) {
  button(label).click();
  await flushPromises();
}

function text() {
  return document.body.textContent?.replace(/\s+/g, ' ');
}

beforeEach(() => {
  vi.resetAllMocks();
  pinia = createPinia();
  setActivePinia(pinia);
  useAppStore().settings.defaultEditor = 'zed';
  vi.mocked(backend.addProject).mockResolvedValue(project());
  vi.mocked(backend.updateProject).mockResolvedValue(project());
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  disposePinia(pinia);
  document.body.innerHTML = '';
});

describe('ProjectDialog', () => {
  it('prefills an add draft, displays its Git branch and focuses the name', async () => {
    await openDialog({ draft: draft() });
    expect(input('path').element.value).toBe('/projects/projecthub');
    expect(input('name').element.value).toBe('ProjectHub');
    expect(text()).toContain('Git repository · main · uncommitted changes');
    expect(document.activeElement).toBe(input('name').element);
    expect(input('name').attributes('maxlength')).toBe('200');
    expect(document.querySelector('textarea')?.getAttribute('maxlength')).toBe('2000');
    expect(text()).toContain('Register an existing directory. Files are never modified.');
  });

  it('focuses the directory when no draft was supplied', async () => {
    await openDialog();
    expect(document.activeElement).toBe(input('path').element);
    expect(button('Add Project').disabled).toBe(true);
  });

  it('updates an untouched name on Browse and preserves a name typed by the user', async () => {
    await openDialog({ draft: draft() });
    vi.mocked(backend.chooseDirectory).mockResolvedValue(
      draft({ path: '/projects/next', suggestedName: 'Next' }),
    );
    await click('Browse…');
    expect(input('path').element.value).toBe('/projects/next');
    expect(input('name').element.value).toBe('Next');
    await input('name').setValue('My custom name');
    vi.mocked(backend.chooseDirectory).mockResolvedValue(
      draft({ path: '/projects/third', suggestedName: 'Third' }),
    );
    await click('Browse…');
    expect(input('path').element.value).toBe('/projects/third');
    expect(input('name').element.value).toBe('My custom name');
  });

  it('leaves fields and draft information unchanged when Browse is cancelled', async () => {
    await openDialog({ draft: draft() });
    vi.mocked(backend.chooseDirectory).mockResolvedValue(
      draft({ cancelled: true, path: '', suggestedName: '' }),
    );
    await click('Browse…');
    expect(input('path').element.value).toBe('/projects/projecthub');
    expect(input('name').element.value).toBe('ProjectHub');
    expect(text()).toContain('Git repository · main');
  });

  it('displays Browse failures inline without closing', async () => {
    const view = await openDialog({ draft: draft() });
    vi.mocked(backend.chooseDirectory).mockRejectedValue(new Error('Folder picker unavailable'));
    await click('Browse…');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain(
      'Folder picker unavailable',
    );
    expect(view.emitted('close')).toBeUndefined();
  });

  it('blocks duplicate drafts and clears their Git and duplicate information after manual path edits', async () => {
    await openDialog({ draft: draft({ duplicateId: 'other', duplicateName: 'Existing' }) });
    expect(document.querySelector('[role="alert"]')?.textContent).toContain(
      'Already registered as “Existing”.',
    );
    expect(button('Add Project').disabled).toBe(true);
    await input('path').setValue('/projects/new');
    expect(document.querySelector('[role="alert"]')).toBeNull();
    expect(text()).not.toContain('Git repository');
    expect(button('Add Project').disabled).toBe(false);
  });

  it('trims name and description, preserves the path and emits saved followed by close', async () => {
    const view = await openDialog({ draft: draft() });
    await input('name').setValue('  My Project  ');
    await input('path').setValue('  /projects/my project  ');
    await new DOMWrapper(document.querySelector('textarea')!).setValue('  Description  ');
    await new DOMWrapper(document.querySelector('select')!).setValue('zed');
    await new DOMWrapper(
      document.querySelector<HTMLInputElement>('input[type="checkbox"]')!,
    ).setValue(true);
    await click('Add Project');
    expect(backend.addProject).toHaveBeenCalledExactlyOnceWith({
      name: 'My Project',
      description: 'Description',
      path: '  /projects/my project  ',
      defaultEditor: 'zed',
      favorite: true,
    });
    expect(view.emitted('saved')).toEqual([[project()]]);
    expect(view.emitted('close')).toEqual([[]]);
    expect(
      Object.keys(view.emitted()).filter((event) => ['saved', 'close'].includes(event)),
    ).toEqual(['saved', 'close']);
  });

  it('submits a draft directory ending in a space unchanged', async () => {
    await openDialog({ draft: draft({ path: '/code/project ' }) });
    await click('Add Project');
    expect(backend.addProject).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({ path: '/code/project ' }),
    );
  });

  it('preserves the form and shows backend validation errors on add', async () => {
    const view = await openDialog({ draft: draft() });
    vi.mocked(backend.addProject).mockRejectedValue(new Error('Directory does not exist'));
    await click('Add Project');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain(
      'Directory does not exist',
    );
    expect(input('name').element.value).toBe('ProjectHub');
    expect(button('Add Project').disabled).toBe(false);
    expect(view.emitted('saved')).toBeUndefined();
    expect(view.emitted('close')).toBeUndefined();
  });

  it('prefills edit fields and updates the target project', async () => {
    const target = project();
    const view = await openDialog({ mode: 'edit', project: target, draft: draft() });
    expect(input('name').element.value).toBe(target.name);
    expect(input('path').element.value).toBe(target.path);
    expect(document.querySelector('textarea')?.value).toBe(target.description);
    expect(document.querySelector('select')?.value).toBe(target.defaultEditor);
    expect(document.querySelector<HTMLInputElement>('input[type="checkbox"]')?.checked).toBe(true);
    expect(text()).not.toContain('Git repository');
    await click('Save');
    expect(backend.updateProject).toHaveBeenCalledExactlyOnceWith(target.id, {
      name: target.name,
      description: target.description,
      path: target.path,
      defaultEditor: target.defaultEditor,
      favorite: target.favorite,
    });
    expect(backend.addProject).not.toHaveBeenCalled();
    expect(view.emitted('saved')).toEqual([[target]]);
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('permits an edit draft duplicate referring to the same project and preserves its name', async () => {
    await openDialog({ mode: 'edit', project: project() });
    vi.mocked(backend.chooseDirectory).mockResolvedValue(
      draft({
        duplicateId: 'project-1',
        duplicateName: 'ProjectHub',
        suggestedName: 'Other suggestion',
      }),
    );
    await click('Browse…');
    expect(document.querySelector('[role="alert"]')).toBeNull();
    expect(button('Save').disabled).toBe(false);
    expect(input('name').element.value).toBe('ProjectHub');
  });

  it('disables submit for a whitespace name or path', async () => {
    await openDialog({ draft: draft() });
    await input('name').setValue('   ');
    expect(button('Add Project').disabled).toBe(true);
    await input('name').setValue('Valid');
    await input('path').setValue('   ');
    expect(button('Add Project').disabled).toBe(true);
    expect(backend.addProject).not.toHaveBeenCalled();
  });

  it('offers inheritance from the global Zed default plus supported editors', async () => {
    await openDialog();
    expect(
      Array.from(document.querySelectorAll('select option')).map((option) =>
        option.textContent?.trim(),
      ),
    ).toEqual(['Use global default (Zed)', 'Sublime Text', 'Zed']);
    expect(document.querySelector('select')?.value).toBe('');
  });

  it.each([
    [GitState.StateNotRepo, 'Not a Git repository'],
    [GitState.StateUnavailable, 'Git is not installed'],
    [GitState.StateError, 'Repository inspection failed'],
  ])('renders the %s Git state', async (state, label) => {
    const value = draft();
    value.git.state = state;
    value.git.message = 'Repository inspection failed';
    await openDialog({ draft: value });
    expect(text()).toContain(label);
  });

  it('shows seven SHA characters for detached HEAD', async () => {
    const value = draft();
    value.git.repo.detached = true;
    await openDialog({ draft: value });
    expect(text()).toContain('Git repository · 1234567');
    expect(text()).not.toContain('1234567890abcdef');
  });

  it('blocks resubmission and closing while a save is pending', async () => {
    let resolveSave!: (value: Project) => void;
    vi.mocked(backend.addProject).mockReturnValue(
      new Promise((resolve) => {
        resolveSave = resolve;
      }),
    );
    const view = await openDialog({ draft: draft() });
    await click('Add Project');
    expect(button('Add Project').disabled).toBe(true);
    expect(button('Cancel').disabled).toBe(true);
    document
      .querySelector('[role="dialog"]')!
      .dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(view.emitted('close')).toBeUndefined();
    resolveSave(project());
    await flushPromises();
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('resets fields and nameTouched on reopening', async () => {
    const view = await openDialog({ draft: draft() });
    await input('name').setValue('Custom');
    await view.setProps({ open: false });
    await view.setProps({ open: true, draft: draft({ suggestedName: 'Fresh' }) });
    expect(input('name').element.value).toBe('Fresh');
    vi.mocked(backend.chooseDirectory).mockResolvedValue(draft({ suggestedName: 'Picked' }));
    await click('Browse…');
    expect(input('name').element.value).toBe('Picked');
    await click('Cancel');
    expect(view.emitted('close')).toEqual([[]]);
  });
});
