import { ProjectSort } from '@/services/backend';
import type { Project, Settings } from '@/services/backend';

/** Delay before a single click opens the "Open with" menu, so a double click can cancel it. */
export const DOUBLE_CLICK_DELAY_MS = 250;

export type ProjectTab = 'all' | 'favorites' | 'recent';

export interface ListOptions {
  tab: ProjectTab;
  query: string;
  sort: ProjectSort;
}

const collator = new Intl.Collator(undefined, { sensitivity: 'base', numeric: true });

/** Case-insensitive match of a search query against name, description and path. */
export function matchesQuery(project: Project, query: string): boolean {
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) {
    return true;
  }
  return (
    project.name.toLocaleLowerCase().includes(needle) ||
    project.description.toLocaleLowerCase().includes(needle) ||
    project.path.toLocaleLowerCase().includes(needle)
  );
}

/** Milliseconds since epoch of the last launch, or 0 when never opened. */
export function lastOpenedTime(project: Project): number {
  if (!project.lastOpened) {
    return 0;
  }
  const time = Date.parse(project.lastOpened);
  return Number.isNaN(time) ? 0 : time;
}

export function compareByName(a: Project, b: Project): number {
  return (
    collator.compare(a.name, b.name) || a.path.localeCompare(b.path) || a.id.localeCompare(b.id)
  );
}

/** Most recently opened first; never-opened projects last, by name. */
export function compareByLastOpened(a: Project, b: Project): number {
  return lastOpenedTime(b) - lastOpenedTime(a) || compareByName(a, b);
}

/**
 * Applies search, tab filtering and ordering:
 * - all: favorites pinned first, the chosen sort within each group;
 * - favorites: favorites only, the chosen sort;
 * - recent: opened projects only, most recent first.
 */
export function visibleProjects(projects: readonly Project[], options: ListOptions): Project[] {
  const matching = projects.filter((p) => matchesQuery(p, options.query));
  const byChosenSort =
    options.sort === ProjectSort.ProjectSortLastOpened ? compareByLastOpened : compareByName;

  switch (options.tab) {
    case 'favorites':
      return matching.filter((p) => p.favorite).sort(byChosenSort);
    case 'recent':
      return matching.filter((p) => lastOpenedTime(p) > 0).sort(compareByLastOpened);
    default:
      return matching.sort((a, b) => Number(b.favorite) - Number(a.favorite) || byChosenSort(a, b));
  }
}

/** Editor used by double-click / Ctrl+Enter, mirroring desktop.EffectiveEditor in Go. */
export function effectiveEditor(
  project: Project,
  settings: Settings,
  supported: readonly string[],
): string {
  if (supported.includes(project.defaultEditor)) {
    return project.defaultEditor;
  }
  if (
    settings.rememberLastEditor &&
    project.lastUsedEditor &&
    supported.includes(project.lastUsedEditor)
  ) {
    return project.lastUsedEditor;
  }
  return settings.defaultEditor;
}

/** Up to two initials for the project avatar. */
export function initials(name: string): string {
  const words = name
    .trim()
    .split(/[\s._-]+/u)
    .filter(Boolean);
  if (words.length === 0) {
    return '?';
  }
  if (words.length === 1) {
    return Array.from(words[0] ?? '?')
      .slice(0, 2)
      .join('')
      .toLocaleUpperCase();
  }
  return words
    .slice(0, 2)
    .map((word) => Array.from(word)[0] ?? '')
    .join('')
    .toLocaleUpperCase();
}

/** Stable hue (0-359) derived from the project id for avatar colouring. */
export function avatarHue(id: string): number {
  let hash = 0;
  for (const char of id) {
    hash = (hash * 31 + (char.codePointAt(0) ?? 0)) >>> 0;
  }
  return hash % 360;
}

/** Human-friendly relative time such as "5 min ago" for lastOpened tooltips. */
export function relativeTime(iso: string | null | undefined, now: number = Date.now()): string {
  if (!iso) {
    return 'Never opened';
  }
  const time = Date.parse(iso);
  if (Number.isNaN(time)) {
    return 'Never opened';
  }
  const seconds = Math.max(0, Math.round((now - time) / 1000));
  if (seconds < 60) {
    return 'just now';
  }
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) {
    return `${minutes} min ago`;
  }
  const hours = Math.round(minutes / 60);
  if (hours < 24) {
    return `${hours} h ago`;
  }
  const days = Math.round(hours / 24);
  if (days < 30) {
    return `${days} d ago`;
  }
  return new Date(time).toLocaleDateString();
}
