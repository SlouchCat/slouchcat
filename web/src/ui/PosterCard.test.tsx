import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Button } from './Button';
import { PosterCard } from './PosterCard';

const pen = (
  <Button size="sm" icon="pen" iconOnly overlay aria-label="Glut: Metadaten bearbeiten" />
);
const mark = (pressed: boolean) => (
  <Button
    size="sm"
    icon="bookmark"
    iconOnly
    overlay
    aria-pressed={pressed}
    aria-label="Glut merken"
  />
);

describe('PosterCard', () => {
  it('puts link, pen and mark in this order into one cell, outside the link', () => {
    render(<PosterCard href="/items/1" title="Glut" tool={pen} mark={mark(false)} />);
    const link = screen.getByRole('link', { name: 'Glut' });
    const toolButton = screen.getByRole('button', { name: 'Glut: Metadaten bearbeiten' });
    const markButton = screen.getByRole('button', { name: 'Glut merken' });
    const cell = link.parentElement;
    expect(cell).toContainElement(toolButton);
    expect(cell).toContainElement(markButton);
    expect(link).not.toContainElement(markButton);
    const follows = (first: Element, second: Element) =>
      Boolean(first.compareDocumentPosition(second) & Node.DOCUMENT_POSITION_FOLLOWING);
    expect(follows(link, toolButton)).toBe(true);
    expect(follows(toolButton, markButton)).toBe(true);
  });

  it('draws a mark without a pen in a cell of its own', () => {
    render(<PosterCard href="/items/1" title="Glut" mark={mark(true)} />);
    const link = screen.getByRole('link', { name: 'Glut' });
    expect(link.parentElement).toContainElement(
      screen.getByRole('button', { name: 'Glut merken' }),
    );
    expect(screen.getAllByRole('button')).toHaveLength(1);
  });

  it('stays a bare link without pen and mark', () => {
    const { container } = render(<PosterCard href="/items/1" title="Glut" />);
    expect(container.firstElementChild).toBe(screen.getByRole('link', { name: 'Glut' }));
  });

  it('a square cover sits in the 2:3 frame on its blurred copy, or in a square frame of its own', () => {
    const { rerender } = render(
      <PosterCard href="/items/1" title="Glut" posterUrl="/p.jpg" shape="square" />,
    );
    const link = () => screen.getByRole('link', { name: 'Glut' });
    expect(link().querySelectorAll('img')).toHaveLength(2);
    expect(link()).not.toHaveClass('own');
    // Concept 19 C: the library overview of audio.
    rerender(
      <PosterCard href="/items/1" title="Glut" posterUrl="/p.jpg" shape="square" frame="own" />,
    );
    const images = link().querySelectorAll('img');
    expect(images).toHaveLength(1);
    expect(images[0]).toHaveAttribute('height', '400');
    expect(link()).toHaveClass('own');
    // A poster ignores it.
    rerender(<PosterCard href="/items/1" title="Glut" posterUrl="/p.jpg" frame="own" />);
    expect(link()).not.toHaveClass('own');
    expect(link().querySelector('img')).toHaveAttribute('height', '600');
  });
});
