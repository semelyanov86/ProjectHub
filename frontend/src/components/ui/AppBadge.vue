<script setup lang="ts">
interface Props {
  tone?: 'neutral' | 'accent';
}

withDefaults(defineProps<Props>(), {
  tone: 'neutral',
});
</script>

<template>
  <span
    class="inline-flex items-center gap-1 rounded-md px-1.5 py-0.5 text-[11px] font-medium"
    :class="
      tone === 'accent'
        ? 'bg-[var(--accent-soft)] text-[var(--accent)]'
        : 'border border-[var(--border-subtle)] bg-[var(--bg-elevated)] text-[var(--text-secondary)]'
    "
  >
    <slot />
  </span>
</template>
