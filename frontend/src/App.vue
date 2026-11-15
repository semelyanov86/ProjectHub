<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { AlertTriangle, LoaderCircle } from '@lucide/vue';
import { backend, NoticeLevel } from '@/services/backend';
import type {
  DirectoryDraft,
  DropResult,
  ImportPreview,
  Notice,
  Project,
  ProjectSort,
} from '@/services/backend';
import { errorMessage } from '@/lib/errors';
import { useTheme } from '@/composables/useTheme';
import { useAppStore } from '@/stores/app';
import { useGitStore } from '@/stores/git';
import { useNotificationsStore } from '@/stores/notifications';
import { useProjectsStore } from '@/stores/projects';
import { useToolsStore } from '@/stores/tools';
import AppHeader from '@/components/AppHeader.vue';
import ConfirmDialog from '@/components/ConfirmDialog.vue';
import ImportDialog from '@/components/ImportDialog.vue';
import ProjectDialog from '@/components/ProjectDialog.vue';
import ProjectTabs from '@/components/ProjectTabs.vue';
import ProjectToolbar from '@/components/ProjectToolbar.vue';
import SettingsDialog from '@/components/SettingsDialog.vue';
import ToastHost from '@/components/ToastHost.vue';
import ProjectsView from '@/views/ProjectsView.vue';

const app = useAppStore();
const projects = useProjectsStore();
const git = useGitStore();
const tools = useToolsStore();
const notifications = useNotificationsStore();

useTheme(computed(() => app.settings.theme));

const toolbar = ref<InstanceType<typeof ProjectToolbar> | null>(null);
const projectsView = ref<InstanceType<typeof ProjectsView> | null>(null);

const settingsOpen = ref(false);
const projectDialog = ref<{
  mode: 'add' | 'edit';
  project: Project | null;
  draft: DirectoryDraft | null;
} | null>(null);
const importPreview = ref<ImportPreview | null>(null);
const removeTarget = ref<Project | null>(null);

const anyDialogOpen = computed(
  () =>
    settingsOpen.value ||
    projectDialog.value !== null ||
    importPreview.value !== null ||
    removeTarget.value !== null,
);

const unsubscribers: Array<() => void> = [];

async function initialize() {
  await Promise.all([app.initialize(), projects.load()]);
  void tools.refresh();
  void git.refreshAll();
}

function openAdd(draft: DirectoryDraft | null = null) {
  projectDialog.value = { mode: 'add', project: null, draft };
}

async function addProject() {
  // Open the native folder picker first, like JetBrains Toolbox; the dialog
  // then shows the suggested name and Git state of the chosen directory.
  try {
    const draft = await backend.chooseDirectory();
    if (!draft.cancelled) {
      openAdd(draft);
    }
  } catch (err) {
    notifications.error('Could not open folder picker', errorMessage(err));
  }
}

function openEdit(id: string) {
  const project = projects.byId(id);
  if (project) {
    projectDialog.value = { mode: 'edit', project, draft: null };
  }
}

function onProjectSaved(project: Project) {
  void git.refreshProjects([project.id]);
}

function requestRemove(id: string) {
  removeTarget.value = projects.byId(id) ?? null;
}

async function confirmRemove() {
  const project = removeTarget.value;
  if (project && (await projects.remove(project.id))) {
    git.forget(project.id);
  }
}

async function startImport() {
  try {
    const preview = await backend.prepareImport();
    if (!preview.cancelled) {
      importPreview.value = preview;
    }
  } catch (err) {
    notifications.error('Import failed', errorMessage(err));
  }
}

async function onImported() {
  await projects.load();
  void git.refreshAll();
}

async function exportProjects() {
  try {
    const result = await backend.exportProjects();
    if (!result.cancelled) {
      notifications.success(`Exported ${result.count} projects`, result.path);
    }
  } catch (err) {
    notifications.error('Export failed', errorMessage(err));
  }
}

async function refreshAll() {
  await projects.load();
  void tools.refresh();
  await git.refreshAll();
}

function setSort(sort: ProjectSort) {
  void app.setProjectSort(sort);
}

async function onDrop(result: DropResult) {
  if (result.error) {
    notifications.error('Could not add dropped folder', result.error);
    return;
  }
  if (result.draft) {
    if (!anyDialogOpen.value) {
      openAdd(result.draft);
    }
    return;
  }
  if (result.report) {
    const added = result.report.added ?? [];
    const skipped = result.report.skipped ?? [];
    await projects.load();
    void git.refreshProjects(added.map((p) => p.id));
    const detail = skipped.map((s) => `${s.name || s.path}: ${s.reason}`).join('\n');
    if (added.length > 0) {
      notifications.success(
        `Added ${added.length} projects`,
        skipped.length ? `Skipped ${skipped.length}:\n${detail}` : undefined,
      );
    } else {
      notifications.warning('No folders were added', detail || undefined);
    }
  }
}

/** Tray "Settings…": the window is already shown by Go; open Preferences unless busy. */
function onOpenSettings() {
  if (!anyDialogOpen.value) {
    settingsOpen.value = true;
  }
}

function onNotice(notice: Notice) {
  const tone =
    notice.level === NoticeLevel.NoticeError
      ? 'error'
      : notice.level === NoticeLevel.NoticeWarning
        ? 'warning'
        : 'success';
  notifications.notify({ tone, title: notice.title, detail: notice.detail || undefined });
}

function onWindowFocus() {
  void git.refreshIfStale();
  if (Date.now() - tools.refreshedAt > 60_000) {
    void tools.refresh();
  }
}

function onGlobalKeydown(event: KeyboardEvent) {
  if (anyDialogOpen.value) {
    return;
  }
  if ((event.ctrlKey || event.metaKey) && !event.altKey && event.key.toLowerCase() === 'f') {
    event.preventDefault();
    toolbar.value?.focusSearch();
  }
}

onMounted(() => {
  unsubscribers.push(backend.onProjectsDropped((result) => void onDrop(result)));
  unsubscribers.push(backend.onWindowFocus(onWindowFocus));
  unsubscribers.push(backend.onOpenSettings(onOpenSettings));
  unsubscribers.push(backend.onNotice(onNotice));
  unsubscribers.push(backend.onProjectsChanged(() => void projects.load()));
  window.addEventListener('keydown', onGlobalKeydown);
  void initialize();
});

onBeforeUnmount(() => {
  unsubscribers.forEach((unsubscribe) => unsubscribe());
  window.removeEventListener('keydown', onGlobalKeydown);
});
</script>

<template>
  <div
    data-file-drop-target
    class="app-shell relative flex h-screen min-w-[360px] flex-col bg-[var(--bg-app)] text-[var(--text-primary)]"
  >
    <div class="border-b border-[var(--border-subtle)] bg-[var(--bg-surface)]">
      <AppHeader
        :theme="app.settings.theme"
        :project-count="projects.projects.length"
        :theme-disabled="app.saving || !app.settingsLoaded"
        @cycle-theme="app.cycleTheme"
        @open-settings="settingsOpen = true"
      />
      <div class="space-y-2 px-3.5 pb-2.5">
        <ProjectToolbar
          ref="toolbar"
          v-model:search-query="projects.searchQuery"
          :sort="app.settings.projectSort"
          :sort-disabled="!app.settingsLoaded"
          :refreshing="git.refreshing"
          @update:sort="setSort"
          @add="addProject"
          @refresh="refreshAll"
          @import="startImport"
          @export="exportProjects"
          @search-arrow-down="projectsView?.focusList()"
        />
        <ProjectTabs v-model="projects.tab" :counts="projects.counts" />
      </div>
    </div>

    <div
      v-if="app.error"
      role="alert"
      class="mx-3 mt-2 flex items-start gap-2 rounded-lg border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-2 text-xs text-[var(--danger)]"
    >
      <AlertTriangle class="mt-0.5 h-3.5 w-3.5 shrink-0" />
      <span class="break-words">{{ app.error }}</span>
    </div>

    <main class="flex min-h-0 flex-1 flex-col">
      <ProjectsView
        ref="projectsView"
        :suspended="anyDialogOpen"
        @add="addProject"
        @edit="openEdit"
        @remove="requestRemove"
        @clear-search="projects.searchQuery = ''"
      />
    </main>

    <footer
      class="flex items-center justify-between gap-2 border-t border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3.5 py-1.5 text-[11px] text-[var(--text-muted)]"
    >
      <span class="inline-flex min-w-0 items-center gap-1 truncate">
        <template v-if="git.refreshing">
          <LoaderCircle class="h-3 w-3 animate-spin" aria-hidden="true" />
          Checking Git status…
        </template>
        <template v-else-if="git.gitUnavailable">Git not found — branch info unavailable</template>
        <template v-else
          >{{ projects.visible.length }} of {{ projects.projects.length }} shown</template
        >
      </span>
      <span>v{{ app.appInfo?.version ?? '' }}</span>
    </footer>

    <ProjectDialog
      :open="projectDialog !== null"
      :mode="projectDialog?.mode ?? 'add'"
      :project="projectDialog?.project ?? null"
      :draft="projectDialog?.draft ?? null"
      @saved="onProjectSaved"
      @close="projectDialog = null"
    />
    <SettingsDialog :open="settingsOpen" @close="settingsOpen = false" />
    <ImportDialog
      :open="importPreview !== null"
      :preview="importPreview"
      @imported="onImported"
      @close="importPreview = null"
    />
    <ConfirmDialog
      :open="removeTarget !== null"
      title="Remove project?"
      :message="`“${removeTarget?.name ?? ''}” will be removed from ProjectHub. The directory ${removeTarget?.path ?? ''} and its files are not touched.`"
      confirm-label="Remove"
      :on-confirm="confirmRemove"
      @close="removeTarget = null"
    />
    <ToastHost />
  </div>
</template>
