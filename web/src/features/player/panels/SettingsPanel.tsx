import { useTranslation } from 'react-i18next';
import { OptionLinkRow, OptionList, OptionRow, PlayerPanel, Switch } from '@/ui/aria';
import { RATE_MAX, RATE_MIN, RATE_STEP } from '../keymap';

/** The pages of the gear panel (11.2). */
export type SettingsPage = 'main' | 'quality' | 'version' | 'speed';

/** One row of a page that chooses: a quality rung, a version. */
export interface SettingsOption {
  id: string;
  label: string;
  /** Second line: the bit rate of a rung. */
  detail?: string;
  disabled?: boolean;
}

/** A choice behind a row of the first page. */
export interface SettingsChoice {
  options: readonly SettingsOption[];
  selectedId: string;
  onSelect: (id: string) => void;
}

/** The speeds of the speed page: 0.5× to 2× in steps of 0.25 (V-7, like [ and ]). */
export const RATES: readonly number[] = Array.from(
  { length: Math.round((RATE_MAX - RATE_MIN) / RATE_STEP) + 1 },
  (_, step) => RATE_MIN + step * RATE_STEP,
);

interface SettingsPanelProps {
  page: SettingsPage;
  /**
   * Opened straight on a page (Q opens the quality): no way up to the first page, Backspace closes
   * like Escape.
   */
  direct: boolean;
  onPage: (page: SettingsPage) => void;
  onClose: () => void;
  /** The quality rungs; none where the page cannot switch (offline). */
  quality?: SettingsChoice;
  /** The versions of the title, only with two or more files. */
  version?: SettingsChoice;
  rate: number;
  onRate: (rate: number) => void;
  /** The stream info over the video (key I). */
  streamInfo?: { open: boolean; onToggle: () => void };
}

/**
 * The gear panel (11.2): a first page with quality, version and speed, each a row that leads to a
 * page of its own (PlayerPanel's back button and Backspace go up again), and the stream info as a
 * switch. The quality and the version are the server's: a choice starts a new session.
 */
export function SettingsPanel({
  page,
  direct,
  onPage,
  onClose,
  quality,
  version,
  rate,
  onRate,
  streamInfo,
}: SettingsPanelProps) {
  const { t, i18n } = useTranslation();
  const rateText = (value: number) =>
    t('player.settings.rateValue', {
      rate: new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(value),
    });
  const labelOf = (choice: SettingsChoice | undefined) =>
    choice?.options.find((option) => option.id === choice.selectedId)?.label;
  const back = direct ? undefined : () => onPage('main');

  const choicePage = (title: string, choice: SettingsChoice) => (
    <PlayerPanel title={title} onClose={onClose} onBack={back}>
      {/* A list of its own per page (key): the focused row of the page before goes with it, and
          the panel moves the focus to the chosen row of this one. */}
      <OptionList
        key={title}
        label={title}
        selectedId={choice.selectedId}
        onSelect={choice.onSelect}
      >
        {choice.options.map((option) => (
          <OptionRow
            key={option.id}
            id={option.id}
            label={option.label}
            detail={option.detail}
            isDisabled={option.disabled}
          />
        ))}
      </OptionList>
    </PlayerPanel>
  );

  if (page === 'quality' && quality) return choicePage(t('player.settings.quality'), quality);
  if (page === 'version' && version) return choicePage(t('player.settings.version'), version);
  if (page === 'speed') {
    return choicePage(t('player.settings.speed'), {
      options: RATES.map((value) => ({ id: String(value), label: rateText(value) })),
      selectedId: String(rate),
      onSelect: (id) => onRate(Number(id)),
    });
  }

  const rows = [
    ...(quality ? (['quality'] as const) : []),
    ...(version ? (['version'] as const) : []),
    'speed' as const,
  ];
  const values = { quality: labelOf(quality), version: labelOf(version), speed: rateText(rate) };
  return (
    <PlayerPanel title={t('player.settings.title')} onClose={onClose}>
      <OptionList
        key="main"
        label={t('player.settings.title')}
        selectedId={null}
        onSelect={() => undefined}
        actionIds={rows}
      >
        {rows.map((row) => (
          <OptionLinkRow
            key={row}
            id={row}
            label={t(`player.settings.${row}`)}
            value={values[row]}
            onOpen={() => onPage(row)}
          />
        ))}
      </OptionList>
      {streamInfo && (
        <Switch isSelected={streamInfo.open} onChange={streamInfo.onToggle}>
          {t('player.settings.streamInfo')}
        </Switch>
      )}
    </PlayerPanel>
  );
}
