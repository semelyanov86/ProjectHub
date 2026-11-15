<script setup lang="ts">
import { CircleCheck, CircleX, Info, TriangleAlert, X } from '@lucide/vue';
import { useNotificationsStore } from '@/stores/notifications';

const notifications = useNotificationsStore();
const tones = {
  info: { icon: Info, color: 'text-[var(--accent)]' },
  success: { icon: CircleCheck, color: 'text-[var(--success)]' },
  warning: { icon: TriangleAlert, color: 'text-[var(--warning)]' },
  error: { icon: CircleX, color: 'text-[var(--danger)]' },
};
</script>

<template>
  <div
    v-if="notifications.items.length"
    role="region"
    aria-label="Notifications"
    class="pointer-events-none fixed bottom-3 left-1/2 z-50 w-[calc(100%-24px)] max-w-[400px] -translate-x-1/2"
  >
    <TransitionGroup name="toast" tag="div" class="flex flex-col gap-1.5">
      <div
        v-for="toast in notifications.items"
        :key="toast.id"
        :role="toast.tone === 'error' ? 'alert' : 'status'"
        class="pointer-events-auto flex items-start gap-2 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3 py-2 text-xs text-[var(--text-primary)] shadow-[var(--shadow-popover)]"
      >
        <component
          :is="tones[toast.tone].icon"
          class="mt-0.5 h-4 w-4 shrink-0"
          :class="tones[toast.tone].color"
          aria-hidden="true"
        />
        <div class="min-w-0 flex-1">
          <p class="font-medium break-words">{{ toast.title }}</p>
          <p
            v-if="toast.detail"
            :title="toast.detail"
            class="mt-0.5 line-clamp-3 text-[11px] break-words text-[var(--text-muted)]"
          >
            {{ toast.detail }}
          </p>
        </div>
        <button
          type="button"
          aria-label="Dismiss notification"
          class="shrink-0 cursor-pointer rounded p-0.5 text-[var(--text-muted)] hover:bg-[var(--bg-hover)] focus-visible:outline-2 focus-visible:outline-[var(--accent)]"
          @click="notifications.dismiss(toast.id)"
        >
          <X class="h-3.5 w-3.5" aria-hidden="true" />
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-enter-active,
.toast-leave-active {
  transition:
    opacity 150ms,
    transform 150ms;
}

.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(4px);
}
</style>
