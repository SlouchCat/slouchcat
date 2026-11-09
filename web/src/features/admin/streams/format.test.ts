import type { AdminStream } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { formatBitrate } from '@/lib/format';
import { streamQuality } from './format';

const t = i18n.t.bind(i18n);

const stream: AdminStream = {
  id: 'ps-1',
  accountEmail: 'owner@example.org',
  profileName: 'Marco',
  itemId: '01J9000000000000000000FILM',
  itemTitle: 'Alien',
  mediaFileId: 'mf-1',
  kind: 'movie',
  source: 'file',
  mode: 'remux',
  hardware: false,
  startedAt: '2026-11-04T20:00:00Z',
  lastActivityAt: '2026-11-04T20:00:00Z',
};

describe('admin stream quality (review 7, m-8)', () => {
  it('names original with the word of the locales, not the raw value', () => {
    expect(streamQuality(t, { ...stream, quality: 'original', bitrate: 6_000_000 }, 'de')).toBe(
      `Original · ${formatBitrate(6_000_000, 'de')}`,
    );
    expect(streamQuality(t, { ...stream, quality: 'original' }, 'de')).toBe('Original');
  });

  it('keeps the rungs, the height of auto and the source of direct play', () => {
    expect(streamQuality(t, { ...stream, mode: 'transcode', quality: '2160p' }, 'de')).toBe(
      '2160p',
    );
    expect(streamQuality(t, { ...stream, quality: 'auto', height: 1080 }, 'de')).toBe('1080p');
    expect(streamQuality(t, { ...stream, mode: 'direct' }, 'de')).toBe('Original');
  });
});
