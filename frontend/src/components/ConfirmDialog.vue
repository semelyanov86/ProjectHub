<script setup lang="ts">
import { ref, watch } from 'vue';
import AppButton from './ui/AppButton.vue';
import AppDialog from './ui/AppDialog.vue';

interface Props {
  open: boolean;
  title: string;
  message: string;
  confirmLabel: string;
  /** Resolves when the confirmed action finished; the dialog stays busy meanwhile. */
  onConfirm: () => Promise<unknown>;
}

const props = defineProps<Props>();

const emit = defineEmits<{
  close: [];
}>();

const busy = ref(false);

watch(
  () => props.open,
  (open) => {
    if (open) {
      busy.value = false;
    }
  },
);

async function confirm() {
  busy.value = true;
  try {
    await props.onConfirm();
  } finally {
    busy.value = false;
    emit('close');
  }
}
</script>

<template>
  <AppDialog :open="open" :title="title" size="sm" :busy="busy" @close="emit('close')">
    <p class="text-xs leading-relaxed text-[var(--text-secondary)]">{{ message }}</p>
    <template #footer>
      <AppButton :disabled="busy" @click="emit('close')">Cancel</AppButton>
      <AppButton
        variant="primary"
        class="!bg-[var(--danger)] hover:!opacity-90"
        :disabled="busy"
        data-autofocus
        @click="confirm"
      >
        {{ confirmLabel }}
      </AppButton>
    </template>
  </AppDialog>
</template>
