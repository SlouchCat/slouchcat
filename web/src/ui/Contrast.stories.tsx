import type { Meta, StoryObj } from '@storybook/react-vite';
import type { CSSProperties } from 'react';
import { Button } from './Button';
import { CheckboxGroup } from './CheckboxGroup';
import { Chip } from './Chip';
import { LetterBar } from './LetterBar';
import { LockToggle } from './LockToggle';
import { RadioGroup } from './RadioGroup';
import { RoundAction } from './RoundAction';
import { SearchField } from './SearchField';
import { SelectCheck } from './SelectCheck';
import { SelectField } from './SelectField';
import { Switch } from './Switch';
import { Tabs } from './Tabs';
import { Text } from './Text';
import { TextField } from './TextField';

/*
 * The manual check of concept 24 (6.1, «von Hand»): every control in every state, one story per
 * surface it can stand on, in the viewer's and in the administration's look. Z-3: each control
 * stands out from its ground at 3:1 by outline or fill; Z-5: a disabled control is readable and
 * recognisable by its dashed outline without fill. Its screenshots are part of the comparisons
 * (components.spec.ts, one 1280 x 720 viewport per story).
 */

type Surface = 'bg-base' | 'bg-elevated' | 'surface-1' | 'surface-2' | 'surface-3';

const ground = (surface: Surface): CSSProperties => ({
  display: 'grid',
  gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
  gap: 'var(--sc-space-6)',
  padding: 'var(--sc-space-4)',
  border: '1px solid var(--sc-color-border-frame)',
  borderRadius: 'var(--sc-radius-lg)',
  backgroundColor: `var(--sc-color-${surface})`,
});

const column: CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 'var(--sc-space-4)',
};

const row: CSSProperties = {
  display: 'flex',
  flexWrap: 'wrap',
  gap: 'var(--sc-space-2) var(--sc-space-3)',
  alignItems: 'center',
};

const noop = () => undefined;
const segments = [
  { value: 'on', label: 'Ein' },
  { value: 'off', label: 'Aus' },
  { value: 'never', label: 'Gesperrt', isDisabled: true },
];
const sources = [
  { value: 'tmdb', label: 'TMDB' },
  { value: 'tvdb', label: 'TVDB' },
];

function Controls({ surface }: { surface: Surface }) {
  return (
    <section style={ground(surface)} aria-label={surface}>
      <div style={column}>
        <Text tone="secondary">{surface}</Text>
        <div style={row}>
          <Button variant="primary">Speichern</Button>
          <Button variant="secondary">Erneut scannen</Button>
          <Button variant="ghost">Verwerfen</Button>
          <Button variant="danger">Entfernen</Button>
          <Button variant="dangerQuiet">Löschen</Button>
        </div>
        <div style={row}>
          <Button variant="primary" isDisabled>
            Speichern
          </Button>
          <Button variant="secondary" isDisabled>
            Zuordnen
          </Button>
          <Button variant="ghost" isDisabled>
            Verwerfen
          </Button>
        </div>
        <Text tone="muted">Gesperrt: nur für einen Film oder eine Serie.</Text>
        <div style={row}>
          <Chip>Filme</Chip>
          <Chip tone="outline">Neu</Chip>
          <Chip tone="info">Dateisystem nfs</Chip>
          <Chip tone="success">Aktuell</Chip>
          <Chip tone="warning">Veraltet</Chip>
          <Chip tone="danger">Fehler</Chip>
        </div>
        <div style={row}>
          <TextField label="Name" defaultValue="Filme" />
          <TextField label="Pfad" mono defaultValue="/media/nas" isDisabled />
          <TextField label="Port" defaultValue="0" isInvalid />
        </div>
        <div style={row}>
          <SelectField
            label="Sortierung"
            value="title"
            options={[
              { value: 'title', label: 'Titel' },
              { value: 'year', label: 'Jahr' },
            ]}
            onChange={noop}
          />
          <SearchField label="Suchen" value="" placeholder="Titel suchen" onChange={noop} />
        </div>
      </div>
      <div style={column}>
        <Tabs
          label="Bereiche"
          selectedKey="files"
          tabs={[
            { id: 'files', label: 'Dateien', content: null },
            { id: 'images', label: 'Bilder', content: null },
            { id: 'cast', label: 'Besetzung', content: null },
          ]}
        />
        <RadioGroup label="Untertitel" defaultValue="on" options={segments} />
        <div style={row}>
          <CheckboxGroup label="Quellen" defaultValue={['tmdb']} options={sources} />
          <CheckboxGroup label="Gesperrt" isDisabled defaultValue={['tmdb']} options={sources} />
          <div style={column}>
            <Switch defaultSelected>Ein</Switch>
            <Switch>Aus</Switch>
            <Switch isDisabled>Gesperrt</Switch>
            <Switch isDisabled defaultSelected>
              Gesperrt, an
            </Switch>
          </div>
        </div>
        <div style={row}>
          <SelectCheck label="Gewählt" isSelected onChange={noop} />
          <SelectCheck label="Frei" isSelected={false} onChange={noop} />
          <SelectCheck label="Gesperrt" isSelected={false} onChange={noop} isDisabled />
          <SelectCheck label="Gesperrt, gewählt" isSelected onChange={noop} isDisabled />
          <LockToggle
            label="Titel sperren"
            isLocked
            onChange={noop}
            lockedText="Gesperrt"
            freeText="Frei"
          />
          <LockToggle
            label="Jahr sperren"
            isLocked={false}
            onChange={noop}
            lockedText="Gesperrt"
            freeText="Frei"
            isDisabled
          />
        </div>
        <div style={row}>
          <RoundAction label="Merken" icon="plus" onPress={noop} />
          <RoundAction label="Gesehen" icon="check" pressed onPress={noop} />
          <RoundAction label="Trailer" icon="play" isDisabled onPress={noop} />
          <LetterBar
            label="Buchstaben"
            letters={['#', 'A', 'B', 'C']}
            available={['A', 'C']}
            current="A"
            onJump={noop}
          />
        </div>
      </div>
    </section>
  );
}

const meta = {
  title: 'UI/Kontrast',
  component: Controls,
  args: { surface: 'bg-base' },
} satisfies Meta<typeof Controls>;

export default meta;
type Story = StoryObj<typeof meta>;

const admin = { density: 'compact' };

export const OnBase: Story = {};
export const OnElevated: Story = { args: { surface: 'bg-elevated' } };
export const OnSurface1: Story = { args: { surface: 'surface-1' } };
export const OnSurface2: Story = { args: { surface: 'surface-2' } };
export const OnSurface3: Story = { args: { surface: 'surface-3' } };

/** The same in the administration's look and compact density (admin-look.css, ADR 0020). */
export const AdminOnBase: Story = { globals: admin };
export const AdminOnElevated: Story = { args: { surface: 'bg-elevated' }, globals: admin };
export const AdminOnSurface1: Story = { args: { surface: 'surface-1' }, globals: admin };
export const AdminOnSurface2: Story = { args: { surface: 'surface-2' }, globals: admin };
export const AdminOnSurface3: Story = { args: { surface: 'surface-3' }, globals: admin };
