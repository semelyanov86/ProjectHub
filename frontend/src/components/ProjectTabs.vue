<script setup lang="ts">
import type { ProjectTab } from '@/lib/projects';

interface Props {
  modelValue: ProjectTab;
  counts: Record<ProjectTab, number>;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  'update:modelValue': [value: ProjectTab];
}>();

const tabs: { id: ProjectTab; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'favorites', label: 'Favorites' },
  { id: 'recent', label: 'Recent' },
];

function onKeydown(event: KeyboardEvent) {
  const index = tabs.findIndex((tab) => tab.id === props.modelValue);
  let next: number;
  if (event.key === 'ArrowRight') {
    next = (index + 1) % tabs.length;
  } else if (event.key === 'ArrowLeft') {
    next = (index - 1 + tabs.length) % tabs.length;
  } else {
    return;
  }
  event.preventDefault();
  const tab = tabs[next];
  if (tab) {
    emit('update:modelValue', tab.id);
    (event.currentTarget as HTMLElement)
      .querySelector<HTMLButtonElement>(`[data-tab="${tab.id}"]`)
      ?.focus();
  }
}
</script>

<template>
  <div
    role="tablist"
    aria-label="Project filter"
    class="flex items-center gap-1"
    @keydown="onKeydown"
  >
    <button
      v-for="tab in tabs"
      :key="tab.id"
      type="button"
      role="tab"
      :data-tab="tab.id"
      :aria-selected="modelValue === tab.id"
      :tabindex="modelValue === tab.id ? 0 : -1"
      class="inline-flex cursor-pointer items-center gap-1 rounded-md px-2 py-1 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-[var(--accent)]"
      :class="
        modelValue === tab.id
          ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
          : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]'
      "
      @click="emit('update:modelValue', tab.id)"
    >
      {{ tab.label }}
      <span class="text-[10px] opacity-70">{{ counts[tab.id] }}</span>
    </button>
  </div>
</template>
