import { computed, ref, shallowRef } from 'vue';
import { defineStore } from 'pinia';
import { backend, DirectoryState, GitState } from '@/services/backend';
import type { ProjectInspection } from '@/services/backend';
import { errorMessage } from '@/lib/errors';
import { useNotificationsStore } from './notifications';
import { useProjectsStore } from './projects';

/** Projects inspected per backend call (the backend caps a call at 64). */
export const INSPECT_BATCH_SIZE = 24;
/** Git data older than this is refreshed when the window regains focus. */
export const STALE_AFTER_MS = 60_000;

/**
 * Live directory and Git state per project. Loaded asynchronously in batches
 * so the UI renders immediately; never polled — refreshed on startup, on
 * explicit request, after catalog changes, and on focus when stale.
 */
export const useGitStore = defineStore('git', () => {
  const projectsStore = useProjectsStore();
  const notifications = useNotificationsStore();

  const inspections = shallowRef<Record<string, ProjectInspection>>({});
  const refreshing = ref(false);
  const lastRefreshAt = ref(0);

  let fullRefresh: Promise<void> | null = null;
  let fullRefreshQueued = false;

  const gitUnavailable = computed(() =>
    Object.values(inspections.value).some((i) => i.git.state === GitState.StateUnavailable),
  );

  function get(id: string): ProjectInspection | undefined {
    return inspections.value[id];
  }

  function isMissing(id: string): boolean {
    const directory = inspections.value[id]?.directory;
    return directory !== undefined && directory !== DirectoryState.DirectoryPresent;
  }

  /** Visible projects first, so what the user sees resolves first. */
  function orderedIds(): string[] {
    const visibleIds = projectsStore.visible.map((p) => p.id);
    const seen = new Set(visibleIds);
    return [
      ...visibleIds,
      ...projectsStore.projects.map((p) => p.id).filter((id) => !seen.has(id)),
    ];
  }

  async function inspect(ids: readonly string[]): Promise<boolean> {
    for (let start = 0; start < ids.length; start += INSPECT_BATCH_SIZE) {
      const batch = ids.slice(start, start + INSPECT_BATCH_SIZE);
      let results: ProjectInspection[];
      try {
        results = await backend.inspectProjects(batch);
      } catch (err) {
        notifications.error('Could not refresh project status', errorMessage(err));
        return false;
      }
      const next = { ...inspections.value };
      for (const result of results) {
        const project = projectsStore.byId(result.projectId);
        // Discard results for projects removed or moved while the call ran.
        if (project && project.path === result.path) {
          next[result.projectId] = result;
        }
      }
      inspections.value = next;
    }
    return true;
  }

  /** Inspects specific projects (e.g. after add or edit). */
  async function refreshProjects(ids: readonly string[]): Promise<void> {
    if (ids.length > 0) {
      await inspect(ids);
    }
  }

  /** Inspects all projects; overlapping calls coalesce into one trailing refresh. */
  function refreshAll(): Promise<void> {
    if (fullRefresh) {
      fullRefreshQueued = true;
      return fullRefresh;
    }
    refreshing.value = true;
    fullRefresh = (async () => {
      do {
        fullRefreshQueued = false;
        const ok = await inspect(orderedIds());
        if (ok) {
          lastRefreshAt.value = Date.now();
        }
      } while (fullRefreshQueued);
    })().finally(() => {
      fullRefresh = null;
      refreshing.value = false;
    });
    return fullRefresh;
  }

  /** Refreshes everything when the last full refresh is older than STALE_AFTER_MS. */
  function refreshIfStale(now: number = Date.now()): Promise<void> {
    if (now - lastRefreshAt.value < STALE_AFTER_MS) {
      return Promise.resolve();
    }
    return refreshAll();
  }

  function forget(id: string): void {
    if (id in inspections.value) {
      const next = { ...inspections.value };
      delete next[id];
      inspections.value = next;
    }
  }

  return {
    inspections,
    refreshing,
    lastRefreshAt,
    gitUnavailable,
    get,
    isMissing,
    refreshProjects,
    refreshAll,
    refreshIfStale,
    forget,
  };
});
