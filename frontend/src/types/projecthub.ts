export type ThemeMode = 'system' | 'light' | 'dark';

export interface WindowPreferences {
  width: number;
  height: number;
  rememberSize: boolean;
  alwaysOnTop: boolean;
}

export interface Settings {
  schemaVersion: number;
  defaultEditor: string;
  preferredTerminal: string;
  preferredFileManager: string;
  theme: ThemeMode;
  window: WindowPreferences;
}

export interface Project {
  id: string;
  name: string;
  description: string;
  path: string;
  favorite: boolean;
  defaultEditor: string;
  lastOpened?: string;
  createdAt: string;
  updatedAt: string;
}

export interface AppInfo {
  name: string;
  version: string;
  configDir: string;
  os: string;
  arch: string;
}
