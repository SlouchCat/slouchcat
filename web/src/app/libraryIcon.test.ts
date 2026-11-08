import { describe, expect, it } from 'vitest';
import { libraryIcon } from './libraryIcon';

const library = { id: 1, name: 'Bibliothek', itemCount: 3 };

describe('libraryIcon', () => {
  it('gives a video library with only series folders the television', () => {
    expect(libraryIcon({ ...library, kind: 'video', contents: ['series'] })).toBe('tv');
  });

  it('gives any other video library the film strip', () => {
    expect(libraryIcon({ ...library, kind: 'video', contents: ['movie'] })).toBe('film');
    expect(libraryIcon({ ...library, kind: 'video', contents: ['movie', 'series'] })).toBe('film');
    expect(libraryIcon({ ...library, kind: 'video', contents: [] })).toBe('film');
  });

  it('gives audiobooks the book and podcasts the microphone', () => {
    expect(libraryIcon({ ...library, kind: 'audiobooks', contents: ['audiobook'] })).toBe('book');
    expect(libraryIcon({ ...library, kind: 'podcasts', contents: [] })).toBe('mic');
  });
});
