import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { OptionLinkRow, OptionList, OptionMoreRow, OptionRow, OptionSection } from './OptionRow';

function renderList(selectedId: string | null = 'de') {
  const onSelect = vi.fn();
  render(
    <OptionList label="Untertitel" selectedId={selectedId} onSelect={onSelect}>
      <OptionRow id="off" label="Aus" />
      <OptionRow id="de" label="Deutsch" detail="SRT" marks={['default']} />
      <OptionRow id="de-forced" label="Deutsch" detail="SRT" marks={['forced']} />
      <OptionRow id="en-pgs" label="Englisch" detail="PGS" marks={['burnIn']} />
      <OptionRow id="en-sdh" label="Englisch SDH" marks={['sdh']} />
      <OptionRow id="dts" label="Deutsch · DTS · 5.1" marks={['default', 'convert']} />
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
    expect(option('Englisch')).toHaveAccessibleDescription('PGSBild-Untertitel, wird eingebrannt');
    expect(option('Englisch SDH')).toHaveAccessibleDescription('SDH');
    expect(option('Deutsch · DTS · 5.1')).toHaveAccessibleDescription('Standardwird umgewandelt');
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

  it('marks a disabled row as such and never chooses it', async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(
      <OptionList label="Qualität" selectedId="auto" onSelect={onSelect}>
        <OptionRow id="auto" label="Automatisch" />
        <OptionRow id="360p" label="360p" isDisabled />
      </OptionList>,
    );
    expect(option('360p')).toHaveAttribute('aria-disabled', 'true');
    await user.click(option('360p'));
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('starts without a chosen row when none is given', () => {
    renderList(null);
    expect(screen.queryByRole('option', { selected: true })).toBeNull();
  });

  it('leaves letters to the player: M on «Mandarin» reaches window (11.2)', async () => {
    const outside = vi.fn();
    window.addEventListener('keydown', outside);
    try {
      const user = userEvent.setup();
      render(
        <OptionList label="Ton" selectedId="de" onSelect={vi.fn()}>
          <OptionRow id="de" label="Deutsch" />
          <OptionRow id="zh" label="Mandarin" />
          <OptionRow id="ko" label="Koreanisch" />
        </OptionList>,
      );
      act(() => option('Deutsch').focus());
      await user.keyboard('m');
      await user.keyboard('k');
      // No type to select: the focus stays, both keys went on to the page.
      expect(option('Deutsch')).toHaveFocus();
      expect(outside.mock.calls.map(([event]) => (event as KeyboardEvent).key)).toEqual(['m', 'k']);
    } finally {
      window.removeEventListener('keydown', outside);
    }
  });
});

/** «Deine Sprachen», then «Weitere Sprachen (2)» behind a row that shows them (11.10). */
function Grouped({ onSelect }: { onSelect: (id: string) => void }) {
  const [more, setMore] = useState(false);
  return (
    <OptionList label="Untertitel" selectedId="de" onSelect={onSelect} actionIds={['more']}>
      <OptionSection title="Deine Sprachen">
        <OptionRow id="de" label="Deutsch" />
        <OptionRow id="en" label="Englisch" />
      </OptionSection>
      <OptionSection title="Weitere Sprachen">
        <OptionMoreRow
          id="more"
          label="Weitere Sprachen (2)"
          isExpanded={more}
          onToggle={() => setMore((shown) => !shown)}
        />
        {more && <OptionRow id="fi" label="Finnisch" />}
        {more && <OptionRow id="it" label="Italienisch" />}
      </OptionSection>
    </OptionList>
  );
}

describe('OptionSection and OptionMoreRow (concept 13, 11.10)', () => {
  it('runs ↑/↓ across both groups of one list; the more row shows rows and is never chosen', async () => {
    const onSelect = vi.fn();
    const user = userEvent.setup();
    render(<Grouped onSelect={onSelect} />);
    expect(screen.getByRole('group', { name: 'Deine Sprachen' })).toBeInTheDocument();
    expect(screen.queryByRole('option', { name: 'Finnisch' })).toBeNull();
    act(() => option('Deutsch').focus());
    await user.keyboard('{ArrowDown}{ArrowDown}');
    const more = option('Weitere Sprachen (2)');
    expect(more).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(onSelect).not.toHaveBeenCalled();
    expect(more).toHaveAttribute('aria-selected', 'false');
    await user.keyboard('{ArrowDown}');
    expect(option('Finnisch')).toHaveFocus();
    await user.keyboard('{ArrowDown}{Enter}');
    expect(onSelect).toHaveBeenCalledWith('it');
    // Back up across the groups to the first one; a click on the more row hides the rows again.
    await user.keyboard('{ArrowUp}{ArrowUp}{ArrowUp}');
    expect(option('Englisch')).toHaveFocus();
    await user.click(option('Weitere Sprachen (2)'));
    expect(screen.queryByRole('option', { name: 'Finnisch' })).toBeNull();
    expect(onSelect).toHaveBeenCalledTimes(1);
  });
});

describe('OptionLinkRow (concept 13, 11.2: the gear)', () => {
  it('opens its page on Enter or a click, reads its value and is never chosen', async () => {
    const onSelect = vi.fn();
    const onOpen = vi.fn();
    const user = userEvent.setup();
    render(
      <OptionList
        label="Einstellungen"
        selectedId={null}
        onSelect={onSelect}
        actionIds={['quality', 'speed']}
      >
        <OptionLinkRow id="quality" label="Qualität" value="Automatisch" onOpen={onOpen} />
        <OptionLinkRow id="speed" label="Geschwindigkeit" value="1×" onOpen={vi.fn()} />
      </OptionList>,
    );
    const quality = option('Qualität');
    expect(quality).toHaveAccessibleDescription('Automatisch');
    act(() => quality.focus());
    await user.keyboard('{Enter}');
    expect(onOpen).toHaveBeenCalledTimes(1);
    await user.keyboard('{ArrowDown}');
    expect(option('Geschwindigkeit')).toHaveFocus();
    await user.click(quality);
    expect(onOpen).toHaveBeenCalledTimes(2);
    expect(onSelect).not.toHaveBeenCalled();
    expect(quality).toHaveAttribute('aria-selected', 'false');
  });
});
