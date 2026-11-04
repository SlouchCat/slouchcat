import type { Chapter } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { chapterLabel } from './labels';

interface ChapterChoiceOptions {
  chapters: readonly Chapter[];
  /** The chapter playing (`chapterAt`), -1 before the first one. */
  chapterIndex: number;
  durationMs: number;
  seekTo: (ms: number) => void;
  closePanel: () => void;
}

/**
 * The chapters as a page of the gear, on a phone (11.2), for the online and the offline player.
 * The page closes with the choice, as a choice of the quality does, and the player jumps to the
 * chapter's start.
 */
export function useChapterChoice({
  chapters,
  chapterIndex,
  durationMs,
  seekTo,
  closePanel,
}: ChapterChoiceOptions) {
  const { t } = useTranslation();
  return {
    options: chapters.map((chapter, index) => ({
      id: String(index),
      label: chapterLabel(t, chapter, index, durationMs),
    })),
    selectedId: String(Math.max(0, chapterIndex)),
    onSelect: (id: string) => {
      const chapter = chapters[Number(id)];
      closePanel();
      if (chapter) seekTo(chapter.startMs);
    },
  };
}
