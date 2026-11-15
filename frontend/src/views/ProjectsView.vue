<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import {
  Code2,
  Copy,
  FolderOpen,
  FolderSearch,
  Pencil,
  RefreshCw,
  SquareCode,
  Star,
  Terminal,
  Trash2,
} from '@lucide/vue';
import { backend, Target, TerminalMode } from '@/services/backend';
import type { Project } from '@/services/backend';
import { EDITOR_IDS, editorName } from '@/lib/editors';
import { errorMessage } from '@/lib/errors';
import { DOUBLE_CLICK_DELAY_MS, effectiveEditor } from '@/lib/projects';
import { useAppStore } from '@/stores/app';
import { useGitStore } from '@/stores/git';
import { useNotificationsStore } from '@/stores/notifications';
import { useProjectsStore } from '@/stores/projects';
import { useToolsStore } from '@/stores/tools';
import AppButton from '@/components/ui/AppButton.vue';
import AppMenu from '@/components/ui/AppMenu.vue';
import type { MenuItem, MenuPosition } from '@/components/ui/menu';
import EmptyState from '@/components/EmptyState.vue';
import ProjectRow from '@/components/ProjectRow.vue';

const props = withDefaults(defineProps<{ suspended?: boolean }>(), { suspended: false });

const emit = defineEmits<{
  add: [];
  edit: [id: string];
  remove: [id: string];
  clearSearch: [];
}>();

const app = useAppStore();
const projects = useProjectsStore();
const git = useGitStore();
const tools = useToolsStore();
const notifications = useNotificationsStore();

type MenuKind = 'open-with' | 'context';
const menu = ref<{ kind: MenuKind; projectId: string; position: MenuPosition } | null>(null);
const list = ref<HTMLElement | null>(null);
let clickTimer: ReturnType<typeof setTimeout> | undefined;
let clickedRow: HTMLElement | null = null;

const focusableId = computed(
  () =>
    projects.visible.find((project) => project.id === projects.selectedId)?.id ??
    projects.visible[0]?.id ??
    null,
);

const menuProject = computed(() => (menu.value ? projects.byId(menu.value.projectId) : undefined));

const terminalTarget = computed(() =>
  app.settings.preferredTerminal === TerminalMode.TerminalModeCustom
    ? Target.TargetTerminal
    : Target.TargetGhostty,
);

function defaultEditorOf(project: Project): string {
  return effectiveEditor(project, app.settings, EDITOR_IDS);
}

/** Disabled hint for a target, or undefined when it can be launched. */
function unavailableHint(project: Project, target: Target): string | undefined {
  if (git.isMissing(project.id)) {
    return 'Project directory not found — edit the project to fix its path.';
  }
  const tool = tools.byTarget(target);
  if (tools.loaded && tool && !tool.available) {
    return tool.reason || 'Not available';
  }
  return undefined;
}

function launchItem(
  project: Project,
  target: Target,
  label: string,
  icon: MenuItem['icon'],
): MenuItem {
  const hint = unavailableHint(project, target);
  return {
    id: `open:${target}`,
    label,
    icon,
    disabled: hint !== undefined,
    hint,
    shortcut: target === defaultEditorOf(project) ? 'Default' : undefined,
  };
}

const openWithItems = computed<MenuItem[]>(() => {
  const project = menuProject.value;
  if (!project) {
    return [];
  }
  const terminal = tools.byTarget(terminalTarget.value);
  const items = [
    launchItem(project, Target.TargetSublime, 'Sublime Text', Code2),
    launchItem(project, Target.TargetZed, 'Zed', SquareCode),
    launchItem(project, terminalTarget.value, terminal?.name ?? 'Ghostty', Terminal),
    launchItem(project, Target.TargetFileManager, 'File Manager', FolderOpen),
  ];
  if (git.isMissing(project.id)) {
    items.push({ id: 'edit', label: 'Edit project…', icon: Pencil, separatorBefore: true });
  }
  return items;
});

const contextItems = computed<MenuItem[]>(() => {
  const project = menuProject.value;
  if (!project) {
    return [];
  }
  const editorId = defaultEditorOf(project);
  const hint = unavailableHint(project, editorId as Target);
  return [
    {
      id: 'open-default',
      label: `Open in ${editorName(editorId)}`,
      icon: Code2,
      shortcut: 'Ctrl+Enter',
      disabled: hint !== undefined,
      hint,
    },
    { id: 'open-with', label: 'Open with…', icon: FolderSearch, shortcut: 'Enter' },
    { id: 'edit', label: 'Edit…', icon: Pencil, shortcut: 'F2', separatorBefore: true },
    {
      id: 'favorite',
      label: project.favorite ? 'Remove from favorites' : 'Add to favorites',
      icon: Star,
    },
    { id: 'copy-path', label: 'Copy path', icon: Copy },
    { id: 'refresh', label: 'Refresh status', icon: RefreshCw },
    {
      id: 'remove',
      label: 'Remove from ProjectHub…',
      icon: Trash2,
      danger: true,
      shortcut: 'Del',
      separatorBefore: true,
    },
  ];
});

function openMenu(kind: MenuKind, projectId: string, position: MenuPosition) {
  if (props.suspended) return;
  menu.value = { kind, projectId, position };
}

function closeMenu() {
  menu.value = null;
}

function rowElement(id: string): HTMLElement | null {
  return list.value?.querySelector<HTMLElement>(`[data-project-id="${CSS.escape(id)}"]`) ?? null;
}

function rowAnchor(id: string): MenuPosition {
  const rect = rowElement(id)?.getBoundingClientRect();
  return rect
    ? { x: rect.left + 44, y: rect.bottom + 2, avoidTop: rect.top - 2 }
    : { x: 16, y: 120 };
}

async function focusRow(id: string) {
  projects.select(id);
  await nextTick();
  const row = rowElement(id);
  row?.focus({ preventScroll: true });
  row?.scrollIntoView?.({ block: 'nearest' });
}

function onRowFocus(id: string) {
  if (projects.selectedId !== id) projects.select(id);
}

function cancelPendingClick() {
  if (clickTimer !== undefined) {
    clearTimeout(clickTimer);
    clickTimer = undefined;
  }
  document.removeEventListener('pointerdown', onPendingPointerdown, true);
  clickedRow = null;
}

function onPendingPointerdown(event: PointerEvent) {
  if (event.target instanceof Node && clickedRow?.contains(event.target)) return;
  cancelPendingClick();
}

function onActivate(project: Project, event: MouseEvent) {
  if (props.suspended) return;
  projects.select(project.id);
  if (event.detail > 1) {
    return;
  }
  const row = (event.target as HTMLElement).closest<HTMLElement>('[data-project-id]');
  if (!row) return;
  const rect = row.getBoundingClientRect();
  const position = { x: event.clientX, y: rect.bottom + 2, avoidTop: rect.top - 2 };
  cancelPendingClick();
  if (!app.settings.doubleClickOpensEditor) {
    openMenu('open-with', project.id, position);
    return;
  }
  clickedRow = rowElement(project.id);
  document.addEventListener('pointerdown', onPendingPointerdown, true);
  clickTimer = setTimeout(() => {
    cancelPendingClick();
    openMenu('open-with', project.id, position);
  }, DOUBLE_CLICK_DELAY_MS);
}

function onOpenDefault(project: Project) {
  if (!app.settings.doubleClickOpensEditor) {
    return;
  }
  cancelPendingClick();
  closeMenu();
  void projects.open(project.id);
}

function onContextMenu(project: Project, event: MouseEvent) {
  cancelPendingClick();
  projects.select(project.id);
  openMenu('context', project.id, { x: event.clientX, y: event.clientY });
}

function onMenuButton(project: Project, event: MouseEvent) {
  cancelPendingClick();
  projects.select(project.id);
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  openMenu('context', project.id, { x: rect.right - 200, y: rect.bottom + 2 });
}

function onToggleFavorite(project: Project) {
  projects.select(project.id);
  void projects.toggleFavorite(project.id);
}

async function copyPath(project: Project) {
  try {
    await backend.copyText(project.path);
    notifications.success('Path copied', project.path);
  } catch (err) {
    notifications.error('Could not copy path', errorMessage(err));
  }
}

async function onMenuSelect(id: string) {
  const current = menu.value;
  const project = menuProject.value;
  closeMenu();
  if (!current || !project) {
    return;
  }
  if (id.startsWith('open:')) {
    void projects.open(project.id, id.slice('open:'.length) as Target);
    return;
  }
  switch (id) {
    case 'open-default':
      void projects.open(project.id);
      break;
    case 'open-with':
      await nextTick();
      openMenu('open-with', project.id, { ...current.position });
      break;
    case 'edit':
      emit('edit', project.id);
      break;
    case 'favorite':
      onToggleFavorite(project);
      break;
    case 'copy-path':
      void copyPath(project);
      break;
    case 'refresh':
      void git.refreshProjects([project.id]);
      break;
    case 'remove':
      emit('remove', project.id);
      break;
  }
}

function moveSelection(event: KeyboardEvent, visible: Project[], index: number): boolean {
  let next: number;
  switch (event.key) {
    case 'ArrowDown':
      next = Math.min(visible.length - 1, index + 1);
      break;
    case 'ArrowUp':
      next = Math.max(0, index - 1);
      break;
    case 'Home':
      next = 0;
      break;
    case 'End':
      next = visible.length - 1;
      break;
    default:
      return false;
  }
  const target = visible[next];
  if (target) {
    void focusRow(target.id);
  }
  return true;
}

function onListKeydown(event: KeyboardEvent) {
  const visible = projects.visible;
  if (visible.length === 0) {
    return;
  }
  const target = event.target as HTMLElement;
  const row = target.closest<HTMLElement>('[data-project-id]');
  let index = visible.findIndex((p) => p.id === (row?.dataset.projectId ?? projects.selectedId));
  if (index < 0) {
    index = 0;
  }
  const project = visible[index];
  if (!project) {
    return;
  }
  if (projects.selectedId !== project.id) {
    projects.select(project.id);
  }
  if (moveSelection(event, visible, index)) {
    event.preventDefault();
    return;
  }
  if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
    void projects.open(project.id);
  } else if (event.key === 'Enter' || event.key === ' ') {
    if (row && target !== row && target.closest('button')) return;
    openMenu('open-with', project.id, rowAnchor(project.id));
  } else if (event.key === 'ContextMenu' || (event.key === 'F10' && event.shiftKey)) {
    openMenu('context', project.id, rowAnchor(project.id));
  } else if (event.key === 'F2') {
    emit('edit', project.id);
  } else if (event.key === 'Delete') {
    emit('remove', project.id);
  } else {
    return;
  }
  event.preventDefault();
}

/** Focuses the selected (or first) project row, e.g. on ArrowDown from the search field. */
function focusList() {
  const visible = projects.visible;
  const target = visible.find((p) => p.id === projects.selectedId) ?? visible[0];
  if (target) {
    void focusRow(target.id);
  }
}

const emptyTabMessage = computed(() => {
  if (projects.searchQuery.trim()) {
    return `No projects match “${projects.searchQuery.trim()}”.`;
  }
  if (projects.tab === 'favorites') {
    return 'No favorites yet. Use the star on a project to pin it here.';
  }
  return 'Projects you open will appear here, most recent first.';
});

watch(
  () => props.suspended,
  (suspended) => {
    if (suspended) {
      cancelPendingClick();
      closeMenu();
    }
  },
);

onBeforeUnmount(cancelPendingClick);

defineExpose({ focusList });
</script>

<template>
  <div class="flex min-h-0 flex-1 flex-col">
    <div
      v-if="projects.loadError"
      role="alert"
      class="m-3 mb-0 rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-3 text-xs text-[var(--danger)]"
    >
      <p class="font-medium">
        {{
          projects.loaded
            ? 'Projects could not be reloaded — showing the last loaded list.'
            : 'Projects could not be loaded.'
        }}
      </p>
      <p class="mt-1 break-words">{{ projects.loadError }}</p>
      <AppButton class="mt-2" size="sm" :disabled="projects.loading" @click="projects.load()">
        Retry
      </AppButton>
    </div>

    <template v-if="projects.loaded || !projects.loadError">
      <div
        v-if="projects.loading && !projects.loaded"
        class="m-3 rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-4 text-center text-xs text-[var(--text-muted)]"
      >
        Loading projects…
      </div>

      <EmptyState v-else-if="projects.projects.length === 0" @add="emit('add')" />

      <div
        v-else-if="projects.visible.length === 0"
        class="m-3 flex flex-col items-center rounded-xl border border-dashed border-[var(--border-subtle)] px-4 py-8 text-center"
      >
        <p class="text-xs text-[var(--text-secondary)]">{{ emptyTabMessage }}</p>
        <AppButton
          v-if="projects.searchQuery.trim()"
          class="mt-3"
          size="sm"
          @click="emit('clearSearch')"
        >
          Clear search
        </AppButton>
      </div>

      <div
        v-else
        ref="list"
        role="listbox"
        aria-label="Projects"
        class="min-h-0 flex-1 space-y-0.5 overflow-y-auto px-2 py-1.5"
        @keydown="onListKeydown"
      >
        <ProjectRow
          v-for="project in projects.visible"
          :key="project.id"
          :project="project"
          :inspection="git.inspections[project.id]"
          :selected="project.id === projects.selectedId"
          :focusable="project.id === focusableId"
          :missing="git.isMissing(project.id)"
          :show-git-dirty="app.settings.showGitDirty"
          :home-dir="app.appInfo?.homeDir ?? ''"
          @focusin="onRowFocus(project.id)"
          @activate="onActivate(project, $event)"
          @open-default="onOpenDefault(project)"
          @context-menu="onContextMenu(project, $event)"
          @menu-button="onMenuButton(project, $event)"
          @toggle-favorite="onToggleFavorite(project)"
        />
      </div>
    </template>

    <AppMenu
      :open="menu !== null"
      :position="menu?.position ?? { x: 0, y: 0 }"
      :items="menu?.kind === 'context' ? contextItems : openWithItems"
      :label="menu?.kind === 'context' ? 'Project actions' : 'Open with'"
      :title="menu?.kind === 'context' ? menuProject?.name : 'Open with'"
      @select="onMenuSelect"
      @close="closeMenu"
    />
  </div>
</template>
