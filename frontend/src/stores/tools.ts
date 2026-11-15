import { ref } from 'vue';
import { defineStore } from 'pinia';
import { backend } from '@/services/backend';
import type { TerminalPreset, ToolStatus, Target } from '@/services/backend';
import { errorMessage } from '@/lib/errors';

/** Availability of the applications projects can be opened with. */
export const useToolsStore = defineStore('tools', () => {
  const tools = ref<ToolStatus[]>([]);
  const presets = ref<TerminalPreset[]>([]);
  const loaded = ref(false);
  const error = ref<string | null>(null);
  const refreshedAt = ref(0);
  let presetsLoaded = false;

  async function refresh(): Promise<void> {
    try {
      tools.value = await backend.listTools();
      loaded.value = true;
      error.value = null;
    } catch (err) {
      error.value = errorMessage(err);
    } finally {
      refreshedAt.value = Date.now();
    }
  }

  async function loadPresets(): Promise<void> {
    if (presetsLoaded) {
      return;
    }
    try {
      presets.value = await backend.terminalPresets();
      presetsLoaded = true;
    } catch (err) {
      error.value = errorMessage(err);
    }
  }

  function byTarget(target: Target | string): ToolStatus | undefined {
    return tools.value.find((tool) => tool.target === target);
  }

  return { tools, presets, loaded, error, refreshedAt, refresh, loadPresets, byTarget };
});
