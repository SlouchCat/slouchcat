import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { OptionList, OptionRow } from './OptionRow';

function renderList(selectedId: string | null = 'de') {
  const onSelect = vi.fn();
  render(
    <OptionList label="Untertitel" selectedId={selectedId} onSelect={onSelect}>
      <OptionRow id="off" label="Aus" />
      <OptionRow id="de" label="Deutsch" detail="SRT" marks={['default']} />
      <OptionRow id="de-forced" label="Deutsch" detail="SRT" marks={['forced']} />
      <OptionRow id="en-pgs" label="Englisch" detail="PGS" marks={['burnIn']} />
    </OptionList>,
  );
  return { onSelect, user: userEvent.setup() };
}

const option = (name: string, description?: string) =>
  screen
    .getAllByRole('option', { name })
    .find((element) => description === undefined || element.textContent?.includes(description))!;

describe('OptionRow and OptionList (concept 13, V-5)', () => {
  it('names the list and marks the chosen row with a tick', () => {
    renderList();
    expect(screen.getByRole('listbox', { name: 'Untertitel' })).toBeInTheDocument();
    const chosen = option('Deutsch', 'Standard');
    expect(chosen).toHaveAttribute('aria-selected', 'true');
    expect(chosen.querySelector('svg')).not.toBeNull();
    expect(option('Aus')).toHaveAttribute('aria-selected', 'false');
    expect(option('Aus').querySelector('svg')).toBeNull();
  });

  it('reads detail and marks as the second line', () => {
    renderList();
    expect(option('Deutsch', 'Standard')).toHaveAccessibleDescription('SRTStandard');
    expect(option('Deutsch', 'erzwungen')).toHaveAccessibleDescription('SRTerzwungen');
    expect(option('Englisch')).toHaveAccessibleDescription('PGSwird eingebrannt');
    expect(option('Aus')).not.toHaveAccessibleDescription();
  });

  it('moves with ↑/↓ without choosing; Enter chooses', async () => {
    const { onSelect, user } = renderList();
    act(() => option('Deutsch', 'Standard').focus());
    await user.keyboard('{ArrowDown}');
    expect(option('Deutsch', 'erzwungen')).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(option('Englisch')).toHaveFocus();
    await user.keyboard('{ArrowUp}{ArrowUp}{ArrowUp}');
    expect(option('Aus')).toHaveFocus();
    expect(onSelect).not.toHaveBeenCalled();
    await user.keyboard('{Enter}');
    expect(onSelect).toHaveBeenCalledWith('off');
  });

  it('chooses with a click; the chosen row cannot be unchosen', async () => {
    const { onSelect, user } = renderList();
    await user.click(option('Englisch'));
    expect(onSelect).toHaveBeenLastCalledWith('en-pgs');
    await user.click(option('Deutsch', 'Standard'));
    expect(onSelect).toHaveBeenCalledTimes(1);
  });

  it('starts without a chosen row when none is given', () => {
    renderList(null);
    expect(screen.queryByRole('option', { selected: true })).toBeNull();
  });
});
