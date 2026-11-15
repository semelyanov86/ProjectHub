import { Target } from '@/services/backend';

export interface EditorOption {
  id: Target;
  name: string;
}

/** Supported editors in menu order; mirrors internal/editor in Go. */
export const EDITORS: readonly EditorOption[] = [
  { id: Target.TargetSublime, name: 'Sublime Text' },
  { id: Target.TargetZed, name: 'Zed' },
];

export const EDITOR_IDS: readonly string[] = EDITORS.map((editor) => editor.id);

export function editorName(id: string): string {
  return EDITORS.find((editor) => editor.id === id)?.name ?? id;
}
