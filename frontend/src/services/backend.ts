import { Clipboard, Events } from '@wailsio/runtime';
import { Service as AppService } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/app/index.js';
import type { Info } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/app/index.js';
import {
  Service as ConfigService,
  CloseBehavior,
  FileManagerMode,
  ProjectSort,
  TerminalMode,
  ThemeMode,
} from '../../bindings/github.com/semelyanov86/ProjectHub/internal/config/index.js';
import type {
  ExecutablePaths,
  Settings,
  TerminalCommand,
  WindowPreferences,
} from '../../bindings/github.com/semelyanov86/ProjectHub/internal/config/index.js';
import {
  Service as DesktopService,
  DirectoryState,
  ImportMode,
} from '../../bindings/github.com/semelyanov86/ProjectHub/internal/desktop/index.js';
import {
  NoticeLevel,
  type DirectoryDraft,
  type DropResult,
  type ExportResult,
  type ImportPreview,
  type ImportReport,
  type ImportRequest,
  type Notice,
  type OpenResult,
  type ProjectInspection,
} from '../../bindings/github.com/semelyanov86/ProjectHub/internal/desktop/index.js';
import { State as GitState } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/git/index.js';
import type {
  Inspection,
  RepoInfo,
} from '../../bindings/github.com/semelyanov86/ProjectHub/internal/git/index.js';
import { Target } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/launcher/index.js';
import type {
  TerminalPreset,
  ToolStatus,
} from '../../bindings/github.com/semelyanov86/ProjectHub/internal/launcher/index.js';
import type {
  BatchReport,
  Input as ProjectInput,
  Project,
  Skip,
} from '../../bindings/github.com/semelyanov86/ProjectHub/internal/project/index.js';

export type {
  BatchReport,
  DirectoryDraft,
  DropResult,
  ExecutablePaths,
  ExportResult,
  ImportPreview,
  ImportReport,
  ImportRequest,
  Info,
  Inspection,
  Notice,
  OpenResult,
  Project,
  ProjectInput,
  ProjectInspection,
  RepoInfo,
  Settings,
  Skip,
  TerminalCommand,
  TerminalPreset,
  ToolStatus,
  WindowPreferences,
};
export {
  CloseBehavior,
  DirectoryState,
  FileManagerMode,
  GitState,
  ImportMode,
  NoticeLevel,
  ProjectSort,
  Target,
  TerminalMode,
  ThemeMode,
};

/** Event emitted by the Go drop handler after directories were dropped on the window. */
export const PROJECTS_DROPPED_EVENT = 'projects:dropped';
/** Event asking the frontend to show Preferences (tray menu "Settings…"). */
export const OPEN_SETTINGS_EVENT = 'app:open-settings';
/** Notification produced by a background action such as opening a project from the tray. */
export const NOTICE_EVENT = 'app:notice';
/** The catalog changed outside the frontend's own calls (e.g. a tray launch). */
export const PROJECTS_CHANGED_EVENT = 'projects:changed';

/**
 * Typed facade over the generated Wails bindings. Components and stores use
 * this module only, so tests can mock it and generated paths stay private.
 */
export const backend = {
  getAppInfo(): Promise<Info> {
    return AppService.GetInfo();
  },
  trayAvailable(): Promise<boolean> {
    return AppService.TrayAvailable();
  },
  getSettings(): Promise<Settings> {
    return ConfigService.GetSettings();
  },
  updateSettings(settings: Settings): Promise<Settings> {
    return ConfigService.UpdateSettings(settings);
  },
  async listProjects(): Promise<Project[]> {
    return (await DesktopService.ListProjects()) ?? [];
  },
  chooseDirectory(): Promise<DirectoryDraft> {
    return DesktopService.ChooseDirectory();
  },
  addProject(input: ProjectInput): Promise<Project> {
    return DesktopService.AddProject(input);
  },
  updateProject(id: string, input: ProjectInput): Promise<Project> {
    return DesktopService.UpdateProject(id, input);
  },
  removeProject(id: string): Promise<void> {
    return DesktopService.RemoveProject(id);
  },
  setFavorite(id: string, favorite: boolean): Promise<Project> {
    return DesktopService.SetFavorite(id, favorite);
  },
  async listTools(): Promise<ToolStatus[]> {
    return (await DesktopService.ListTools()) ?? [];
  },
  async terminalPresets(): Promise<TerminalPreset[]> {
    return (await DesktopService.TerminalPresets()) ?? [];
  },
  openProject(id: string, target: Target): Promise<OpenResult> {
    return DesktopService.OpenProject(id, target);
  },
  openProjectDefault(id: string): Promise<OpenResult> {
    return DesktopService.OpenProjectDefault(id);
  },
  async inspectProjects(ids: string[]): Promise<ProjectInspection[]> {
    return (await DesktopService.InspectProjects(ids)) ?? [];
  },
  prepareImport(): Promise<ImportPreview> {
    return DesktopService.PrepareImport();
  },
  applyImport(request: ImportRequest): Promise<ImportReport> {
    return DesktopService.ApplyImport(request);
  },
  discardImport(token: string): Promise<void> {
    return DesktopService.DiscardImport(token);
  },
  exportProjects(): Promise<ExportResult> {
    return DesktopService.ExportProjects();
  },
  chooseExecutable(): Promise<string> {
    return DesktopService.ChooseExecutable();
  },
  copyText(text: string): Promise<void> {
    return Clipboard.SetText(text);
  },
  onProjectsDropped(callback: (result: DropResult) => void): () => void {
    return Events.On(PROJECTS_DROPPED_EVENT, (event) => callback(event.data as DropResult));
  },
  onWindowFocus(callback: () => void): () => void {
    return Events.On(Events.Types.Common.WindowFocus, () => callback());
  },
  onOpenSettings(callback: () => void): () => void {
    return Events.On(OPEN_SETTINGS_EVENT, () => callback());
  },
  onNotice(callback: (notice: Notice) => void): () => void {
    return Events.On(NOTICE_EVENT, (event) => callback(event.data as Notice));
  },
  onProjectsChanged(callback: () => void): () => void {
    return Events.On(PROJECTS_CHANGED_EVENT, () => callback());
  },
};
