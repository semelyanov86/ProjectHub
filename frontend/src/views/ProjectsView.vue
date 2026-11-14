<script setup lang="ts">
import { SearchX } from '@lucide/vue';
import type { Project } from '@/services/backend';
import EmptyState from '@/components/EmptyState.vue';
import ProjectCard from '@/components/ProjectCard.vue';
import AppButton from '@/components/ui/AppButton.vue';

interface Props {
  projects: Project[];
  totalProjects: number;
  searchQuery: string;
  configDir: string;
  loading: boolean;
}

defineProps<Props>();

const emit = defineEmits<{
  clearSearch: [];
}>();
</script>

<template>
  <div class="space-y-2.5">
    <div
      v-if="loading"
      class="rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] p-4 text-center text-xs text-[var(--text-muted)]"
    >
      Loading local projects...
    </div>

    <EmptyState v-else-if="totalProjects === 0" :config-dir="configDir" />

    <div
      v-else-if="projects.length === 0"
      class="flex flex-col items-center rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-4 py-8 text-center shadow-[var(--shadow-card)]"
    >
      <SearchX class="h-6 w-6 text-[var(--text-muted)]" />
      <p class="mt-2 text-xs font-medium text-[var(--text-primary)]">
        No projects match "{{ searchQuery }}"
      </p>
      <AppButton class="mt-3" size="sm" @click="emit('clearSearch')">Clear filter</AppButton>
    </div>

    <div v-else class="space-y-2">
      <ProjectCard v-for="project in projects" :key="project.id" :project="project" />
    </div>
  </div>
</template>
