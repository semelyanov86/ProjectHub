<script setup lang="ts">
import { ref } from 'vue';
import {
  ArrowDownAZ,
  Clock,
  Download,
  Ellipsis,
  Plus,
  RefreshCw,
  Search,
  Upload,
} from '@lucide/vue';
import { ProjectSort } from '@/services/backend';
import AppButton from './ui/AppButton.vue';
import AppInput from './ui/AppInput.vue';
import AppMenu from './ui/AppMenu.vue';
import type { MenuItem, MenuPosition } from './ui/menu';

interface Props {
  searchQuery: string;
  sort: ProjectSort;
  refreshing: boolean;
  sortDisabled?: boolean;
}

const props = withDefaults(defineProps<Props>(), { sortDisabled: false });

const emit = defineEmits<{
  'update:searchQuery': [value: string];
  'update:sort': [value: ProjectSort];
  add: [];
  refresh: [];
  import: [];
  export: [];
  searchArrowDown: [];
}>();

const searchInput = ref<InstanceType<typeof AppInput> | null>(null);
const menuOpen = ref(false);
const menuPosition = ref<MenuPosition>({ x: 0, y: 0 });

const menuItems: MenuItem[] = [
  { id: 'import', label: 'Import projects…', icon: Upload },
  { id: 'export', label: 'Export projects…', icon: Download },
];

function toggleSort() {
  emit(
    'update:sort',
    props.sort === ProjectSort.ProjectSortName
      ? ProjectSort.ProjectSortLastOpened
      : ProjectSort.ProjectSortName,
  );
}

function openMenu(event: MouseEvent) {
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect();
  menuPosition.value = { x: rect.right - 200, y: rect.bottom + 4 };
  menuOpen.value = true;
}

function onMenuSelect(id: string) {
  menuOpen.value = false;
  if (id === 'import') {
    emit('import');
  } else if (id === 'export') {
    emit('export');
  }
}

function onSearchKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') {
    event.preventDefault();
    emit('searchArrowDown');
  } else if (event.key === 'Escape' && props.searchQuery) {
    event.preventDefault();
    emit('update:searchQuery', '');
  }
}

defineExpose({
  focusSearch() {
    searchInput.value?.focus();
  },
});
</script>

<template>
  <div class="flex items-center gap-1.5">
    <div class="min-w-0 flex-1" @keydown="onSearchKeydown">
      <AppInput
        ref="searchInput"
        :model-value="searchQuery"
        placeholder="Search name, description or path"
        aria-label="Search projects"
        clearable
        @update:model-value="emit('update:searchQuery', $event)"
      >
        <template #leading>
          <Search class="h-3.5 w-3.5 shrink-0 text-[var(--text-muted)]" />
        </template>
      </AppInput>
    </div>

    <AppButton
      variant="ghost"
      size="sm"
      class="!px-1.5"
      :disabled="sortDisabled"
      :title="
        sort === ProjectSort.ProjectSortName
          ? 'Sorted by name — switch to last opened'
          : 'Sorted by last opened — switch to name'
      "
      :aria-label="sort === ProjectSort.ProjectSortName ? 'Sort by last opened' : 'Sort by name'"
      @click="toggleSort"
    >
      <ArrowDownAZ v-if="sort === ProjectSort.ProjectSortName" class="h-4 w-4" />
      <Clock v-else class="h-4 w-4" />
    </AppButton>

    <AppButton
      variant="ghost"
      size="sm"
      class="!px-1.5"
      :disabled="refreshing"
      title="Refresh Git status"
      aria-label="Refresh Git status"
      @click="emit('refresh')"
    >
      <RefreshCw class="h-4 w-4" :class="refreshing ? 'animate-spin' : ''" />
    </AppButton>

    <AppButton
      variant="ghost"
      size="sm"
      class="!px-1.5"
      title="Import or export"
      aria-label="Import or export"
      aria-haspopup="menu"
      @click="openMenu"
    >
      <Ellipsis class="h-4 w-4" />
    </AppButton>

    <AppButton
      variant="primary"
      size="sm"
      title="Add project"
      aria-label="Add project"
      @click="emit('add')"
    >
      <Plus class="h-3.5 w-3.5" />
      Add
    </AppButton>

    <AppMenu
      :open="menuOpen"
      :position="menuPosition"
      :items="menuItems"
      label="Import and export"
      @select="onMenuSelect"
      @close="menuOpen = false"
    />
  </div>
</template>
