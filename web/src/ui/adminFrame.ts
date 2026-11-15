import { createContext, createElement, useContext, type ReactNode } from 'react';
import type { AreaTabsProps } from './AreaTabs';

// The frame of the administration (redesign, build plan 2.5 and 2.6): the admin layout tells
// every AdminPage below it where the page stands. AdminPage then shows the area's tabs above its
// header, the line «MEDIEN / BIBLIOTHEKEN» over the title and the intro sentence open below it.
// Without the frame (the /settings/* pages, stories without it) AdminPage stays as it was: the
// title alone and the help button.

export interface AdminFrameValue {
  /** The line over the title: area and page ("Medien / Bibliotheken"), shown in capitals. */
  eyebrow: string;
  /** The tabs of the page's area; none on a page outside the areas. */
  tabs?: AreaTabsProps;
}

const AdminFrameContext = createContext<AdminFrameValue | null>(null);

/** Puts the frame around the administration's pages (AdminLayout). */
export function AdminFrame({ value, children }: { value: AdminFrameValue; children: ReactNode }) {
  return createElement(AdminFrameContext, { value }, children);
}

/** The frame the page stands in, or null outside the administration. */
export function useAdminFrame(): AdminFrameValue | null {
  return useContext(AdminFrameContext);
}
