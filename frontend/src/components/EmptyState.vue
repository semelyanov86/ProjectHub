<script setup lang="ts">
import { Code2, FolderKanban, FolderOpen, GitBranch, Terminal } from '@lucide/vue';
import AppBadge from './ui/AppBadge.vue';

interface Props {
  configDir: string;
}

defineProps<Props>();

const plannedCapabilities = [
  { icon: Code2, label: 'Editors', detail: 'Open workspaces in your preferred IDE or editor' },
  { icon: Terminal, label: 'Ghostty', detail: 'Launch a terminal session directly at repo root' },
  { icon: FolderOpen, label: 'Files', detail: 'Reveal project directory in your file manager' },
  { icon: GitBranch, label: 'Git', detail: 'Inspect local branch and repository status' },
];
</script>

<template>
  <section
    class="flex flex-col items-center rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-4 py-6 text-center shadow-[var(--shadow-card)]"
  >
    <div
      class="flex h-11 w-11 items-center justify-center rounded-xl bg-[var(--accent-soft)] text-[var(--accent)]"
    >
      <FolderKanban class="h-5 w-5" />
    </div>

    <h2 class="mt-3 text-sm font-semibold text-[var(--text-primary)]">
      No projects registered yet
    </h2>
    <p class="mt-1 max-w-[320px] text-xs leading-relaxed text-[var(--text-secondary)]">
      ProjectHub stores your local workspaces in XDG JSON files with zero telemetry or cloud sync.
    </p>

    <div class="mt-4 grid w-full grid-cols-2 gap-2 text-left">
      <div
        v-for="item in plannedCapabilities"
        :key="item.label"
        class="rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-elevated)] p-2.5"
      >
        <div class="flex items-center gap-1.5 text-xs font-medium text-[var(--text-primary)]">
          <component :is="item.icon" class="h-3.5 w-3.5 text-[var(--accent)]" />
          <span>{{ item.label }}</span>
        </div>
        <p class="mt-1 text-[11px] leading-snug text-[var(--text-muted)]">
          {{ item.detail }}
        </p>
      </div>
    </div>

    <div class="mt-4 flex flex-wrap items-center justify-center gap-1.5">
      <AppBadge tone="accent">Foundation Ready</AppBadge>
      <AppBadge v-if="configDir">
        <span class="max-w-[230px] truncate font-mono">{{ configDir }}</span>
      </AppBadge>
    </div>
  </section>
</template>
