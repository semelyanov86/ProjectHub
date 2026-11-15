<script setup lang="ts">
import { ref } from 'vue';
import { X } from '@lucide/vue';

interface Props {
  modelValue: string;
  placeholder?: string;
  ariaLabel?: string;
  clearable?: boolean;
}

withDefaults(defineProps<Props>(), {
  placeholder: '',
  ariaLabel: undefined,
  clearable: false,
});

const emit = defineEmits<{
  'update:modelValue': [value: string];
}>();

const input = ref<HTMLInputElement | null>(null);

defineExpose({
  focus() {
    input.value?.focus();
    input.value?.select();
  },
});

function onInput(event: Event) {
  const target = event.target as HTMLInputElement;
  emit('update:modelValue', target.value);
}
</script>

<template>
  <div
    class="flex items-center gap-2 rounded-lg border border-[var(--border-subtle)] bg-[var(--bg-surface)] px-2.5 py-1.5 transition-colors focus-within:border-[var(--accent)]"
  >
    <slot name="leading" />
    <input
      ref="input"
      :value="modelValue"
      :placeholder="placeholder"
      :aria-label="ariaLabel ?? placeholder"
      type="text"
      autocomplete="off"
      spellcheck="false"
      class="w-full bg-transparent text-xs text-[var(--text-primary)] placeholder:text-[var(--text-muted)] focus:outline-none"
      @input="onInput"
    />
    <button
      v-if="clearable && modelValue"
      type="button"
      aria-label="Clear input"
      class="cursor-pointer rounded p-0.5 text-[var(--text-muted)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)]"
      @click="emit('update:modelValue', '')"
    >
      <X class="h-3.5 w-3.5" />
    </button>
  </div>
</template>
