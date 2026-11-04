import type { CastMember, Chapter, ItemRef } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { imageUrl } from '@/api/client';
import { creditCaption, episodeCode } from '@/features/catalog/labels';
import { formatRuntime } from '@/lib/format';
import { ChapterCard, PlayerCard, type PlayerChromeTab } from '@/ui/aria';
import { chapterAt } from '../labels';
import styles from '../Player.module.css';

/** The ids of the tabs under the bar (11.2); C opens `chapters`. */
export type PlayerTabId = 'chapters' | 'next' | 'cast' | 'info';

/**
 * How much of a chapter has played: 100 behind the position, none ahead of it nor at its very
 * start (no «0 % gesehen»).
 */
export function chapterProgress(chapter: Chapter, positionMs: number): number | undefined {
  if (positionMs <= chapter.startMs) return undefined;
  if (positionMs >= chapter.endMs || chapter.endMs <= chapter.startMs) return 100;
  return ((positionMs - chapter.startMs) / (chapter.endMs - chapter.startMs)) * 100;
}

interface PlayerTabsInput {
  t: TFunction;
  chapters: readonly Chapter[];
  positionMs: number;
  durationMs: number;
  onChapter: (startMs: number) => void;
  /** Bumped by C: the chapter playing takes the focus (PlayerChromeTab `focusRequest`). */
  focusChapter?: number;
  /** The episodes of the season of an episode (itemQuery of the season), none for a film. */
  episodes?: readonly ItemRef[];
  /** The title that plays: its card is marked and pressing it closes the tab. */
  currentId: string;
  onEpisode: (episode: ItemRef) => void;
  onClose: () => void;
  cast?: readonly CastMember[];
  onPerson: (personId: number) => void;
  /**
   * The description and its facts line («2017 · 2 Std. 44 Min. · Science-Fiction», an episode
   * «S01E04 · Wahrheit · 52 Min.»), from the title's details.
   */
  info?: { overview?: string; facts?: string };
}

/**
 * The tabs under the bar (11.2, V-8, V-11): «Kapitel» as chapter cards (number and time until
 * stage 3 brings the images), «Als Nächstes» with the episodes of the season, «Besetzung» and
 * «Info». A tab without content is left out. Everything comes from requests the detail page makes
 * as well (11.10); the server decided what the profile may see (rule 12).
 *
 * A hook by name only (it calls none). As a plain function the React compiler takes it to run
 * the callbacks it gets (seekTo, which reads the video ref) while the player renders, and refuses
 * the player («Cannot access refs during render»). The cards' `render` runs while CardRow renders;
 * the callbacks inside it run on a press.
 */
export function usePlayerTabs({
  t,
  chapters,
  positionMs,
  durationMs,
  onChapter,
  focusChapter,
  episodes,
  currentId,
  onEpisode,
  onClose,
  cast,
  onPerson,
  info,
}: PlayerTabsInput): PlayerChromeTab[] {
  const tabs: PlayerChromeTab[] = [];
  if (chapters.length > 0) {
    const current = Math.max(0, chapterAt(chapters, positionMs));
    tabs.push({
      id: 'chapters',
      label: t('player.chapters.button'),
      cards: {
        count: chapters.length,
        current,
        focusRequest: focusChapter,
        render: (index, excluded) => {
          const chapter = chapters[index]!;
          return (
            <ChapterCard
              index={index}
              title={chapter.title}
              startMs={chapter.startMs}
              durationMs={durationMs}
              progress={chapterProgress(chapter, positionMs)}
              isCurrent={index === current}
              onPress={() => onChapter(chapter.startMs)}
              excludeFromTabOrder={excluded}
            />
          );
        },
      },
    });
  }
  if (episodes && episodes.length > 0) {
    const current = episodes.findIndex((episode) => episode.id === currentId);
    tabs.push({
      id: 'next',
      label: t('player.tabs.next'),
      cards: {
        count: episodes.length,
        // The row starts at the episode after this one: «Als Nächstes».
        current: current < 0 ? 0 : Math.min(current + 1, episodes.length - 1),
        render: (index, excluded) => {
          const episode = episodes[index]!;
          const percent = episode.progress?.percent;
          return (
            <PlayerCard
              eyebrow={episodeCode(t, episode)}
              title={episode.title}
              caption={
                episode.runtimeMinutes ? formatRuntime(t, episode.runtimeMinutes) : undefined
              }
              imageUrl={imageUrl(episode.images.backdropSmall ?? episode.images.backdrop)}
              progress={
                percent !== undefined && percent > 0
                  ? {
                      percent,
                      label: t('player.chapters.progress', { percent: Math.round(percent) }),
                    }
                  : undefined
              }
              isCurrent={episode.id === currentId}
              onPress={() => (episode.id === currentId ? onClose() : onEpisode(episode))}
              excludeFromTabOrder={excluded}
            />
          );
        },
      },
    });
  }
  if (cast && cast.length > 0) {
    tabs.push({
      id: 'cast',
      label: t('item.cast.title'),
      cards: {
        count: cast.length,
        render: (index, excluded) => {
          const member = cast[index]!;
          return (
            <PlayerCard
              shape="person"
              title={member.name}
              caption={creditCaption(t, member)}
              onPress={() => onPerson(member.personId)}
              excludeFromTabOrder={excluded}
            />
          );
        },
      },
    });
  }
  if (info && (info.overview || info.facts)) {
    tabs.push({
      id: 'info',
      label: t('player.tabs.info'),
      content: (
        <div className={styles.tabInfo}>
          {info.facts && <p className={styles.tabFacts}>{info.facts}</p>}
          {info.overview && <p>{info.overview}</p>}
        </div>
      ),
    });
  }
  return tabs;
}
