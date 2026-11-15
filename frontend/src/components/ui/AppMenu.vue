<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';
import type { MenuItem, MenuPosition } from './menu';

const props = defineProps<{
  open: boolean;
  position: MenuPosition;
  items: MenuItem[];
  label: string;
  title?: string;
}>();

const emit = defineEmits<{
  select: [id: string];
  close: [];
}>();

const menu = ref<HTMLElement | null>(null);
const coordinates = ref<MenuPosition>({ x: 0, y: 0 });

function buttons(): HTMLButtonElement[] {
  return Array.from(menu.value?.querySelectorAll<HTMLButtonElement>('[role="menuitem"]') ?? []);
}

function placeMenu() {
  if (!menu.value) return;
  const { width, height } = menu.value.getBoundingClientRect();
  const { x, y, avoidTop } = props.position;
  coordinates.value = {
    x: Math.max(
      8,
      Math.min(x + width > window.innerWidth - 8 ? x - width : x, window.innerWidth - width - 8),
    ),
    y: Math.max(
      8,
      Math.min(
        y + height > window.innerHeight - 8 ? (avoidTop ?? y) - height : y,
        window.innerHeight - height - 8,
      ),
    ),
  };
}

function select(item: MenuItem) {
  if (!item.disabled) emit('select', item.id);
}

function onKeydown(event: KeyboardEvent) {
  const elements = buttons();
  const index = elements.findIndex((element) => element === document.activeElement);
  let target: number;
  switch (event.key) {
    case 'ArrowDown':
      target = (index + 1) % elements.length;
      break;
    case 'ArrowUp':
      target = (index - 1 + elements.length) % elements.length;
      break;
    case 'Home':
      target = 0;
      break;
    case 'End':
      target = elements.length - 1;
      break;
    case 'Enter':
    case ' ':
      event.preventDefault();
      if (props.items[index]) select(props.items[index]);
      return;
    case 'Escape':
      event.preventDefault();
      event.stopPropagation();
      emit('close');
      return;
    case 'Tab':
      emit('close');
      return;
    default:
      return;
  }
  event.preventDefault();
  elements[target]?.focus({ preventScroll: true });
}

function onPointerdown(event: PointerEvent) {
  if (event.target instanceof Node && !menu.value?.contains(event.target)) emit('close');
}

function close() {
  emit('close');
}

watch(
  () => props.open,
  async (open, _previous, onCleanup) => {
    if (!open) return;
    const previousFocus = document.activeElement;
    let cancelled = false;
    document.addEventListener('pointerdown', onPointerdown, true);
    window.addEventListener('resize', close);
    window.addEventListener('blur', close);
    window.addEventListener('scroll', close, true);
    onCleanup(() => {
      cancelled = true;
      document.removeEventListener('pointerdown', onPointerdown, true);
      window.removeEventListener('resize', close);
      window.removeEventListener('blur', close);
      window.removeEventListener('scroll', close, true);
      if (
        previousFocus instanceof HTMLElement &&
        previousFocus.isConnected &&
        (menu.value?.contains(document.activeElement) || document.activeElement === document.body)
      ) {
        previousFocus.focus({ preventScroll: true });
      }
    });
    await nextTick();
    if (cancelled) return;
    placeMenu();
    const firstEnabled = props.items.findIndex((item) => !item.disabled);
    buttons()[Math.max(0, firstEnabled)]?.focus({ preventScroll: true });
  },
  { immediate: true },
);

watch(
  () => props.position,
  () => {
    if (props.open) placeMenu();
  },
  { deep: true, flush: 'post' },
);
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      ref="menu"
      role="menu"
      :aria-label="label"
      :style="{ left: `${coordinates.x}px`, top: `${coordinates.y}px` }"
      class="fixed z-50 max-w-[min(280px,calc(100vw-16px))] min-w-[200px] rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] py-1 text-xs shadow-[var(--shadow-popover)]"
      @keydown="onKeydown"
    >
      <div v-if="title" class="px-3 py-1.5 text-[11px] text-[var(--text-muted)]">{{ title }}</div>
      <template v-for="item in items" :key="item.id">
        <div
          v-if="item.separatorBefore"
          role="separator"
          class="my-1 border-t border-[var(--border-subtle)]"
        />
        <button
          type="button"
          role="menuitem"
          tabindex="-1"
          :aria-disabled="item.disabled ? 'true' : undefined"
          :title="item.hint"
          class="flex w-full items-center gap-2 px-3 py-1.5 text-left hover:bg-[var(--bg-hover)] focus:bg-[var(--bg-hover)] focus:outline-none"
          :class="
            item.disabled
              ? 'cursor-not-allowed text-[var(--text-muted)]'
              : item.danger
                ? 'cursor-pointer text-[var(--danger)]'
                : 'cursor-pointer text-[var(--text-primary)]'
          "
          @click="select(item)"
          @mouseenter="($event.currentTarget as HTMLButtonElement).focus({ preventScroll: true })"
        >
          <span
            v-if="item.icon"
            class="flex h-4 w-4 shrink-0 items-center justify-center"
            aria-hidden="true"
          >
            <component :is="item.icon" class="h-4 w-4" />
          </span>
          <span class="min-w-0 flex-1 break-words">
            <span class="block">{{ item.label }}</span>
            <span v-if="item.hint" class="block text-[11px] text-[var(--text-muted)]">{{
              item.hint
            }}</span>
          </span>
          <span v-if="item.shortcut" class="shrink-0 text-[11px] text-[var(--text-muted)]">{{
            item.shortcut
          }}</span>
        </button>
      </template>
    </div>
  </Teleport>
</template>
