<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue';
import AppDialog from '@/components/ui/AppDialog.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppBadge from '@/components/ui/AppBadge.vue';
import AppInput from '@/components/ui/AppInput.vue';
import {
  backend,
  CloseBehavior,
  FileManagerMode,
  Target,
  TerminalMode,
  ThemeMode,
} from '@/services/backend';
import type { Settings } from '@/services/backend';
import { cloneSettings, useAppStore } from '@/stores/app';
import type { SettingsPatch } from '@/stores/app';
import { useToolsStore } from '@/stores/tools';
import { useNotificationsStore } from '@/stores/notifications';
import { EDITORS } from '@/lib/editors';
import { errorMessage } from '@/lib/errors';
import { abbreviateHome } from '@/lib/projects';

const props = defineProps<{ open: boolean }>();
const emit = defineEmits<{ close: [] }>();
const app = useAppStore();
const tools = useToolsStore();
const notifications = useNotificationsStore();
const id = useId();
const draft = ref(cloneSettings(app.settings));
let baseline = cloneSettings(app.settings);
const argumentsText = ref('');
const selectedPreset = ref('');
const saveFailed = ref(false);
const configDirLabel = computed(() =>
  app.appInfo ? abbreviateHome(app.appInfo.configDir, app.appInfo.homeDir) : '~/.config/projecthub',
);
/** null while unknown (not yet checked or the check failed). */
const trayAvailable = ref<boolean | null>(null);

const sections = [
  'Appearance',
  'Editors',
  'Terminal',
  'Applications',
  'File manager',
  'Git',
  'Window',
];
type ExecutableField = keyof Settings['executables'] | 'customTerminal';

const toolFields: readonly {
  key: ExecutableField;
  section: string;
  label: string;
  placeholder: string;
  target: Target;
}[] = [
  {
    key: 'ghostty',
    section: 'Terminal',
    label: 'Ghostty executable',
    placeholder: 'Auto-detect (ghostty)',
    target: Target.TargetGhostty,
  },
  {
    key: 'customTerminal',
    section: 'Terminal',
    label: 'Custom terminal executable',
    placeholder: 'kitty or /usr/bin/kitty',
    target: Target.TargetTerminal,
  },
  {
    key: 'sublime',
    section: 'Applications',
    label: 'Sublime Text path',
    placeholder: 'Auto-detect',
    target: Target.TargetSublime,
  },
  {
    key: 'zed',
    section: 'Applications',
    label: 'Zed path',
    placeholder: 'Auto-detect',
    target: Target.TargetZed,
  },
  {
    key: 'fileManager',
    section: 'File manager',
    label: 'File manager executable',
    placeholder: 'Executable path',
    target: Target.TargetFileManager,
  },
];

function executableValue(settings: Settings, field: ExecutableField): string {
  return field === 'customTerminal'
    ? settings.customTerminal.executable
    : settings.executables[field];
}

function setExecutable(settings: Settings, field: ExecutableField, value: string) {
  if (field === 'customTerminal') settings.customTerminal.executable = value;
  else settings.executables[field] = value;
}

const fields = computed(() =>
  toolFields.map((field) => {
    const changed =
      executableValue(draft.value, field.key) !== executableValue(app.settings, field.key) ||
      (field.key === 'customTerminal' &&
        JSON.stringify(draft.value.customTerminal.args ?? []) !==
          JSON.stringify(app.settings.customTerminal.args ?? []));
    const status = tools.byTarget(field.target);
    return {
      ...field,
      changed,
      status,
      visible:
        (field.key !== 'customTerminal' ||
          draft.value.preferredTerminal === TerminalMode.TerminalModeCustom) &&
        (field.key !== 'fileManager' ||
          draft.value.preferredFileManager === FileManagerMode.FileManagerModeCustom),
    };
  }),
);

const terminalInvalid = computed(
  () =>
    draft.value.preferredTerminal === TerminalMode.TerminalModeCustom &&
    !draft.value.customTerminal.executable.trim(),
);
const fileManagerInvalid = computed(
  () =>
    draft.value.preferredFileManager === FileManagerMode.FileManagerModeCustom &&
    !draft.value.executables.fileManager.trim(),
);
const saveDisabled = computed(
  () => !app.settingsLoaded || app.saving || terminalInvalid.value || fileManagerInvalid.value,
);

function resetDraft() {
  baseline = cloneSettings(app.settings);
  draft.value = cloneSettings(app.settings);
  argumentsText.value = (draft.value.customTerminal.args ?? []).join('\n');
  selectedPreset.value = '';
  saveFailed.value = false;
}

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    resetDraft();
    void tools.loadPresets();
    void tools.refresh();
    void checkTray();
  },
  { immediate: true },
);

watch(
  () => app.settingsLoaded,
  (loaded) => {
    if (loaded && props.open) resetDraft();
  },
);

async function checkTray() {
  try {
    trayAvailable.value = await backend.trayAvailable();
  } catch {
    trayAvailable.value = null;
  }
}

function choosePreset() {
  const preset = (tools.presets ?? []).find((item) => item.id === selectedPreset.value);
  if (!preset) return;
  draft.value.customTerminal.executable = preset.executable;
  draft.value.customTerminal.args = [...(preset.args ?? [])];
  argumentsText.value = draft.value.customTerminal.args.join('\n');
}

function updateArguments(event: Event) {
  argumentsText.value = (event.target as HTMLTextAreaElement).value;
  draft.value.customTerminal.args = argumentsText.value
    .split('\n')
    .map((arg) => arg.replace(/\r$/u, ''))
    .filter((arg) => arg.length > 0);
}

async function browse(field: ExecutableField) {
  const currentDraft = draft.value;
  try {
    const path = await backend.chooseExecutable();
    if (path) setExecutable(currentDraft, field, path);
  } catch (err) {
    notifications.error('Could not open file picker', errorMessage(err));
  }
}

function changedFields<T extends object>(next: T, previous: T): Partial<T> {
  const patch: Partial<T> = {};
  for (const key of Object.keys(next) as (keyof T)[]) {
    if (next[key] !== previous[key]) patch[key] = next[key];
  }
  return patch;
}

function draftPatch(next: Settings): SettingsPatch {
  const { window, executables, customTerminal, ...scalars } = next;
  const {
    window: previousWindow,
    executables: previousExecutables,
    customTerminal: previousTerminal,
    ...previousScalars
  } = baseline;
  const patch: SettingsPatch = changedFields(scalars, previousScalars);
  const windowPatch = changedFields(window, previousWindow);
  if (Object.keys(windowPatch).length > 0) patch.window = windowPatch;
  const executablePatch = changedFields(executables, previousExecutables);
  if (Object.keys(executablePatch).length > 0) patch.executables = executablePatch;
  const terminalPatch: SettingsPatch['customTerminal'] = {};
  if (customTerminal.executable !== previousTerminal.executable) {
    terminalPatch.executable = customTerminal.executable;
  }
  const args = customTerminal.args ?? [];
  const previousArgs = previousTerminal.args ?? [];
  if (
    args.length !== previousArgs.length ||
    args.some((arg, index) => arg !== previousArgs[index])
  ) {
    terminalPatch.args = [...args];
  }
  if (Object.keys(terminalPatch).length > 0) patch.customTerminal = terminalPatch;
  return patch;
}

async function save() {
  if (saveDisabled.value) return;
  saveFailed.value = false;
  const next = cloneSettings(draft.value);
  for (const field of toolFields) {
    setExecutable(next, field.key, executableValue(next, field.key).trim());
  }
  const patch = draftPatch(next);
  if (Object.keys(patch).length > 0) {
    const saved = await app.patchSettings(patch);
    if (!saved) {
      saveFailed.value = true;
      return;
    }
    void tools.refresh();
  }
  notifications.success('Preferences saved');
  emit('close');
}
</script>

<template>
  <AppDialog :open="open" title="Preferences" size="md" :busy="app.saving" @close="emit('close')">
    <form :id="`${id}-form`" class="space-y-3" @submit.prevent="save">
      <p
        v-if="!app.settingsLoaded"
        role="alert"
        class="rounded-lg bg-[var(--warning-soft)] p-2 text-xs text-[var(--warning)]"
      >
        Preferences could not be loaded, so they cannot be saved. Fix or remove settings.json and
        restart.
      </p>
      <p
        v-if="saveFailed && app.error"
        role="alert"
        class="rounded-lg bg-[var(--danger-soft)] p-2 text-xs text-[var(--danger)]"
      >
        {{ app.error }}
      </p>

      <section v-for="section in sections" :key="section" class="space-y-2" :aria-label="section">
        <h3 class="text-[11px] font-semibold tracking-wide text-[var(--text-muted)] uppercase">
          {{ section }}
        </h3>

        <div v-if="section === 'Appearance'" class="space-y-1">
          <label :for="`${id}-theme`" class="block text-xs text-[var(--text-secondary)]"
            >Theme</label
          >
          <select
            :id="`${id}-theme`"
            v-model="draft.theme"
            data-autofocus
            class="w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2.5 py-1.5 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
          >
            <option :value="ThemeMode.ThemeSystem">System default</option>
            <option :value="ThemeMode.ThemeLight">Light</option>
            <option :value="ThemeMode.ThemeDark">Dark</option>
          </select>
        </div>

        <template v-if="section === 'Editors'">
          <label :for="`${id}-editor`" class="block text-xs text-[var(--text-secondary)]"
            >Default editor</label
          >
          <select
            :id="`${id}-editor`"
            v-model="draft.defaultEditor"
            class="w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2.5 py-1.5 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
          >
            <option v-for="editor in EDITORS" :key="editor.id" :value="editor.id">
              {{ editor.name
              }}{{ tools.byTarget(editor.id)?.available === false ? ' (not found)' : '' }}
            </option>
          </select>
          <label class="flex items-center gap-2 text-xs text-[var(--text-secondary)]"
            ><input
              v-model="draft.rememberLastEditor"
              type="checkbox"
              class="accent-[var(--accent)]"
            />Remember last used editor per project</label
          >
          <label class="flex items-center gap-2 text-xs text-[var(--text-secondary)]"
            ><input
              v-model="draft.doubleClickOpensEditor"
              type="checkbox"
              class="accent-[var(--accent)]"
            />Double-click opens the default editor</label
          >
        </template>

        <template v-if="section === 'Terminal'">
          <fieldset class="space-y-1.5 text-xs text-[var(--text-secondary)]">
            <legend class="mb-1">Preferred terminal</legend>
            <label class="flex items-center gap-2"
              ><input
                v-model="draft.preferredTerminal"
                :name="`${id}-terminal`"
                type="radio"
                :value="TerminalMode.TerminalModeGhostty"
                class="accent-[var(--accent)]"
              />Ghostty</label
            >
            <label class="flex items-center gap-2"
              ><input
                v-model="draft.preferredTerminal"
                :name="`${id}-terminal`"
                type="radio"
                :value="TerminalMode.TerminalModeCustom"
                class="accent-[var(--accent)]"
              />Custom terminal</label
            >
          </fieldset>
          <div v-if="draft.preferredTerminal === TerminalMode.TerminalModeCustom" class="space-y-1">
            <label :for="`${id}-preset`" class="block text-xs text-[var(--text-secondary)]"
              >Terminal preset</label
            >
            <select
              :id="`${id}-preset`"
              v-model="selectedPreset"
              class="w-full rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2.5 py-1.5 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
              @change="choosePreset"
            >
              <option value="">Choose a preset…</option>
              <option v-for="preset in tools.presets ?? []" :key="preset.id" :value="preset.id">
                {{ preset.name }}
              </option>
            </select>
          </div>
        </template>

        <fieldset
          v-if="section === 'File manager'"
          class="space-y-1.5 text-xs text-[var(--text-secondary)]"
        >
          <legend class="sr-only">Preferred file manager</legend>
          <label class="flex items-center gap-2"
            ><input
              v-model="draft.preferredFileManager"
              :name="`${id}-file-manager`"
              type="radio"
              :value="FileManagerMode.FileManagerModeAuto"
              class="accent-[var(--accent)]"
            />System default (gio / xdg-open)</label
          >
          <label class="flex items-center gap-2"
            ><input
              v-model="draft.preferredFileManager"
              :name="`${id}-file-manager`"
              type="radio"
              :value="FileManagerMode.FileManagerModeCustom"
              class="accent-[var(--accent)]"
            />Custom</label
          >
        </fieldset>

        <template v-for="field in fields" :key="field.key">
          <div v-if="field.section === section && field.visible" class="space-y-1">
            <div class="flex items-end gap-2">
              <label class="min-w-0 flex-1 space-y-1">
                <span class="block text-xs text-[var(--text-secondary)]">{{ field.label }}</span>
                <AppInput
                  :model-value="executableValue(draft, field.key)"
                  :aria-label="field.label"
                  :placeholder="field.placeholder"
                  @update:model-value="setExecutable(draft, field.key, $event)"
                />
              </label>
              <AppButton
                size="sm"
                :aria-label="`Browse ${field.label}`"
                :disabled="app.saving"
                @click="browse(field.key)"
                >Browse…</AppButton
              >
            </div>
            <AppBadge v-if="field.changed">Save to re-check</AppBadge>
            <p
              v-else-if="field.status"
              class="truncate text-[11px]"
              :class="field.status.available ? 'text-[var(--success)]' : 'text-[var(--text-muted)]'"
              :title="field.status.available ? field.status.executable : field.status.reason"
            >
              {{
                field.status.available ? `Found: ${field.status.executable}` : field.status.reason
              }}
            </p>
          </div>
        </template>

        <template
          v-if="
            section === 'Terminal' && draft.preferredTerminal === TerminalMode.TerminalModeCustom
          "
        >
          <p v-if="terminalInvalid" class="text-[11px] text-[var(--danger)]">
            Enter a custom terminal executable.
          </p>
          <label :for="`${id}-arguments`" class="block text-xs text-[var(--text-secondary)]"
            >Arguments (one per line)</label
          >
          <textarea
            :id="`${id}-arguments`"
            :value="argumentsText"
            rows="3"
            class="w-full resize-y rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] px-2.5 py-1.5 font-mono text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
            @input="updateArguments"
          />
          <p class="text-[11px] text-[var(--text-muted)]">
            {path} is replaced by the project directory. Arguments are passed as-is — no shell.
          </p>
        </template>
        <p
          v-if="section === 'File manager' && fileManagerInvalid"
          class="text-[11px] text-[var(--danger)]"
        >
          Enter a custom file manager executable.
        </p>

        <label
          v-if="section === 'Git'"
          class="flex items-center gap-2 text-xs text-[var(--text-secondary)]"
          ><input v-model="draft.showGitDirty" type="checkbox" class="accent-[var(--accent)]" />Show
          uncommitted-changes indicator</label
        >
        <template v-if="section === 'Window'">
          <label class="flex items-center gap-2 text-xs text-[var(--text-secondary)]"
            ><input
              v-model="draft.window.rememberSize"
              type="checkbox"
              class="accent-[var(--accent)]"
            />Remember window size</label
          >
          <label class="flex items-center gap-2 text-xs text-[var(--text-secondary)]"
            ><input
              v-model="draft.window.alwaysOnTop"
              type="checkbox"
              class="accent-[var(--accent)]"
            />Always on top</label
          >
          <fieldset class="space-y-1.5 pt-1 text-xs text-[var(--text-secondary)]">
            <legend class="mb-1">When the window is closed</legend>
            <label class="flex items-center gap-2"
              ><input
                v-model="draft.closeBehavior"
                :name="`${id}-close`"
                type="radio"
                :value="CloseBehavior.CloseBehaviorTray"
                class="accent-[var(--accent)]"
              />Keep running in the system tray</label
            >
            <label class="flex items-center gap-2"
              ><input
                v-model="draft.closeBehavior"
                :name="`${id}-close`"
                type="radio"
                :value="CloseBehavior.CloseBehaviorQuit"
                class="accent-[var(--accent)]"
              />Quit ProjectHub</label
            >
          </fieldset>
          <p
            v-if="
              trayAvailable === false && draft.closeBehavior === CloseBehavior.CloseBehaviorTray
            "
            data-testid="tray-unavailable"
            class="text-[11px] text-[var(--text-muted)]"
          >
            No system tray was detected in this session, so closing the window quits ProjectHub. On
            GNOME, tray icons need the AppIndicator extension (enabled by default on Ubuntu).
          </p>
        </template>
      </section>
    </form>
    <template #footer>
      <p
        class="mr-auto min-w-0 truncate text-[11px] text-[var(--text-muted)]"
        :title="`Saved in ${app.appInfo?.configDir ?? '~/.config/projecthub'}`"
      >
        Saved in {{ configDirLabel }}
      </p>
      <AppButton variant="secondary" size="sm" :disabled="app.saving" @click="emit('close')"
        >Cancel</AppButton
      >
      <AppButton
        type="submit"
        :form="`${id}-form`"
        variant="primary"
        size="sm"
        :disabled="saveDisabled"
        >{{ app.saving ? 'Saving…' : 'Save' }}</AppButton
      >
    </template>
  </AppDialog>
</template>
