<script setup lang="ts">
import { computed } from 'vue';
import { Monitor, Moon, Settings, Sun } from '@lucide/vue';
import { ThemeMode } from '@/services/backend';
import AppButton from './ui/AppButton.vue';

interface Props {
  theme: ThemeMode;
  projectCount: number;
  themeDisabled?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  themeDisabled: false,
});

const emit = defineEmits<{
  cycleTheme: [];
  openSettings: [];
}>();

const themeLabel = computed(() => {
  switch (props.theme) {
    case ThemeMode.ThemeDark:
      return 'Theme: Dark (click to cycle)';
    case ThemeMode.ThemeLight:
      return 'Theme: Light (click to cycle)';
    default:
      return 'Theme: System (click to cycle)';
  }
});
</script>

<template>
  <header class="flex items-center justify-between gap-2 px-3.5 pt-2.5 pb-2">
    <div class="flex min-w-0 items-center gap-2.5">
      <img src="/projecthub.svg" alt="" class="h-6 w-6 rounded-md" />
      <h1 class="text-sm font-semibold tracking-tight text-[var(--text-primary)]">ProjectHub</h1>
      <span
        v-if="projectCount > 0"
        class="rounded-full bg-[var(--bg-elevated)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--text-muted)]"
        :title="`${projectCount} registered projects`"
      >
        {{ projectCount }}
      </span>
    </div>

    <div class="flex items-center gap-0.5">
      <AppButton
        variant="ghost"
        size="sm"
        class="!px-1.5"
        :disabled="themeDisabled"
        :title="themeLabel"
        :aria-label="themeLabel"
        @click="emit('cycleTheme')"
      >
        <Sun v-if="theme === ThemeMode.ThemeLight" class="h-4 w-4" />
        <Moon v-else-if="theme === ThemeMode.ThemeDark" class="h-4 w-4" />
        <Monitor v-else class="h-4 w-4" />
      </AppButton>
      <AppButton
        variant="ghost"
        size="sm"
        class="!px-1.5"
        title="Settings"
        aria-label="Settings"
        @click="emit('openSettings')"
      >
        <Settings class="h-4 w-4" />
      </AppButton>
    </div>
  </header>
</template>
