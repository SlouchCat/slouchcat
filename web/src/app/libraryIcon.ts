import type { components } from '@slouchcat/api-client';
import type { IconName } from '@/ui';

/**
 * Icon of a library in the side navigation (concept 17, D-8): a video library that holds only
 * series folders the television, any other video library the film strip.
 */
export function libraryIcon({ kind, contents }: components['schemas']['LibraryNav']): IconName {
  if (kind === 'audiobooks') return 'book';
  if (kind === 'podcasts') return 'mic';
  return contents.length > 0 && contents.every((content) => content === 'series') ? 'tv' : 'film';
}
