import { computed, onMounted, onUnmounted, ref, watchEffect, type Ref } from 'vue';
import { ThemeMode } from '@/services/backend';

export function useTheme(preference: Ref<ThemeMode>) {
  const systemPrefersDark = ref(true);
  let mediaQuery: MediaQueryList | null = null;

  const updateSystemPreference = () => {
    if (mediaQuery) {
      systemPrefersDark.value = mediaQuery.matches;
    }
  };

  const resolvedTheme = computed<'light' | 'dark'>(() => {
    if (preference.value === ThemeMode.ThemeLight) {
      return 'light';
    }
    if (preference.value === ThemeMode.ThemeDark) {
      return 'dark';
    }
    return systemPrefersDark.value ? 'dark' : 'light';
  });

  onMounted(() => {
    if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
      mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      updateSystemPreference();
      mediaQuery.addEventListener('change', updateSystemPreference);
    }
  });

  onUnmounted(() => {
    mediaQuery?.removeEventListener('change', updateSystemPreference);
  });

  watchEffect(() => {
    if (typeof document !== 'undefined') {
      document.documentElement.setAttribute('data-theme', resolvedTheme.value);
    }
  });

  return {
    resolvedTheme,
  };
}
