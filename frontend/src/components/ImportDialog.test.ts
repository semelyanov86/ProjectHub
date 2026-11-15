import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, disposePinia, setActivePinia } from 'pinia';
import { DOMWrapper, flushPromises, mount } from '@vue/test-utils';
import type { VueWrapper } from '@vue/test-utils';
import { backend, ImportMode } from '@/services/backend';
import type { ImportPreview, ImportReport } from '@/services/backend';
import ImportDialog from './ImportDialog.vue';

vi.mock('@/services/backend', () => ({
  ImportMode: { ImportMerge: 'merge', ImportReplace: 'replace' },
  backend: { applyImport: vi.fn(), discardImport: vi.fn() },
}));

let wrapper: VueWrapper | undefined;
let pinia: ReturnType<typeof createPinia>;

function preview(overrides: Partial<ImportPreview> = {}): ImportPreview {
  return {
    cancelled: false,
    token: 'import-token',
    fileName: 'projects.json',
    total: 5,
    eligible: 4,
    new: 3,
    duplicates: 1,
    current: 2,
    skipped: [
      { name: 'Missing project', path: '/other/computer/missing', reason: 'Directory not found' },
    ],
    ...overrides,
  };
}

function report(overrides: Partial<ImportReport> = {}): ImportReport {
  return { mode: ImportMode.ImportMerge, added: 3, skipped: null, backupPath: '', ...overrides };
}

async function openDialog(value: ImportPreview | null = preview()) {
  wrapper = mount(ImportDialog, {
    attachTo: document.body,
    props: { open: true, preview: value },
    global: { plugins: [pinia] },
  });
  await flushPromises();
  return wrapper;
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

async function selectMode(value: 'merge' | 'replace') {
  await new DOMWrapper(
    document.querySelector<HTMLInputElement>(`input[value="${value}"]`)!,
  ).setValue(true);
}

async function confirmReplace() {
  await new DOMWrapper(
    document.querySelector<HTMLInputElement>('input[type="checkbox"]')!,
  ).setValue(true);
}

function text() {
  return document.body.textContent?.replace(/\s+/g, ' ');
}

beforeEach(() => {
  vi.resetAllMocks();
  pinia = createPinia();
  setActivePinia(pinia);
  vi.mocked(backend.applyImport).mockResolvedValue(report());
  vi.mocked(backend.discardImport).mockResolvedValue(undefined);
});

afterEach(() => {
  wrapper?.unmount();
  wrapper = undefined;
  disposePinia(pinia);
  document.body.innerHTML = '';
});

describe('ImportDialog', () => {
  it('renders file name, all summary counts, mode counts and skipped details', async () => {
    await openDialog();
    expect(text()).toContain('From projects.json');
    expect(text()).toContain('5 projects in file');
    expect(text()).toContain('3 new');
    expect(text()).toContain('1 already registered');
    expect(text()).toContain('1 skipped — directory not found on this computer');
    expect(text()).toContain('Merge — add 3 new projects, keep existing ones');
    expect(text()).toContain('Replace — replace all 2 current projects with 4 imported ones');
    const details = document.querySelector('details')!;
    details.open = true;
    expect(details.textContent).toContain('Missing project');
    expect(details.textContent).toContain('/other/computer/missing');
    expect(details.textContent).toContain('Directory not found');
  });

  it('defaults to merge and applies the preview, then shows and emits the result', async () => {
    const view = await openDialog();
    expect(document.querySelector<HTMLInputElement>('input[value="merge"]')?.checked).toBe(true);
    await click('Import');
    expect(backend.applyImport).toHaveBeenCalledExactlyOnceWith({
      token: 'import-token',
      mode: 'merge',
      replaceConfirmed: false,
    });
    expect(text()).toContain('Imported 3 projects');
    expect(document.querySelector('form')).toBeNull();
    expect(view.emitted('imported')).toEqual([[report()]]);
    expect(view.emitted('close')).toBeUndefined();
    expect(button('Done')).toBeDefined();
    expect(button('Cancel')).toBeUndefined();
  });

  it('requires explicit replace confirmation and passes it to the backend', async () => {
    await openDialog();
    await selectMode('replace');
    expect(button('Replace projects').disabled).toBe(true);
    expect(text()).toContain(
      'Your current project list will be replaced. A backup copy of projects.json is saved first.',
    );
    expect(text()).toContain('I understand that all 2 current projects will be replaced');
    await click('Replace projects');
    expect(backend.applyImport).not.toHaveBeenCalled();
    await confirmReplace();
    expect(button('Replace projects').disabled).toBe(false);
    await click('Replace projects');
    expect(backend.applyImport).toHaveBeenCalledExactlyOnceWith({
      token: 'import-token',
      mode: 'replace',
      replaceConfirmed: true,
    });
  });

  it('clears replace confirmation when returning to merge', async () => {
    await openDialog();
    await selectMode('replace');
    await confirmReplace();
    await selectMode('merge');
    await selectMode('replace');
    expect(document.querySelector<HTMLInputElement>('input[type="checkbox"]')?.checked).toBe(false);
    expect(button('Replace projects').disabled).toBe(true);
  });

  it('shows the backup path and skipped records in the result view', async () => {
    vi.mocked(backend.applyImport).mockResolvedValue(
      report({
        mode: ImportMode.ImportReplace,
        added: 4,
        backupPath: '/config/projecthub/projects.backup.json',
        skipped: [{ name: 'Unavailable', path: '/unavailable', reason: 'No longer exists' }],
      }),
    );
    await openDialog();
    await selectMode('replace');
    await confirmReplace();
    await click('Replace projects');
    expect(text()).toContain('Imported 4 projects');
    expect(text()).toContain('Previous list backed up to /config/projecthub/projects.backup.json');
    expect(text()).toContain('1 skipped');
    expect(text()).toContain('Unavailable');
    expect(text()).toContain('/unavailable');
    expect(text()).toContain('No longer exists');
  });

  it('shows apply errors, retains the form and permits a retry', async () => {
    const view = await openDialog();
    vi.mocked(backend.applyImport).mockRejectedValueOnce(new Error('Backup failed'));
    await click('Import');
    expect(document.querySelector('[role="alert"]')?.textContent).toContain('Backup failed');
    expect(document.querySelector('form')).not.toBeNull();
    expect(view.emitted('close')).toBeUndefined();
    expect(view.emitted('imported')).toBeUndefined();
    expect(button('Import').disabled).toBe(false);
    await click('Import');
    expect(backend.applyImport).toHaveBeenCalledTimes(2);
    expect(view.emitted('imported')).toEqual([[report()]]);
  });

  it('discards the token before emitting close on Cancel', async () => {
    let finishDiscard!: () => void;
    vi.mocked(backend.discardImport).mockReturnValue(
      new Promise((resolve) => {
        finishDiscard = resolve;
      }),
    );
    const view = await openDialog();
    await click('Cancel');
    expect(backend.discardImport).toHaveBeenCalledExactlyOnceWith('import-token');
    expect(view.emitted('close')).toBeUndefined();
    finishDiscard();
    await flushPromises();
    expect(view.emitted('close')).toEqual([[]]);
  });

  it.each(['escape', 'backdrop', 'close button'])(
    'discards an unapplied preview on %s',
    async (method) => {
      const view = await openDialog();
      const panel = document.querySelector<HTMLElement>('[role="dialog"]')!;
      if (method === 'escape') {
        panel.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
      } else if (method === 'backdrop') {
        const overlay = panel.parentElement!;
        overlay.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }));
        overlay.click();
      } else {
        document.querySelector<HTMLButtonElement>('[aria-label="Close"]')!.click();
      }
      await flushPromises();
      expect(backend.discardImport).toHaveBeenCalledExactlyOnceWith('import-token');
      expect(view.emitted('close')).toEqual([[]]);
    },
  );

  it('ignores discard errors and still closes', async () => {
    const view = await openDialog();
    vi.mocked(backend.discardImport).mockRejectedValue(new Error('Token expired'));
    await click('Cancel');
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('does not discard an applied import when Done closes the result', async () => {
    const view = await openDialog();
    await click('Import');
    await click('Done');
    expect(backend.discardImport).not.toHaveBeenCalled();
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('disables merge when there is nothing new and still permits confirmed replace', async () => {
    await openDialog(preview({ new: 0 }));
    expect(button('Import').disabled).toBe(true);
    expect(text()).toContain('Nothing new to import');
    await click('Import');
    expect(backend.applyImport).not.toHaveBeenCalled();
    await selectMode('replace');
    await confirmReplace();
    expect(button('Replace projects').disabled).toBe(false);
  });

  it('normalises null skipped arrays and handles an absent preview', async () => {
    const view = await openDialog(preview({ skipped: null }));
    expect(text()).toContain('0 skipped — directory not found on this computer');
    await view.setProps({ preview: null });
    expect(button('Import').disabled).toBe(true);
    await click('Cancel');
    expect(backend.discardImport).not.toHaveBeenCalled();
    expect(view.emitted('close')).toEqual([[]]);
  });

  it('blocks resubmission and closing while apply is pending', async () => {
    let finishApply!: (value: ImportReport) => void;
    vi.mocked(backend.applyImport).mockReturnValue(
      new Promise((resolve) => {
        finishApply = resolve;
      }),
    );
    const view = await openDialog();
    await click('Import');
    expect(button('Import').disabled).toBe(true);
    expect(button('Cancel').disabled).toBe(true);
    document
      .querySelector('[role="dialog"]')!
      .dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }));
    expect(backend.discardImport).not.toHaveBeenCalled();
    expect(view.emitted('close')).toBeUndefined();
    finishApply(report());
    await flushPromises();
    expect(view.emitted('imported')).toEqual([[report()]]);
  });

  it('resets to an unconfirmed merge form on each open', async () => {
    const view = await openDialog();
    await selectMode('replace');
    await confirmReplace();
    await click('Replace projects');
    await view.setProps({ open: false });
    await view.setProps({ open: true, preview: preview({ token: 'next-token' }) });
    expect(document.querySelector<HTMLInputElement>('input[value="merge"]')?.checked).toBe(true);
    expect(button('Import')).toBeDefined();
    expect(button('Done')).toBeUndefined();
    await selectMode('replace');
    expect(button('Replace projects').disabled).toBe(true);
    await click('Cancel');
    expect(backend.discardImport).toHaveBeenCalledExactlyOnceWith('next-token');
  });
});
