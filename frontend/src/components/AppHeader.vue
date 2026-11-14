<script setup lang="ts">
import { computed } from 'vue';
import { ArrowLeft, Monitor, Moon, Search, SlidersHorizontal, Sun } from '@lucide/vue';
import { ThemeMode } from '@/services/backend';
import type { ActiveView } from '@/stores/app';
import AppButton from './ui/AppButton.vue';
import AppInput from './ui/AppInput.vue';

interface Props {
  searchQuery: string;
  theme: ThemeMode;
  activeView: ActiveView;
  projectCount: number;
  themeDisabled?: boolean;
}

const props = withDefaults(defineProps<Props>(), {
  themeDisabled: false,
});

const emit = defineEmits<{
  'update:searchQuery': [value: string];
  cycleTheme: [];
  toggleSettings: [];
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
  <header
    class="border-b border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3.5 py-2.5 shadow-[var(--shadow-card)]"
  >
    <div class="flex items-center justify-between gap-2">
      <div class="flex items-center gap-2.5">
        <img src="/projecthub.svg" alt="ProjectHub" class="h-6 w-6 rounded-md" />
        <div class="flex items-baseline gap-2">
          <h1 class="text-sm font-semibold tracking-tight text-[var(--text-primary)]">
            ProjectHub
          </h1>
          <span
            v-if="projectCount > 0 && activeView === 'projects'"
            class="rounded-full bg-[var(--bg-elevated)] px-1.5 py-0.5 text-[10px] font-medium text-[var(--text-muted)]"
          >
            {{ projectCount }}
          </span>
        </div>
      </div>

      <div class="flex items-center gap-1">
        <AppButton
          variant="ghost"
          size="sm"
          :disabled="themeDisabled"
          :title="themeLabel"
          :aria-label="themeLabel"
          @click="emit('cycleTheme')"
        >
          <Sun v-if="theme === ThemeMode.ThemeLight" class="h-3.5 w-3.5" />
          <Moon v-else-if="theme === ThemeMode.ThemeDark" class="h-3.5 w-3.5" />
          <Monitor v-else class="h-3.5 w-3.5" />
        </AppButton>

        <AppButton
          variant="ghost"
          size="sm"
          :title="activeView === 'settings' ? 'Back to projects' : 'Settings'"
          :aria-label="activeView === 'settings' ? 'Back to projects' : 'Settings'"
          @click="emit('toggleSettings')"
        >
          <ArrowLeft v-if="activeView === 'settings'" class="h-3.5 w-3.5" />
          <SlidersHorizontal v-else class="h-3.5 w-3.5" />
        </AppButton>
      </div>
    </div>

    <div v-if="activeView === 'projects'" class="mt-2.5">
      <AppInput
        :model-value="searchQuery"
        placeholder="Filter projects by name or path..."
        aria-label="Filter projects"
        clearable
        @update:model-value="emit('update:searchQuery', $event)"
      >
        <template #leading>
          <Search class="h-3.5 w-3.5 shrink-0 text-[var(--text-muted)]" />
        </template>
      </AppInput>
    </div>
  </header>
</template>
