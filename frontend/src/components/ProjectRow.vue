<script setup lang="ts">
import { computed } from 'vue';
import { Ellipsis, FolderX, GitBranch, Star, TriangleAlert } from '@lucide/vue';
import { GitState } from '@/services/backend';
import type { Project, ProjectInspection } from '@/services/backend';
import { avatarHue, initials, relativeTime } from '@/lib/projects';

interface Props {
  project: Project;
  inspection?: ProjectInspection;
  selected: boolean;
  focusable: boolean;
  missing: boolean;
  showGitDirty: boolean;
  homeDir: string;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  activate: [event: MouseEvent];
  openDefault: [];
  contextMenu: [event: MouseEvent];
  menuButton: [event: MouseEvent];
  toggleFavorite: [];
}>();

const displayPath = computed(() => {
  const home = props.homeDir.replace(/\/+$/u, '');
  const path = props.project.path;
  if (home && (path === home || path.startsWith(`${home}/`))) {
    return `~${path.slice(home.length)}`;
  }
  return path;
});

const repo = computed(() => {
  const git = props.inspection?.git;
  return git?.state === GitState.StateRepo ? git.repo : null;
});

const gitError = computed(() =>
  props.inspection?.git.state === GitState.StateError
    ? props.inspection.git.message || 'Git status could not be read'
    : '',
);

const dirtyUncertainMessage =
  'Changes may be inaccurate: repository filters were disabled for safety';

const branchLabel = computed(() => {
  if (!repo.value) {
    return '';
  }
  if (repo.value.detached) {
    return repo.value.headSha.slice(0, 7) || 'detached';
  }
  return repo.value.branch;
});

const branchTitle = computed(() => {
  if (!repo.value) {
    return '';
  }
  const parts = [
    repo.value.detached
      ? `Detached HEAD at ${repo.value.headSha.slice(0, 12)}`
      : `Branch ${repo.value.branch}`,
  ];
  if (repo.value.unborn) {
    parts.push('no commits yet');
  }
  if (repo.value.ahead || repo.value.behind) {
    parts.push(`↑${repo.value.ahead} ↓${repo.value.behind}`);
  }
  if (repo.value.isDirty) {
    parts.push(
      `uncommitted changes${repo.value.dirtyUncertain ? ' (may be inaccurate: repository filters were disabled for safety)' : ''}`,
    );
  }
  return parts.join(' · ');
});

const avatarStyle = computed(() => ({
  backgroundColor: `hsl(${avatarHue(props.project.id)} 45% 42%)`,
}));

const lastOpenedTitle = computed(() =>
  props.project.lastOpened
    ? `Last opened ${relativeTime(props.project.lastOpened)}`
    : 'Never opened',
);
</script>

<template>
  <div
    :id="`project-${project.id}`"
    role="option"
    :aria-selected="selected"
    :tabindex="focusable ? 0 : -1"
    :data-project-id="project.id"
    class="project-row group relative flex cursor-pointer items-center gap-2.5 rounded-lg border border-transparent px-2.5 py-2 transition-colors outline-none hover:bg-[var(--bg-hover)] focus-visible:border-[var(--accent)]"
    :class="selected ? 'bg-[var(--accent-soft)] hover:bg-[var(--accent-soft)]' : ''"
    @click="emit('activate', $event)"
    @dblclick="emit('openDefault')"
    @contextmenu.prevent="emit('contextMenu', $event)"
  >
    <div
      class="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-[11px] font-semibold tracking-wide text-white"
      :class="missing ? 'opacity-50 grayscale' : ''"
      :style="avatarStyle"
      aria-hidden="true"
    >
      {{ initials(project.name) }}
    </div>

    <div class="min-w-0 flex-1" :title="lastOpenedTitle">
      <div class="flex min-w-0 items-center gap-1.5">
        <span
          class="truncate text-[13px] font-medium"
          :class="missing ? 'text-[var(--text-muted)]' : 'text-[var(--text-primary)]'"
        >
          {{ project.name }}
        </span>
        <Star
          v-if="project.favorite"
          class="h-3 w-3 shrink-0 fill-amber-400 text-amber-400"
          aria-label="Favorite"
        />
        <span
          v-if="missing"
          class="inline-flex shrink-0 items-center gap-0.5 rounded bg-[var(--danger-soft)] px-1 text-[10px] font-medium text-[var(--danger)]"
          title="The project directory no longer exists or is not accessible"
        >
          <FolderX class="h-3 w-3" aria-hidden="true" />
          Missing
        </span>
      </div>
      <p class="truncate font-mono text-[11px] text-[var(--text-muted)]" :title="project.path">
        {{ displayPath }}
      </p>
      <p
        v-if="project.description"
        class="truncate text-[11px] text-[var(--text-secondary)]"
        :title="project.description"
      >
        {{ project.description }}
      </p>
    </div>

    <div class="flex shrink-0 flex-col items-end gap-1">
      <span
        v-if="repo"
        class="inline-flex max-w-[120px] items-center gap-1 rounded-md bg-[var(--bg-elevated)] px-1.5 py-0.5 font-mono text-[10px] text-[var(--text-secondary)]"
        :title="branchTitle"
        data-testid="git-branch"
      >
        <GitBranch class="h-3 w-3 shrink-0" aria-hidden="true" />
        <span class="truncate">{{ branchLabel }}</span>
        <span
          v-if="showGitDirty && repo.isDirty && repo.dirtyUncertain"
          class="h-1.5 w-1.5 shrink-0 rounded-full border border-[var(--warning)] bg-transparent"
          :aria-label="dirtyUncertainMessage"
          :title="dirtyUncertainMessage"
          data-testid="git-dirty-uncertain"
        />
        <span
          v-else-if="showGitDirty && repo.isDirty"
          class="h-1.5 w-1.5 shrink-0 rounded-full bg-[var(--warning)]"
          aria-label="Uncommitted changes"
          data-testid="git-dirty"
        />
      </span>
      <span
        v-else-if="gitError"
        class="inline-flex items-center gap-1 rounded-md bg-[var(--warning-soft)] px-1.5 py-0.5 text-[10px] text-[var(--warning)]"
        :title="gitError"
        :aria-label="`Git status error: ${gitError}`"
        data-testid="git-error"
      >
        <TriangleAlert class="h-3 w-3 shrink-0" aria-hidden="true" />
        Git error
      </span>
      <div
        class="flex items-center gap-0.5 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100"
        :class="selected ? 'opacity-100' : ''"
      >
        <button
          type="button"
          tabindex="-1"
          class="cursor-pointer rounded p-0.5 text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-amber-400"
          :aria-label="project.favorite ? 'Remove from favorites' : 'Add to favorites'"
          :title="project.favorite ? 'Remove from favorites' : 'Add to favorites'"
          @click.stop="emit('toggleFavorite')"
          @dblclick.stop
        >
          <Star
            class="h-3.5 w-3.5"
            :class="project.favorite ? 'fill-amber-400 text-amber-400' : ''"
          />
        </button>
        <button
          type="button"
          tabindex="-1"
          class="cursor-pointer rounded p-0.5 text-[var(--text-muted)] hover:bg-[var(--bg-elevated)] hover:text-[var(--text-primary)]"
          aria-label="More actions"
          title="More actions"
          @click.stop="emit('menuButton', $event)"
          @dblclick.stop
        >
          <Ellipsis class="h-3.5 w-3.5" />
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.project-row {
  content-visibility: auto;
  contain-intrinsic-size: auto 56px;
}
</style>
