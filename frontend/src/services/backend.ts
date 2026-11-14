import { Service as AppService } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/app/index.js';
import type { Info } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/app/index.js';
import {
  Service as ConfigService,
  ThemeMode,
} from '../../bindings/github.com/semelyanov86/ProjectHub/internal/config/index.js';
import type { Settings } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/config/index.js';
import { Service as ProjectService } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/project/index.js';
import type { Project } from '../../bindings/github.com/semelyanov86/ProjectHub/internal/project/index.js';

export type { Info, Project, Settings };
export { ThemeMode };

export const backend = {
  getAppInfo(): Promise<Info> {
    return AppService.GetInfo();
  },
  getSettings(): Promise<Settings> {
    return ConfigService.GetSettings();
  },
  updateSettings(settings: Settings): Promise<Settings> {
    return ConfigService.UpdateSettings(settings);
  },
  listProjects(): Promise<Project[] | null> {
    return ProjectService.ListProjects();
  },
};
