import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import {
  formatBytes,
  formatClock,
  formatDate,
  formatDuration,
  formatPlayerTime,
  formatRuntime,
  resolutionClass,
} from './format';

const t = i18n.t.bind(i18n);

describe('format', () => {
  it('formats resume positions as hh:mm:ss', () => {
    expect(formatClock(2_533_000)).toBe('00:42:13');
    expect(formatClock(0)).toBe('00:00:00');
    expect(formatClock(36_000_000)).toBe('10:00:00');
  });

  it('uses hours in the player only for long titles', () => {
    expect(formatPlayerTime(65_000, 600_000)).toBe('1:05');
    expect(formatPlayerTime(65_000, 7_200_000)).toBe('0:01:05');
  });

  it('formats runtimes', () => {
    expect(formatRuntime(t, 45)).toBe('45 min');
    expect(formatRuntime(t, 120)).toBe('2 h');
    expect(formatRuntime(t, 152)).toBe('2 h 32 min');
  });

  it('formats audio durations and publication dates (M6)', () => {
    expect(formatDuration(t, 8_000)).toBe('8 s');
    expect(formatDuration(t, 59_400)).toBe('59 s');
    expect(formatDuration(t, 60_000)).toBe('1 min');
    expect(formatDuration(t, 3_900_000)).toBe('1 h 5 min');
    expect(formatDate('2025-10-13T12:00:00Z', 'de-CH')).toBe('13.10.2025');
    expect(formatDate('2025-10-13T12:00:00Z', 'en')).toBe('Oct 13, 2025');
  });

  it('formats sizes in the profile language', () => {
    expect(formatBytes(14_200_000_000, 'de')).toMatch(/^14,2\sGB$/);
    expect(formatBytes(2_624_977, 'en')).toMatch(/^3\sMB$/);
  });

  it('classifies letterboxed and full-frame resolutions', () => {
    expect(resolutionClass(t, 1920, 800)).toBe('1080p');
    expect(resolutionClass(t, 3840, 1600)).toBe('4K');
    expect(resolutionClass(t, 1280, 536)).toBe('720p');
    expect(resolutionClass(t, 720, 576)).toBe('SD');
  });
});
