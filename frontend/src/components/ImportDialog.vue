<script setup lang="ts">
import { computed, ref, useId, watch } from 'vue';
import AppButton from '@/components/ui/AppButton.vue';
import AppDialog from '@/components/ui/AppDialog.vue';
import { errorMessage } from '@/lib/errors';
import { backend, ImportMode } from '@/services/backend';
import type { ImportPreview, ImportReport } from '@/services/backend';

const props = defineProps<{ open: boolean; preview: ImportPreview | null }>();
const emit = defineEmits<{ close: []; imported: [report: ImportReport] }>();
const formId = useId();
const mode = ref(ImportMode.ImportMerge);
const confirmed = ref(false);
const busy = ref(false);
const error = ref<string | null>(null);
const report = ref<ImportReport | null>(null);
const skipped = computed(() => props.preview?.skipped ?? []);
const resultSkipped = computed(() => report.value?.skipped ?? []);
const canApply = computed(
  () =>
    !busy.value &&
    !report.value &&
    Boolean(props.preview) &&
    (mode.value === ImportMode.ImportReplace ? confirmed.value : (props.preview?.new ?? 0) > 0),
);

watch(
  () => props.open,
  (open) => {
    if (!open) return;
    mode.value = ImportMode.ImportMerge;
    confirmed.value = false;
    busy.value = false;
    error.value = null;
    report.value = null;
  },
  { immediate: true },
);
watch(mode, () => {
  confirmed.value = false;
});

async function apply() {
  if (!canApply.value || !props.preview) return;
  busy.value = true;
  error.value = null;
  try {
    const result = await backend.applyImport({
      token: props.preview.token,
      mode: mode.value,
      replaceConfirmed: mode.value === ImportMode.ImportReplace && confirmed.value,
    });
    report.value = result;
    emit('imported', result);
  } catch (err) {
    error.value = errorMessage(err);
  } finally {
    busy.value = false;
  }
}

async function close() {
  if (busy.value) return;
  busy.value = true;
  try {
    if (!report.value && props.preview) {
      await backend.discardImport(props.preview.token);
    }
  } catch {
    // Discarding a preview must not prevent closing the dialog.
  } finally {
    emit('close');
    busy.value = false;
  }
}
</script>

<template>
  <AppDialog
    :open="open"
    title="Import Projects"
    :description="preview ? `From ${preview.fileName}` : undefined"
    :busy="busy"
    @close="close"
  >
    <div v-if="report" class="flex flex-col gap-3 text-xs">
      <p role="status" class="font-medium text-[var(--success)]">
        Imported {{ report.added }} projects
      </p>
      <p v-if="report.backupPath" class="text-[11px] text-[var(--text-secondary)]">
        Previous list backed up to
        <span class="font-mono break-all select-text">{{ report.backupPath }}</span>
      </p>
      <details v-if="resultSkipped.length" class="text-[11px] text-[var(--text-secondary)]">
        <summary class="cursor-pointer">{{ resultSkipped.length }} skipped</summary>
        <ul class="mt-2 flex max-h-32 flex-col gap-2 overflow-y-auto">
          <li v-for="(item, index) in resultSkipped" :key="`${item.path}-${index}`">
            <p class="font-medium text-[var(--text-primary)]">{{ item.name }}</p>
            <p class="font-mono break-all select-text">{{ item.path }}</p>
            <p>{{ item.reason }}</p>
          </li>
        </ul>
      </details>
    </div>
    <form
      v-else-if="preview"
      :id="formId"
      class="flex flex-col gap-4 text-xs"
      @submit.prevent="apply"
    >
      <div class="flex flex-col gap-1.5 text-[var(--text-secondary)]">
        <ul class="flex flex-col gap-1.5">
          <li>{{ preview.total }} projects in file</li>
          <li>{{ preview.new }} new</li>
          <li>{{ preview.duplicates }} already registered</li>
        </ul>
        <details class="text-[11px]">
          <summary class="cursor-pointer">
            {{ skipped.length }} skipped — directory not found on this computer
          </summary>
          <ul v-if="skipped.length" class="mt-2 flex max-h-32 flex-col gap-2 overflow-y-auto">
            <li v-for="(item, index) in skipped" :key="`${item.path}-${index}`">
              <p class="font-medium text-[var(--text-primary)]">{{ item.name }}</p>
              <p class="font-mono break-all select-text">{{ item.path }}</p>
              <p>{{ item.reason }}</p>
            </li>
          </ul>
        </details>
      </div>
      <fieldset :disabled="busy" class="flex flex-col gap-2">
        <legend class="mb-2 font-medium">Import mode</legend>
        <label class="flex items-start gap-2">
          <input
            v-model="mode"
            :name="`${formId}-mode`"
            type="radio"
            :value="ImportMode.ImportMerge"
            class="mt-0.5 accent-[var(--accent)]"
          />
          <span>Merge — add {{ preview.new }} new projects, keep existing ones</span>
        </label>
        <label class="flex items-start gap-2">
          <input
            v-model="mode"
            :name="`${formId}-mode`"
            type="radio"
            :value="ImportMode.ImportReplace"
            class="mt-0.5 accent-[var(--accent)]"
          />
          <span
            >Replace — replace all {{ preview.current }} current projects with
            {{ preview.eligible }} imported ones</span
          >
        </label>
      </fieldset>
      <div
        v-if="mode === ImportMode.ImportReplace"
        class="flex flex-col gap-2 rounded-lg bg-[var(--warning-soft)] p-2.5 text-[var(--warning)]"
      >
        <p>
          Your current project list will be replaced. A backup copy of projects.json is saved first.
        </p>
        <label class="flex items-start gap-2">
          <input
            v-model="confirmed"
            type="checkbox"
            :disabled="busy"
            class="mt-0.5 accent-[var(--accent)]"
          />
          <span>I understand that all {{ preview.current }} current projects will be replaced</span>
        </label>
      </div>
      <p
        v-if="mode === ImportMode.ImportMerge && preview.new === 0"
        class="text-[11px] text-[var(--text-muted)]"
      >
        Nothing new to import
      </p>
      <p
        v-if="error"
        role="alert"
        class="rounded-lg bg-[var(--danger-soft)] p-2 text-xs text-[var(--danger)]"
      >
        {{ error }}
      </p>
    </form>
    <template #footer>
      <AppButton v-if="report" variant="primary" :disabled="busy" @click="close">Done</AppButton>
      <template v-else>
        <AppButton :disabled="busy" @click="close">Cancel</AppButton>
        <AppButton type="submit" :form="formId" variant="primary" :disabled="!canApply">{{
          mode === ImportMode.ImportReplace ? 'Replace projects' : 'Import'
        }}</AppButton>
      </template>
    </template>
  </AppDialog>
</template>
