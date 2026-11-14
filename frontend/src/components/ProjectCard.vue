<script setup lang="ts">
import { Star } from '@lucide/vue';
import type { Project } from '@/services/backend';
import AppBadge from './ui/AppBadge.vue';

interface Props {
  project: Project;
}

defineProps<Props>();
</script>

<template>
  <article
    class="group rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-3 shadow-[var(--shadow-card)] transition-colors hover:border-[var(--border-strong)] hover:bg-[var(--bg-elevated)]"
  >
    <div class="flex items-start justify-between gap-2">
      <div class="min-w-0 flex-1">
        <div class="flex items-center gap-1.5">
          <h2 class="truncate text-[13px] font-semibold text-[var(--text-primary)]">
            {{ project.name }}
          </h2>
          <Star
            v-if="project.favorite"
            class="h-3.5 w-3.5 shrink-0 fill-amber-400 text-amber-400"
            aria-label="Favorite project"
          />
        </div>
        <p
          v-if="project.description"
          class="mt-0.5 line-clamp-1 text-xs text-[var(--text-secondary)]"
        >
          {{ project.description }}
        </p>
      </div>

      <AppBadge v-if="project.defaultEditor" tone="accent">
        {{ project.defaultEditor }}
      </AppBadge>
    </div>

    <p class="mt-2 truncate font-mono text-[11px] text-[var(--text-muted)]" :title="project.path">
      {{ project.path }}
    </p>
  </article>
</template>
