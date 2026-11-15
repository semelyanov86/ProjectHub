<script setup lang="ts">
import { computed, reactive, ref, useId, watch } from 'vue';
import { GitBranch } from '@lucide/vue';
import AppBadge from '@/components/ui/AppBadge.vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppDialog from '@/components/ui/AppDialog.vue';
import { EDITORS, editorName } from '@/lib/editors';
import { errorMessage } from '@/lib/errors';
import { backend, GitState } from '@/services/backend';
import type { DirectoryDraft, Project, ProjectInput } from '@/services/backend';
import { useAppStore } from '@/stores/app';
import { useProjectsStore } from '@/stores/projects';

const props = defineProps<{
  open: boolean;
  mode: 'add' | 'edit';
  project?: Project | null;
  draft?: DirectoryDraft | null;
}>();
const emit = defineEmits<{ close: []; saved: [project: Project] }>();
const app = useAppStore();
const projects = useProjectsStore();
const formId = useId();
const form = reactive<ProjectInput>({
  name: '',
  description: '',
  path: '',
  defaultEditor: '',
  favorite: false,
});
const currentDraft = ref<DirectoryDraft | null>(null);
const nameTouched = ref(false);
const saving = ref(false);
const browsing = ref(false);
const error = ref<string | null>(null);
const busy = computed(() => saving.value || browsing.value);
const duplicate = computed(() => {
  const draft = currentDraft.value;
  return Boolean(
    draft?.duplicateId && (props.mode === 'add' || draft.duplicateId !== props.project?.id),
  );
});
const canSubmit = computed(
  () =>
    !busy.value &&
    Boolean(form.name.trim() && form.path.trim()) &&
    !duplicate.value &&
    (props.mode === 'add' || Boolean(props.project)),
);
const gitLabel = computed(() => {
  const git = currentDraft.value?.git;
  if (!git) return '';
  switch (git.state) {
    case GitState.StateRepo: {
      const revision = git.repo.detached ? git.repo.headSha.slice(0, 7) : git.repo.branch;
      return `Git repository${revision ? ` · ${revision}` : ''}${git.repo.isDirty ? ' · uncommitted changes' : ''}`;
    }
    case GitState.StateNotRepo:
      return 'Not a Git repository';
    case GitState.StateUnavailable:
      return 'Git is not installed';
    case GitState.StateError:
      return git.message;
    default:
      return '';
  }
});

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    const project = props.mode === 'edit' ? props.project : null;
    Object.assign(form, {
      name: project?.name ?? (props.mode === 'add' ? (props.draft?.suggestedName ?? '') : ''),
      description: project?.description ?? '',
      path: project?.path ?? (props.mode === 'add' ? (props.draft?.path ?? '') : ''),
      defaultEditor: project?.defaultEditor ?? '',
      favorite: project?.favorite ?? false,
    });
    currentDraft.value = props.mode === 'add' ? (props.draft ?? null) : null;
    nameTouched.value = props.mode === 'edit';
    error.value = null;
    saving.value = false;
    browsing.value = false;
  },
  { immediate: true },
);

async function browse() {
  if (busy.value) return;
  browsing.value = true;
  error.value = null;
  try {
    const draft = await backend.chooseDirectory();
    if (draft.cancelled) return;
    form.path = draft.path;
    currentDraft.value = draft;
    if (!nameTouched.value) form.name = draft.suggestedName;
  } catch (err) {
    error.value = errorMessage(err);
  } finally {
    browsing.value = false;
  }
}

async function submit() {
  if (!canSubmit.value) return;
  const input: ProjectInput = {
    name: form.name.trim(),
    description: form.description.trim(),
    path: form.path,
    defaultEditor: form.defaultEditor,
    favorite: form.favorite,
  };
  saving.value = true;
  error.value = null;
  try {
    const result =
      props.mode === 'edit' && props.project
        ? await projects.update(props.project.id, input)
        : await projects.add(input);
    emit('saved', result);
    emit('close');
  } catch (err) {
    error.value = errorMessage(err);
  } finally {
    saving.value = false;
  }
}
</script>

<template>
  <AppDialog
    :open="open"
    :title="mode === 'add' ? 'Add Project' : 'Edit Project'"
    :description="
      mode === 'add' ? 'Register an existing directory. Files are never modified.' : undefined
    "
    :busy="busy"
    @close="emit('close')"
  >
    <form :id="formId" class="flex flex-col gap-3 text-xs" @submit.prevent="submit">
      <div class="flex flex-col gap-1.5">
        <label :for="`${formId}-path`" class="font-medium">Directory</label>
        <div class="flex items-center gap-2">
          <input
            :id="`${formId}-path`"
            v-model="form.path"
            :data-autofocus="!form.path ? '' : undefined"
            :disabled="busy"
            type="text"
            autocomplete="off"
            spellcheck="false"
            class="min-w-0 flex-1 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 font-mono text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
            @input="currentDraft = null"
          />
          <AppButton size="sm" :disabled="busy" @click="browse">Browse…</AppButton>
        </div>
        <AppBadge v-if="gitLabel" class="self-start break-all">
          <GitBranch
            v-if="currentDraft?.git.state === GitState.StateRepo"
            class="h-3 w-3 shrink-0"
            aria-hidden="true"
          />
          {{ gitLabel }}
        </AppBadge>
        <p
          v-if="duplicate"
          role="alert"
          class="rounded-lg bg-[var(--warning-soft)] p-2 text-[11px] text-[var(--warning)]"
        >
          Already registered as “{{ currentDraft?.duplicateName }}”.
        </p>
      </div>
      <div class="flex flex-col gap-1.5">
        <label :for="`${formId}-name`" class="font-medium">Name</label>
        <input
          :id="`${formId}-name`"
          v-model="form.name"
          :data-autofocus="form.path ? '' : undefined"
          :disabled="busy"
          type="text"
          maxlength="200"
          class="rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
          @input="nameTouched = true"
        />
      </div>
      <div class="flex flex-col gap-1.5">
        <label :for="`${formId}-description`" class="font-medium">Description</label>
        <textarea
          :id="`${formId}-description`"
          v-model="form.description"
          :disabled="busy"
          rows="2"
          maxlength="2000"
          class="resize-y rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
        />
      </div>
      <div class="flex flex-col gap-1.5">
        <label :for="`${formId}-editor`" class="font-medium">Default editor</label>
        <select
          :id="`${formId}-editor`"
          v-model="form.defaultEditor"
          :disabled="busy"
          class="rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 text-xs text-[var(--text-primary)] focus:border-[var(--accent)] focus:outline-none"
        >
          <option value="">
            Use global default ({{ editorName(app.settings.defaultEditor) }})
          </option>
          <option v-for="editor in EDITORS" :key="editor.id" :value="editor.id">
            {{ editor.name }}
          </option>
        </select>
      </div>
      <label class="flex items-center gap-2">
        <input
          v-model="form.favorite"
          type="checkbox"
          :disabled="busy"
          class="accent-[var(--accent)]"
        />
        {{ mode === 'add' ? 'Add to favorites' : 'Favorite' }}
      </label>
      <p
        v-if="error"
        role="alert"
        class="rounded-lg bg-[var(--danger-soft)] p-2 text-xs text-[var(--danger)]"
      >
        {{ error }}
      </p>
    </form>
    <template #footer>
      <AppButton :disabled="busy" @click="emit('close')">Cancel</AppButton>
      <AppButton type="submit" :form="formId" variant="primary" :disabled="!canSubmit">{{
        mode === 'add' ? 'Add Project' : 'Save'
      }}</AppButton>
    </template>
  </AppDialog>
</template>
