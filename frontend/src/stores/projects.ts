import { computed, ref } from 'vue';
import { defineStore } from 'pinia';
import { backend } from '@/services/backend';
import type { Project, ProjectInput, Target } from '@/services/backend';
import { errorMessage } from '@/lib/errors';
import { visibleProjects as filterProjects } from '@/lib/projects';
import type { ProjectTab } from '@/lib/projects';
import { useAppStore } from './app';
import { useNotificationsStore } from './notifications';

/**
 * Project catalog state: the registered projects, search/tab/selection UI
 * state, and catalog mutations. Mutations go through the Go backend, which
 * persists them atomically; the store then applies the returned records.
 */
export const useProjectsStore = defineStore('projects', () => {
  const app = useAppStore();
  const notifications = useNotificationsStore();

  const projects = ref<Project[]>([]);
  const loaded = ref(false);
  const loading = ref(false);
  const loadError = ref<string | null>(null);
  const searchQuery = ref('');
  const tab = ref<ProjectTab>('all');
  const selectedId = ref<string | null>(null);
  let mutationCount = 0;

  const visible = computed(() =>
    filterProjects(projects.value, {
      tab: tab.value,
      query: searchQuery.value,
      sort: app.settings.projectSort,
    }),
  );

  const counts = computed(() => ({
    all: projects.value.length,
    favorites: projects.value.filter((p) => p.favorite).length,
    recent: projects.value.filter((p) => Boolean(p.lastOpened)).length,
  }));

  const selected = computed(() => projects.value.find((p) => p.id === selectedId.value) ?? null);

  function byId(id: string): Project | undefined {
    return projects.value.find((p) => p.id === id);
  }

  function upsert(project: Project): void {
    const index = projects.value.findIndex((p) => p.id === project.id);
    if (index >= 0) {
      projects.value.splice(index, 1, project);
    } else {
      projects.value.push(project);
    }
  }

  let loadGeneration = 0;
  let latestLoad: Promise<boolean> = Promise.resolve(true);

  /**
   * Reloads the catalog. Only the most recent call may apply its response:
   * an older request that resolves late (e.g. a Refresh overtaken by a drop or
   * import reload) defers to the newest one, and a response that raced with a
   * local mutation is fetched again.
   */
  function load(): Promise<boolean> {
    const generation = ++loadGeneration;
    latestLoad = runLoad(generation);
    return latestLoad;
  }

  async function runLoad(generation: number): Promise<boolean> {
    loading.value = true;
    try {
      let version: number;
      let catalog: Project[];
      do {
        version = mutationCount;
        catalog = await backend.listProjects();
        if (generation !== loadGeneration) {
          return latestLoad;
        }
      } while (version !== mutationCount);
      projects.value = catalog;
      loaded.value = true;
      loadError.value = null;
      if (selectedId.value && !byId(selectedId.value)) {
        selectedId.value = null;
      }
      return true;
    } catch (err) {
      if (generation !== loadGeneration) {
        return latestLoad;
      }
      loadError.value = errorMessage(err);
      return false;
    } finally {
      if (generation === loadGeneration) {
        loading.value = false;
      }
    }
  }

  /** Adds a project; validation errors are rethrown for display in the dialog. */
  async function add(input: ProjectInput): Promise<Project> {
    const created = await backend.addProject(input);
    upsert(created);
    mutationCount += 1;
    selectedId.value = created.id;
    return created;
  }

  /** Updates a project; validation errors are rethrown for display in the dialog. */
  async function update(id: string, input: ProjectInput): Promise<Project> {
    const updated = await backend.updateProject(id, input);
    upsert(updated);
    mutationCount += 1;
    return updated;
  }

  async function remove(id: string): Promise<boolean> {
    const project = byId(id);
    try {
      await backend.removeProject(id);
      projects.value = projects.value.filter((p) => p.id !== id);
      mutationCount += 1;
      if (selectedId.value === id) {
        selectedId.value = null;
      }
      notifications.success(
        'Project removed',
        `${project?.name ?? 'Project'} was removed from ProjectHub. Files on disk were not touched.`,
      );
      return true;
    } catch (err) {
      notifications.error('Could not remove project', errorMessage(err));
      return false;
    }
  }

  async function setFavorite(id: string, favorite: boolean): Promise<boolean> {
    try {
      upsert(await backend.setFavorite(id, favorite));
      mutationCount += 1;
      return true;
    } catch (err) {
      notifications.error('Could not update favorite', errorMessage(err));
      return false;
    }
  }

  function toggleFavorite(id: string): Promise<boolean> {
    const project = byId(id);
    return setFavorite(id, !(project?.favorite ?? false));
  }

  /** Opens a project with a target application, or with its effective editor when target is omitted. */
  async function open(id: string, target?: Target): Promise<boolean> {
    const project = byId(id);
    try {
      const result = target
        ? await backend.openProject(id, target)
        : await backend.openProjectDefault(id);
      const index = projects.value.findIndex(
        (p) => p.id === result.project.id && p.path === result.project.path,
      );
      if (index >= 0) {
        projects.value.splice(index, 1, result.project);
        mutationCount += 1;
      }
      if (result.warning) {
        notifications.warning('Opened with a warning', result.warning);
        void load();
      }
      return true;
    } catch (err) {
      notifications.error(`Could not open ${project?.name ?? 'project'}`, errorMessage(err));
      return false;
    }
  }

  function select(id: string | null): void {
    selectedId.value = id;
  }

  return {
    projects,
    loaded,
    loading,
    loadError,
    searchQuery,
    tab,
    selectedId,
    visible,
    counts,
    selected,
    byId,
    upsert,
    load,
    add,
    update,
    remove,
    setFavorite,
    toggleFavorite,
    open,
    select,
  };
});
