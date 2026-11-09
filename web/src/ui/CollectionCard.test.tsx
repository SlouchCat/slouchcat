import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { CollectionCard } from './CollectionCard';

const card = (props: Parameters<typeof CollectionCard>[0]) => render(<CollectionCard {...props} />);

describe('CollectionCard (concept 17 D-5)', () => {
  it('shows the collection poster before the cover title poster', () => {
    const { container } = card({
      href: '/collections/c1',
      name: 'Alien-Reihe',
      imageUrl: '/own.jpg',
      posterUrl: '/cover-w400.jpg',
      posterSmallUrl: '/cover-w200.jpg',
    });
    const image = container.querySelector('img');
    expect(image).toHaveAttribute('src', '/own.jpg');
    expect(image).not.toHaveAttribute('srcset');
  });

  it('shows the cover title poster without a collection poster', () => {
    const { container } = card({
      href: '/collections/c1',
      name: 'Alien-Reihe',
      posterUrl: '/cover-w400.jpg',
      posterSmallUrl: '/cover-w200.jpg',
    });
    const image = container.querySelector('img');
    expect(image).toHaveAttribute('src', '/cover-w400.jpg');
    expect(image).toHaveAttribute('srcset', '/cover-w200.jpg 200w, /cover-w400.jpg 400w');
  });

  it('falls back from the collection poster to the cover poster, then to the neutral tile', () => {
    const { container } = card({
      href: '/collections/c1',
      name: 'Alien-Reihe',
      imageUrl: '/own.jpg',
      posterUrl: '/cover-w400.jpg',
      posterSmallUrl: '/cover-w200.jpg',
    });
    const first = container.querySelector('img');
    if (first) fireEvent.error(first);
    const second = container.querySelector('img');
    expect(second).toHaveAttribute('src', '/cover-w400.jpg');
    expect(second).toHaveAttribute('srcset', '/cover-w200.jpg 200w, /cover-w400.jpg 400w');
    if (second) fireEvent.error(second);
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('svg')).not.toBeNull();
    expect(screen.getByRole('link', { name: /Alien-Reihe/ })).toHaveAttribute(
      'href',
      '/collections/c1',
    );
  });

  it('falls back to the neutral tile when the collection poster fails and there is no cover', () => {
    const { container } = card({ href: '/collections/c1', name: 'Meine', imageUrl: '/own.jpg' });
    const image = container.querySelector('img');
    if (image) fireEvent.error(image);
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('svg')).not.toBeNull();
  });
});
