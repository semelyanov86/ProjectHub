import { describe, expect, it, vi } from 'vitest';
import { ProjectSort } from '@/services/backend';
import type { Project } from '@/services/backend';
import { createDefaultSettings } from '@/stores/app';
import { EDITOR_IDS } from './editors';
import {
  abbreviateHome,
  avatarHue,
  effectiveEditor,
  initials,
  lastOpenedTime,
  matchesQuery,
  relativeTime,
  visibleProjects,
} from './projects';
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

describe('project helpers', () => {
  it('abbreviates paths inside the home directory only', () => {
    expect(abbreviateHome('/home/dev/code/app', '/home/dev')).toBe('~/code/app');
    expect(abbreviateHome('/home/dev', '/home/dev/')).toBe('~');
    expect(abbreviateHome('/home/developer/app', '/home/dev')).toBe('/home/developer/app');
    expect(abbreviateHome('/srv/app', '')).toBe('/srv/app');
    expect(abbreviateHome('/srv/app', null)).toBe('/srv/app');
  });

  it.each(['alpha', ' DETAILS ', '/CODE/ALPHA', '', '   '])(
    'matches name, description, path or empty query: %s',
    (query) => {
      expect(matchesQuery(project('Alpha', { description: 'Details' }), query)).toBe(true);
    },
  );
  it('rejects an unrelated query', () =>
    expect(matchesQuery(project('Alpha'), 'beta')).toBe(false));
  const entries = [
    project('app10'),
    project('z', { favorite: true }),
    project('App2'),
    project('a', { favorite: true }),
  ];
  it('pins favorites and uses natural case-insensitive name ordering without mutating input', () => {
    const original = entries.slice();
    expect(
      visibleProjects(entries, { tab: 'all', query: '', sort: ProjectSort.ProjectSortName }).map(
        (p) => p.id,
      ),
    ).toEqual(['a', 'z', 'App2', 'app10']);
    expect(entries).toEqual(original);
  });
  it('sorts last-opened descending within both groups with never-opened last', () => {
    const data = [
      project('f0', { favorite: true }),
      project('n0'),
      project('f1', { favorite: true, lastOpened: '2026-11-01' }),
      project('n1', { lastOpened: '2026-11-01' }),
      project('f2', { favorite: true, lastOpened: '2026-11-02' }),
      project('n2', { lastOpened: '2026-11-02' }),
    ];
    expect(
      visibleProjects(data, { tab: 'all', query: '', sort: ProjectSort.ProjectSortLastOpened }).map(
        (p) => p.id,
      ),
    ).toEqual(['f2', 'f1', 'f0', 'n2', 'n1', 'n0']);
  });
  it('filters favorites and combines search with tabs', () => {
    expect(
      visibleProjects(entries, {
        tab: 'favorites',
        query: '',
        sort: ProjectSort.ProjectSortName,
      }).map((p) => p.id),
    ).toEqual(['a', 'z']);
    expect(
      visibleProjects(entries, {
        tab: 'favorites',
        query: 'z',
        sort: ProjectSort.ProjectSortName,
      }).map((p) => p.id),
    ).toEqual(['z']);
    expect(
      visibleProjects(entries, { tab: 'all', query: 'app', sort: ProjectSort.ProjectSortName }).map(
        (p) => p.id,
      ),
    ).toEqual(['App2', 'app10']);
  });
  it.each([ProjectSort.ProjectSortName, ProjectSort.ProjectSortLastOpened])(
    'recent ignores chosen sort %s and favorites pinning',
    (sort) => {
      const data = [
        project('a', { favorite: true, lastOpened: '2026-11-01' }),
        project('z', { lastOpened: '2026-11-02' }),
        project('never'),
        project('bad', { lastOpened: 'invalid' }),
      ];
      expect(visibleProjects(data, { tab: 'recent', query: '', sort }).map((p) => p.id)).toEqual([
        'z',
        'a',
      ]);
      expect(visibleProjects(data, { tab: 'recent', query: 'a', sort }).map((p) => p.id)).toEqual([
        'a',
      ]);
    },
  );
  it('resolves explicit, remembered and global editors while ignoring unsupported project values', () => {
    const settings = { ...createDefaultSettings(), defaultEditor: 'zed' };
    expect(
      effectiveEditor(
        project('a', { defaultEditor: 'sublime', lastUsedEditor: 'zed' }),
        settings,
        EDITOR_IDS,
      ),
    ).toBe('sublime');
    expect(
      effectiveEditor(
        project('a', { defaultEditor: 'unsupported', lastUsedEditor: 'sublime' }),
        settings,
        EDITOR_IDS,
      ),
    ).toBe('sublime');
    expect(
      effectiveEditor(
        project('a', { lastUsedEditor: 'sublime' }),
        { ...settings, rememberLastEditor: false },
        EDITOR_IDS,
      ),
    ).toBe('zed');
    expect(
      effectiveEditor(
        project('a', { defaultEditor: 'unknown', lastUsedEditor: 'unknown' }),
        settings,
        EDITOR_IDS,
      ),
    ).toBe('zed');
  });
  it.each([
    ['ProjectHub', 'PR'],
    ['my-cool app', 'MC'],
    ['', '?'],
    ['  ', '?'],
    ['ёж проект', 'ЁП'],
    ['😀rocket', '😀R'],
  ])('initials for %s', (name, expected) => expect(initials(name)).toBe(expected));
  it.each(['', 'project', 'another-project', '😀'])('hue is stable and bounded for %s', (id) => {
    expect(avatarHue(id)).toBe(avatarHue(id));
    expect(avatarHue(id)).toBeGreaterThanOrEqual(0);
    expect(avatarHue(id)).toBeLessThan(360);
  });
  it.each([null, undefined, 'invalid'])('invalid last opened %s', (date) => {
    expect(relativeTime(date, 0)).toBe('Never opened');
    expect(lastOpenedTime(project('a', { lastOpened: date }))).toBe(0);
  });
  it.each([
    [0, 'just now'],
    [30_000, 'just now'],
    [300_000, '5 min ago'],
    [7_200_000, '2 h ago'],
    [259_200_000, '3 d ago'],
    [-60_000, 'just now'],
  ])('relative time at delta %i', (delta, expected) => {
    const now = Date.parse('2026-11-10T12:00:00Z');
    expect(relativeTime(new Date(now - delta).toISOString(), now)).toBe(expected);
  });
});
