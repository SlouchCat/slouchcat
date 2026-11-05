import { describe, expect, it } from 'vitest';
import { apiUrl } from './client';

describe('apiUrl', () => {
  it('keeps a URL that already carries the API base (Chapter.imageUrl), never doubles it', () => {
    const chapterImage = '/api/v1/items/01J/chapters/2/image?mediaFileId=01K&v=1730700000';
    expect(apiUrl(chapterImage)).toBe(chapterImage);
  });

  it('puts the base in front of a URL relative to it', () => {
    expect(apiUrl('/items/01J/images/poster')).toBe('/api/v1/items/01J/images/poster');
    expect(apiUrl('items/01J/images/poster')).toBe('/api/v1/items/01J/images/poster');
  });

  it('keeps a URL with an origin', () => {
    expect(apiUrl('https://example.org/a.webp')).toBe('https://example.org/a.webp');
  });
});
