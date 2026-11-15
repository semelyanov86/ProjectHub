import { ref } from 'vue';
import { defineStore } from 'pinia';
import {
  backend,
  CloseBehavior,
  FileManagerMode,
  ProjectSort,
  TerminalMode,
  ThemeMode,
} from '@/services/backend';
import type { Info, Settings } from '@/services/backend';
import { errorMessage } from '@/lib/errors';

export type SettingsPatch = Partial<Omit<Settings, 'window' | 'executables' | 'customTerminal'>> & {
  window?: Partial<Settings['window']>;
  executables?: Partial<Settings['executables']>;
  customTerminal?: Partial<Settings['customTerminal']>;
};

export function createDefaultSettings(): Settings {
  return {
    schemaVersion: 2,
    defaultEditor: 'zed',
    preferredTerminal: TerminalMode.TerminalModeGhostty,
    preferredFileManager: FileManagerMode.FileManagerModeAuto,
    theme: ThemeMode.ThemeSystem,
    window: {
      width: 440,
      height: 650,
      rememberSize: true,
      alwaysOnTop: false,
    },
    executables: { sublime: '', zed: '', ghostty: '', fileManager: '' },
    customTerminal: { executable: '', args: [] },
    rememberLastEditor: true,
    doubleClickOpensEditor: true,
    showGitDirty: true,
    projectSort: ProjectSort.ProjectSortName,
    closeBehavior: CloseBehavior.CloseBehaviorTray,
  };
}

/** Deep copy so queued saves never observe later mutations of a draft. */
export function cloneSettings(s: Settings): Settings {
  return {
    ...s,
    window: { ...s.window },
    executables: { ...s.executables },
    customTerminal: {
      executable: s.customTerminal.executable,
      args: [...(s.customTerminal.args ?? [])],
    },
  };
}

export function mergeSettings(base: Settings, patch: SettingsPatch): Settings {
  return cloneSettings({
    ...base,
    ...patch,
    window: { ...base.window, ...patch.window },
    executables: { ...base.executables, ...patch.executables },
    customTerminal: { ...base.customTerminal, ...patch.customTerminal },
  });
}

/**
 * Application metadata and user settings. Settings writes are serialized so
 * concurrent saves (preferences dialog, theme toggle, sort toggle) never drop
 * each other's changes, and are refused when the existing settings failed to
 * load so defaults never overwrite an unreadable file.
 */
export const useAppStore = defineStore('app', () => {
  const appInfo = ref<Info | null>(null);
  const settings = ref<Settings>(createDefaultSettings());
  const settingsLoaded = ref(false);
  const saving = ref(false);
  const error = ref<string | null>(null);

  let pendingSettingsSaves = 0;
  let settingsSaveQueue: Promise<void> = Promise.resolve();

  async function initialize(): Promise<void> {
    error.value = null;
    const [infoResult, settingsResult] = await Promise.allSettled([
      backend.getAppInfo(),
      backend.getSettings(),
    ]);
    const errors: string[] = [];

    if (infoResult.status === 'fulfilled') {
      appInfo.value = infoResult.value;
    } else {
      errors.push(errorMessage(infoResult.reason));
    }

    if (settingsResult.status === 'fulfilled') {
      settings.value = cloneSettings(settingsResult.value);
      settingsLoaded.value = true;
    } else {
      settingsLoaded.value = false;
      errors.push(errorMessage(settingsResult.reason));
    }

    if (errors.length > 0) {
      error.value = errors.join('; ');
    }
  }

  function persistSettings(buildNext: () => Settings): Promise<boolean> {
    pendingSettingsSaves += 1;
    saving.value = true;

    const run = settingsSaveQueue.then(async () => {
      try {
        const saved = await backend.updateSettings(buildNext());
        settings.value = cloneSettings(saved);
        error.value = null;
        return true;
      } catch (err) {
        error.value = errorMessage(err);
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

  function refuseUnloaded(action: string): boolean {
    if (settingsLoaded.value) {
      return false;
    }
    error.value = `Cannot ${action} because existing settings failed to load.`;
    return true;
  }

  async function updateSettings(next: Settings): Promise<boolean> {
    if (refuseUnloaded('save preferences')) {
      return false;
    }
    const snapshot = cloneSettings(next);
    return persistSettings(() => snapshot);
  }

  /** Applies a partial change on top of the latest saved settings when the queued save runs. */
  async function patchSettings(
    patch: SettingsPatch,
    action = 'save preferences',
  ): Promise<boolean> {
    if (refuseUnloaded(action)) {
      return false;
    }
    return persistSettings(() => mergeSettings(settings.value, patch));
  }

  async function cycleTheme(): Promise<void> {
    if (refuseUnloaded('change theme')) {
      return;
    }
    const order: ThemeMode[] = [ThemeMode.ThemeSystem, ThemeMode.ThemeDark, ThemeMode.ThemeLight];
    await persistSettings(() => {
      const currentIndex = order.indexOf(settings.value.theme);
      const nextTheme = order[(currentIndex + 1) % order.length] ?? ThemeMode.ThemeSystem;
      return cloneSettings({ ...settings.value, theme: nextTheme });
    });
  }

  function setProjectSort(sort: ProjectSort): Promise<boolean> {
    return patchSettings({ projectSort: sort }, 'change sorting');
  }

  return {
    appInfo,
    settings,
    settingsLoaded,
    saving,
    error,
    initialize,
    updateSettings,
    patchSettings,
    cycleTheme,
    setProjectSort,
  };
});
