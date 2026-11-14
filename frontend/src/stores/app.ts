import { computed, ref } from 'vue';
import { defineStore } from 'pinia';
import { backend, ThemeMode } from '@/services/backend';
import type { Info, Project, Settings } from '@/services/backend';

export type ActiveView = 'projects' | 'settings';

export function createDefaultSettings(): Settings {
  return {
    schemaVersion: 1,
    defaultEditor: 'vscode',
    preferredTerminal: 'ghostty',
    preferredFileManager: 'auto',
    theme: ThemeMode.ThemeSystem,
    window: {
      width: 440,
      height: 680,
      rememberSize: true,
      alwaysOnTop: false,
    },
  };
}

function formatError(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

function cloneSettings(s: Settings): Settings {
  return {
    ...s,
    window: { ...s.window },
  };
}

export const useAppStore = defineStore('app', () => {
  const appInfo = ref<Info | null>(null);
  const settings = ref<Settings>(createDefaultSettings());
  const settingsLoaded = ref(false);
  const projects = ref<Project[]>([]);
  const searchQuery = ref('');
  const activeView = ref<ActiveView>('projects');
  const loading = ref(false);
  const saving = ref(false);
  const error = ref<string | null>(null);

  let pendingSettingsSaves = 0;
  let settingsSaveQueue: Promise<void> = Promise.resolve();

  const filteredProjects = computed(() => {
    const query = searchQuery.value.trim().toLowerCase();
    const list = query
      ? projects.value.filter(
          (p) =>
            p.name.toLowerCase().includes(query) ||
            p.description.toLowerCase().includes(query) ||
            p.path.toLowerCase().includes(query),
        )
      : [...projects.value];

    return list.sort((a, b) => {
      if (a.favorite !== b.favorite) {
        return a.favorite ? -1 : 1;
      }
      return a.name.localeCompare(b.name);
    });
  });

  async function initialize(): Promise<void> {
    loading.value = true;
    error.value = null;

    const [infoResult, settingsResult, projectsResult] = await Promise.allSettled([
      backend.getAppInfo(),
      backend.getSettings(),
      backend.listProjects(),
    ]);

    const errors: string[] = [];

    if (infoResult.status === 'fulfilled') {
      appInfo.value = infoResult.value;
    } else {
      errors.push(formatError(infoResult.reason));
    }

    if (settingsResult.status === 'fulfilled') {
      settings.value = settingsResult.value;
      settingsLoaded.value = true;
    } else {
      settingsLoaded.value = false;
      errors.push(formatError(settingsResult.reason));
    }

    if (projectsResult.status === 'fulfilled') {
      projects.value = projectsResult.value ?? [];
    } else {
      errors.push(formatError(projectsResult.reason));
    }

    if (errors.length > 0) {
      error.value = errors.join('; ');
    }

    loading.value = false;
  }

  async function persistSettings(buildNext: () => Settings): Promise<boolean> {
    pendingSettingsSaves += 1;
    saving.value = true;

    const run = settingsSaveQueue.then(async () => {
      error.value = null;
      try {
        const saved = await backend.updateSettings(buildNext());
        settings.value = saved;
        return true;
      } catch (err) {
        error.value = formatError(err);
        return false;
      } finally {
        pendingSettingsSaves -= 1;
        if (pendingSettingsSaves === 0) {
          saving.value = false;
        }
      }
    });

    settingsSaveQueue = run.then(
      () => undefined,
      () => undefined,
    );

    return run;
  }

  async function updateSettings(next: Settings): Promise<boolean> {
    if (!settingsLoaded.value) {
      error.value = 'Cannot save preferences because existing settings failed to load.';
      return false;
    }

    const snapshot = cloneSettings(next);
    return persistSettings(() => snapshot);
  }

  async function cycleTheme(): Promise<void> {
    if (!settingsLoaded.value) {
      error.value = 'Cannot change theme because existing settings failed to load.';
      return;
    }

    const order: ThemeMode[] = [ThemeMode.ThemeSystem, ThemeMode.ThemeDark, ThemeMode.ThemeLight];
    await persistSettings(() => {
      const currentIndex = order.indexOf(settings.value.theme);
      const nextTheme = order[(currentIndex + 1) % order.length] ?? ThemeMode.ThemeSystem;
      return {
        ...settings.value,
        theme: nextTheme,
        window: { ...settings.value.window },
      };
    });
  }

  function setView(view: ActiveView): void {
    activeView.value = view;
  }

  return {
    appInfo,
    settings,
    settingsLoaded,
    projects,
    searchQuery,
    activeView,
    loading,
    saving,
    error,
    filteredProjects,
    initialize,
    updateSettings,
    cycleTheme,
    setView,
  };
});
