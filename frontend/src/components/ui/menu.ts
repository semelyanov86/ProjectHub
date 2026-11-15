import type { Component } from 'vue';

export interface MenuItem {
  id: string;
  label: string;
  icon?: Component;
  /** Secondary text shown under the label (e.g. why an application is unavailable). */
  hint?: string;
  disabled?: boolean;
  danger?: boolean;
  /** Draw a separator line above this item. */
  separatorBefore?: boolean;
  /** Right-aligned shortcut hint, e.g. "Enter". */
  shortcut?: string;
}

export interface MenuPosition {
  x: number;
  y: number;
  /** Top edge of the anchor to stay above when the menu cannot fit below y. */
  avoidTop?: number;
}
