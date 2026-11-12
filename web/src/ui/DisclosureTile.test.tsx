import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { DisclosureTile } from './DisclosureTile';

function Harness({ badge }: { badge?: string }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <DisclosureTile
        title="Europa"
        subtitle="51 Länder · 26.395 Sender"
        badge={badge}
        isExpanded={open}
        controls={open ? 'countries' : undefined}
        onToggle={() => setOpen(!open)}
      />
      {open && <div id="countries">Länder in Europa</div>}
    </>
  );
}

describe('DisclosureTile', () => {
  it('is one button that opens its region by click, Enter and Space', async () => {
    const user = userEvent.setup();
    render(<Harness badge="1 gewählt" />);
    const tile = screen.getByRole('button', { name: /^Europa/ });
    expect(tile).toHaveAttribute('type', 'button');
    expect(tile).toHaveAttribute('aria-expanded', 'false');
    expect(tile).not.toHaveAttribute('aria-controls');
    // Name, secondary line and badge are its name, in reading order; the chevron is decoration.
    expect(tile).toHaveAccessibleName('Europa 51 Länder · 26.395 Sender 1 gewählt');
    await user.click(tile);
    expect(tile).toHaveAttribute('aria-expanded', 'true');
    expect(tile).toHaveAttribute('aria-controls', 'countries');
    expect(document.getElementById('countries')).toBeInTheDocument();
    await user.keyboard('{Enter}');
    expect(tile).toHaveAttribute('aria-expanded', 'false');
    await user.keyboard(' ');
    expect(tile).toHaveAttribute('aria-expanded', 'true');
    expect(tile).toHaveFocus();
  });

  it('leaves the badge out when there is none', () => {
    render(<Harness />);
    expect(screen.getByRole('button')).toHaveAccessibleName('Europa 51 Länder · 26.395 Sender');
  });
});
