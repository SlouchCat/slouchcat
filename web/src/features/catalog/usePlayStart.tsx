import type { ItemRef } from '@slouchcat/api-client';
import { useNavigate } from '@tanstack/react-router';
import { lazy, Suspense, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { formatPlayerTime } from '@/lib/format';
import { isAudioKind } from './labels';

// The question comes with react-aria: a chunk of its own, loaded on the first started title, so the
// browsing pages stay without it (P-10).
const ResumeChoice = lazy(() =>
  import('@/ui/aria').then((module) => ({ default: module.ResumeChoice })),
);

/** What a direct start needs to know of its title. */
export type PlayStartItem = Pick<ItemRef, 'id' | 'kind' | 'progress'> & {
  /** The heading of the question ("S01E04 · Die lange Nacht"). */
  title: string;
};

type Started = NonNullable<ItemRef['progress']>;

const startedAt = (progress: ItemRef['progress']): Started | undefined =>
  progress && !progress.completed && progress.positionMs > 0 ? progress : undefined;

const BODY_KEYS = {
  episode: 'item.resumeChoice.bodyEpisode',
  podcast_episode: 'item.resumeChoice.bodyPodcastEpisode',
} as const;

/**
 * A single click that plays without the detail page (concept 20 P-4, F-7): the episode lists of
 * series, seasons and podcasts and «Abspielen» of a film in the spotlight. `ask(item, event)` on
 * the click of such a play link: a started title (position over 0, not finished) asks first
 * whether to resume or start over, and the link does nothing itself; any other title plays at
 * once. Render `dialog` once beside the links. Not for the volumes of an audiobook series (they
 * always resume, O-9), the cards of «Weiterschauen» or the detail page.
 */
export function usePlayStart(): {
  ask: (item: PlayStartItem, event?: { preventDefault: () => void }) => boolean;
  dialog: ReactNode;
  asking: boolean;
} {
  const { t } = useTranslation();
  const navigate = useNavigate();
  // The last title asked about stays while the dialog closes, so it can give the focus back.
  const [pending, setPending] = useState<{ item: PlayStartItem; progress: Started } | null>(null);
  const [open, setOpen] = useState(false);

  const ask = (item: PlayStartItem, event?: { preventDefault: () => void }) => {
    const progress = startedAt(item.progress);
    if (!progress) return false;
    event?.preventDefault();
    setPending({ item, progress });
    setOpen(true);
    return true;
  };

  const go = (href: string) => void navigate({ href });
  const dialog = pending && (
    <Suspense fallback={null}>
      <ResumeChoice
        isOpen={open}
        onOpenChange={setOpen}
        title={pending.item.title}
        body={t(
          pending.item.kind in BODY_KEYS
            ? BODY_KEYS[pending.item.kind as keyof typeof BODY_KEYS]
            : 'item.resumeChoice.bodyMovie',
        )}
        resumeLabel={t(
          isAudioKind(pending.item.kind)
            ? 'item.resumeChoice.resumeListening'
            : 'item.resumeChoice.resume',
          { time: formatPlayerTime(pending.progress.positionMs, pending.progress.durationMs) },
        )}
        restartLabel={t('item.resumeChoice.restart')}
        cancelLabel={t('common.cancel')}
        onResume={() => go(paths.play(pending.item.id))}
        onRestart={() => go(paths.playAt(pending.item.id, 0))}
      />
    </Suspense>
  );
  return { ask, dialog, asking: open };
}
