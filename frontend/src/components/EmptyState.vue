<script setup lang="ts">
import { FolderPlus, MousePointerClick } from '@lucide/vue';
import AppButton from './ui/AppButton.vue';

const emit = defineEmits<{
  add: [];
}>();
</script>

<template>
  <section
    class="m-3 flex flex-col items-center rounded-xl border border-dashed border-[var(--border-strong)] bg-[var(--bg-surface)] px-5 py-8 text-center"
  >
    <div
      class="flex h-11 w-11 items-center justify-center rounded-xl bg-[var(--accent-soft)] text-[var(--accent)]"
    >
      <FolderPlus class="h-5 w-5" />
    </div>

    <h2 class="mt-3 text-sm font-semibold text-[var(--text-primary)]">No projects yet</h2>
    <p class="mt-1 max-w-[300px] text-xs leading-relaxed text-[var(--text-secondary)]">
      Add an existing directory to open it in Sublime Text, Zed, Ghostty or your file manager with
      one click.
    </p>

    <AppButton class="mt-4" variant="primary" @click="emit('add')">
      <FolderPlus class="h-3.5 w-3.5" />
      Add Project…
    </AppButton>

    <p class="mt-3 inline-flex items-center gap-1 text-[11px] text-[var(--text-muted)]">
      <MousePointerClick class="h-3.5 w-3.5" />
      Or drag folders from your file manager into this window.
    </p>
  </section>
</template>
