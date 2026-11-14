<script setup lang="ts">
import { reactive, ref, watch } from 'vue';
import { Check } from '@lucide/vue';
import { ThemeMode } from '@/services/backend';
import type { Settings } from '@/services/backend';
import AppButton from '@/components/ui/AppButton.vue';

interface Props {
  settings: Settings;
  configDir: string;
  saving: boolean;
  canSave?: boolean;
  saveSettings: (next: Settings) => Promise<boolean>;
}

const props = withDefaults(defineProps<Props>(), {
  canSave: true,
});

const savedNotice = ref(false);

function cloneSettings(s: Settings): Settings {
  return {
    ...s,
    window: { ...s.window },
  };
}

let syncedSettings = cloneSettings(props.settings);
let syncingFromProps = false;

const draft = reactive<Settings>(cloneSettings(props.settings));

watch(
  () => props.settings,
  (next) => {
    syncingFromProps = true;
    try {
      draft.schemaVersion = next.schemaVersion;
      // Preserve fields edited locally by the user when unrelated settings (e.g. header theme) update.
      if (draft.defaultEditor === syncedSettings.defaultEditor) {
        draft.defaultEditor = next.defaultEditor;
      }
      if (draft.preferredTerminal === syncedSettings.preferredTerminal) {
        draft.preferredTerminal = next.preferredTerminal;
      }
      if (draft.preferredFileManager === syncedSettings.preferredFileManager) {
        draft.preferredFileManager = next.preferredFileManager;
      }
      if (draft.theme === syncedSettings.theme) {
        draft.theme = next.theme;
      }
      draft.window.width = next.window.width;
      draft.window.height = next.window.height;
      if (draft.window.rememberSize === syncedSettings.window.rememberSize) {
        draft.window.rememberSize = next.window.rememberSize;
      }
      if (draft.window.alwaysOnTop === syncedSettings.window.alwaysOnTop) {
        draft.window.alwaysOnTop = next.window.alwaysOnTop;
      }
      syncedSettings = cloneSettings(next);
    } finally {
      syncingFromProps = false;
    }
  },
  { deep: true, flush: 'sync' },
);

watch(
  draft,
  () => {
    if (!syncingFromProps) {
      savedNotice.value = false;
    }
  },
  { deep: true, flush: 'sync' },
);

async function handleSubmit() {
  savedNotice.value = false;
  const ok = await props.saveSettings({
    ...draft,
    window: { ...draft.window },
  });
  savedNotice.value = ok;
}
</script>

<template>
  <form
    class="space-y-3 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3.5 shadow-[var(--shadow-card)]"
    @submit.prevent="handleSubmit"
  >
    <div>
      <h2 class="text-sm font-semibold text-[var(--text-primary)]">Preferences</h2>
      <p class="mt-0.5 text-[11px] text-[var(--text-muted)]">
        Stored atomically in
        <span class="font-mono">{{ configDir || '~/.config/projecthub' }}</span>
      </p>
    </div>

    <div class="space-y-2.5">
      <label class="block space-y-1">
        <span class="text-xs font-medium text-[var(--text-secondary)]">Theme</span>
        <select
          v-model="draft.theme"
          class="w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2.5 py-1.5 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
        >
          <option :value="ThemeMode.ThemeSystem">System default</option>
          <option :value="ThemeMode.ThemeDark">Dark</option>
          <option :value="ThemeMode.ThemeLight">Light</option>
        </select>
      </label>

      <label class="block space-y-1">
        <span class="text-xs font-medium text-[var(--text-secondary)]">Default editor</span>
        <input
          v-model="draft.defaultEditor"
          type="text"
          class="w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2.5 py-1.5 font-mono text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
        />
      </label>

      <label class="block space-y-1">
        <span class="text-xs font-medium text-[var(--text-secondary)]">Preferred terminal</span>
        <input
          v-model="draft.preferredTerminal"
          type="text"
          class="w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2.5 py-1.5 font-mono text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
        />
      </label>

      <label class="block space-y-1">
        <span class="text-xs font-medium text-[var(--text-secondary)]">Preferred file manager</span>
        <input
          v-model="draft.preferredFileManager"
          type="text"
          class="w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2.5 py-1.5 font-mono text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
        />
      </label>

      <div class="space-y-1.5 pt-1">
        <label
          class="flex cursor-pointer items-center justify-between text-xs text-[var(--text-secondary)]"
        >
          <span>Remember window size</span>
          <input
            v-model="draft.window.rememberSize"
            type="checkbox"
            class="accent-[var(--accent)]"
          />
        </label>
        <label
          class="flex cursor-pointer items-center justify-between text-xs text-[var(--text-secondary)]"
        >
          <span>Always on top</span>
          <input
            v-model="draft.window.alwaysOnTop"
            type="checkbox"
            class="accent-[var(--accent)]"
          />
        </label>
      </div>
    </div>

    <div class="flex items-center justify-between border-t border-[var(--border-subtle)] pt-3">
      <span
        v-if="savedNotice && !saving"
        class="inline-flex items-center gap-1 text-[11px] text-[var(--accent)]"
      >
        <Check class="h-3.5 w-3.5" /> Saved
      </span>
      <span v-else class="text-[11px] text-[var(--text-muted)]"
        >Schema v{{ draft.schemaVersion }}</span
      >

      <AppButton type="submit" variant="primary" size="sm" :disabled="saving || !canSave">
        {{ saving ? 'Saving...' : 'Save preferences' }}
      </AppButton>
    </div>
  </form>
</template>
