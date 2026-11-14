<script setup lang="ts">
import { computed, onMounted } from 'vue';
import { AlertTriangle } from '@lucide/vue';
import { useAppStore } from '@/stores/app';
import { useTheme } from '@/composables/useTheme';
import type { Settings } from '@/services/backend';
import AppHeader from '@/components/AppHeader.vue';
import ProjectsView from '@/views/ProjectsView.vue';
import SettingsView from '@/views/SettingsView.vue';

const store = useAppStore();
const themeRef = computed(() => store.settings.theme);
useTheme(themeRef);

onMounted(() => {
  void store.initialize();
});

function handleToggleSettings() {
  store.setView(store.activeView === 'settings' ? 'projects' : 'settings');
}

function handleSaveSettings(next: Settings): Promise<boolean> {
  return store.updateSettings(next);
}
</script>

<template>
  <div class="flex h-screen min-w-[360px] flex-col bg-[var(--bg-app)] text-[var(--text-primary)]">
    <AppHeader
      :search-query="store.searchQuery"
      :theme="store.settings.theme"
      :active-view="store.activeView"
      :project-count="store.projects.length"
      :theme-disabled="store.saving || !store.settingsLoaded"
      @update:search-query="store.searchQuery = $event"
      @cycle-theme="store.cycleTheme"
      @toggle-settings="handleToggleSettings"
    />

    <main class="flex-1 overflow-y-auto px-3.5 py-3">
      <div
        v-if="store.error"
        role="alert"
        class="mb-3 flex items-start gap-2 rounded-xl border border-[var(--danger)]/30 bg-[var(--danger-soft)] p-2.5 text-xs text-[var(--danger)]"
      >
        <AlertTriangle class="mt-0.5 h-3.5 w-3.5 shrink-0" />
        <span class="break-words">{{ store.error }}</span>
      </div>

      <ProjectsView
        v-if="store.activeView === 'projects'"
        :projects="store.filteredProjects"
        :total-projects="store.projects.length"
        :search-query="store.searchQuery"
        :config-dir="store.appInfo?.configDir ?? ''"
        :loading="store.loading"
        @clear-search="store.searchQuery = ''"
      />

      <SettingsView
        v-else
        :settings="store.settings"
        :config-dir="store.appInfo?.configDir ?? ''"
        :saving="store.saving"
        :can-save="store.settingsLoaded"
        :save-settings="handleSaveSettings"
      />
    </main>

    <footer
      class="flex items-center justify-between border-t border-[var(--border-subtle)] bg-[var(--bg-surface)] px-3.5 py-2 text-[11px] text-[var(--text-muted)]"
    >
      <span class="truncate font-mono">
        {{ store.appInfo?.os ?? 'linux' }}/{{ store.appInfo?.arch ?? 'amd64' }}
      </span>
      <span>v{{ store.appInfo?.version ?? '0.1.0' }}</span>
    </footer>
  </div>
</template>
