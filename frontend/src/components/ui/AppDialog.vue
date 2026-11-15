<script setup lang="ts">
import { nextTick, ref, useId, watch } from 'vue';
import { X } from '@lucide/vue';

const props = withDefaults(
  defineProps<{
    open: boolean;
    title: string;
    description?: string;
    size?: 'sm' | 'md';
    busy?: boolean;
    closeOnBackdrop?: boolean;
  }>(),
  {
    description: undefined,
    size: 'md',
    busy: false,
    closeOnBackdrop: true,
  },
);

const emit = defineEmits<{ close: [] }>();
const titleId = useId();
const descriptionId = useId();
const panel = ref<HTMLElement | null>(null);
const body = ref<HTMLElement | null>(null);
let backdropPointerdown = false;
const focusableSelector =
  'button, [href], input, select, textarea, [tabindex], [contenteditable="true"]';

function focusableElements(root: HTMLElement | null): HTMLElement[] {
  return Array.from(root?.querySelectorAll<HTMLElement>(focusableSelector) ?? []).filter(
    (element) =>
      element.tabIndex >= 0 &&
      !element.matches(':disabled') &&
      !element.closest('[hidden], [inert]') &&
      getComputedStyle(element).display !== 'none' &&
      getComputedStyle(element).visibility !== 'hidden',
  );
}

function close() {
  if (!props.busy) emit('close');
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault();
    event.stopPropagation();
    close();
    return;
  }
  if (event.key !== 'Tab') return;
  const elements = focusableElements(panel.value);
  const first = elements[0];
  const last = elements[elements.length - 1];
  if (!first) {
    event.preventDefault();
    panel.value?.focus();
  } else if (
    event.shiftKey &&
    (document.activeElement === first || !elements.includes(document.activeElement as HTMLElement))
  ) {
    event.preventDefault();
    last?.focus();
  } else if (
    !event.shiftKey &&
    (document.activeElement === last || !elements.includes(document.activeElement as HTMLElement))
  ) {
    event.preventDefault();
    first.focus();
  }
}

function onBackdropClick(event: MouseEvent) {
  if (backdropPointerdown && event.target === event.currentTarget && props.closeOnBackdrop) close();
  backdropPointerdown = false;
}

watch(
  () => props.open,
  async (open, _previous, onCleanup) => {
    if (!open) return;
    const previousFocus = document.activeElement;
    let cancelled = false;
    backdropPointerdown = false;
    onCleanup(() => {
      cancelled = true;
      if (previousFocus instanceof HTMLElement && previousFocus.isConnected)
        previousFocus.focus({ preventScroll: true });
    });
    await nextTick();
    if (cancelled) return;
    const target =
      panel.value?.querySelector<HTMLElement>('[data-autofocus]') ??
      focusableElements(body.value)[0] ??
      panel.value;
    target?.focus({ preventScroll: true });
  },
  { immediate: true },
);
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="fixed inset-0 z-40 flex items-center justify-center bg-[var(--overlay)] p-3"
      @pointerdown="backdropPointerdown = $event.target === $event.currentTarget"
      @click="onBackdropClick"
    >
      <section
        ref="panel"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleId"
        :aria-describedby="description ? descriptionId : undefined"
        tabindex="-1"
        class="flex max-h-[calc(100vh-24px)] max-w-[calc(100vw-24px)] flex-col rounded-xl border border-[var(--border-subtle)] bg-[var(--bg-surface)] text-[var(--text-primary)] shadow-[var(--shadow-popover)] outline-none"
        :class="size === 'sm' ? 'w-[340px]' : 'w-[400px]'"
        @keydown="onKeydown"
      >
        <header
          class="flex shrink-0 items-start justify-between gap-3 border-b border-[var(--border-subtle)] px-4 py-3"
        >
          <div class="min-w-0">
            <h2 :id="titleId" class="text-sm font-semibold">{{ title }}</h2>
            <p
              v-if="description"
              :id="descriptionId"
              class="mt-1 text-[11px] text-[var(--text-muted)]"
            >
              {{ description }}
            </p>
          </div>
          <button
            type="button"
            aria-label="Close"
            :disabled="busy"
            class="shrink-0 cursor-pointer rounded-md p-1 text-[var(--text-muted)] hover:bg-[var(--bg-hover)] focus-visible:outline-2 focus-visible:outline-[var(--accent)] disabled:cursor-not-allowed disabled:opacity-50"
            @click="close"
          >
            <X class="h-4 w-4" aria-hidden="true" />
          </button>
        </header>
        <div ref="body" class="min-h-0 overflow-y-auto px-4 py-3"><slot /></div>
        <footer
          v-if="$slots.footer"
          class="flex shrink-0 items-center justify-end gap-2 border-t border-[var(--border-subtle)] px-4 py-3"
        >
          <slot name="footer" />
        </footer>
      </section>
    </div>
  </Teleport>
</template>
