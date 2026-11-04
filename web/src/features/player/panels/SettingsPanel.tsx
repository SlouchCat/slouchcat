import { useTranslation } from 'react-i18next';
import { AriaButton, OptionLinkRow, OptionList, OptionRow, PlayerPanel, Switch } from '@/ui/aria';
import { RATE_MAX, RATE_MIN, RATE_STEP } from '../keymap';
import type { SubtitleStyle } from '../useSubtitleStyle';
import styles from './SettingsPanel.module.css';
import { PANEL_IDS } from './panelIds';
import { SubtitleStyleOptions } from './SubtitleStyleOptions';

/** The pages of the gear panel (11.2). */
export type SettingsPage =
  'main' | 'quality' | 'version' | 'speed' | 'subtitleStyle' | 'sound' | 'chapters';

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
  /**
   * What this device remembered for the choice (V-17: «Gemerkt: 720p»), shown under the list
   * with a button that forgets it; `note` says what the memory does to other titles.
   */
  remembered?: { label: string; note?: string; resetLabel: string; onReset: () => void };
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
  /** The chapters: on a phone they are a page of the gear, not a tab (11.2). */
  chapters?: SettingsChoice;
  rate: number;
  onRate: (rate: number) => void;
  /** The stream info over the video (key I). */
  streamInfo?: { open: boolean; onToggle: () => void };
  /** The subtitle style of this device (V-6). */
  subtitleStyle: { value: SubtitleStyle; onChange: (patch: Partial<SubtitleStyle>) => void };
  /** Night mode (V-16); none where Web Audio is not offered (Safari, 11.15). */
  nightMode?: { on: boolean; onChange: (on: boolean) => void };
}

/**
 * The gear panel (11.2): a first page with quality, version, speed, the subtitle style and the sound
 * options (night mode), each a row that leads to a
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
  chapters,
  rate,
  onRate,
  streamInfo,
  subtitleStyle,
  nightMode,
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
    <PlayerPanel id={PANEL_IDS.settings} title={title} onClose={onClose} onBack={back}>
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
      {choice.remembered && (
        <div className={styles.remembered}>
          <p className={styles.hint}>{choice.remembered.label}</p>
          {choice.remembered.note && <p className={styles.hint}>{choice.remembered.note}</p>}
          <AriaButton
            variant="secondary"
            onPress={(event) => {
              // The button goes away with the memory: the focus moves to the chosen row first
              // (or the first row that can be chosen, when none is chosen), so it stays in the panel.
              const dialog = event.target.closest('[role="dialog"]');
              (
                dialog?.querySelector<HTMLElement>('[role="option"][aria-selected="true"]') ??
                dialog?.querySelector<HTMLElement>('[role="option"]:not([aria-disabled="true"])')
              )?.focus();
              choice.remembered?.onReset();
            }}
          >
            {choice.remembered.resetLabel}
          </AriaButton>
        </div>
      )}
    </PlayerPanel>
  );

  if (page === 'quality' && quality) return choicePage(t('player.settings.quality'), quality);
  if (page === 'version' && version) return choicePage(t('player.settings.version'), version);
  if (page === 'chapters' && chapters) return choicePage(t('player.settings.chapters'), chapters);
  if (page === 'speed') {
    return choicePage(t('player.settings.speed'), {
      options: RATES.map((value) => ({ id: String(value), label: rateText(value) })),
      selectedId: String(rate),
      onSelect: (id) => onRate(Number(id)),
    });
  }

  if (page === 'subtitleStyle') {
    return (
      <PlayerPanel
        id={PANEL_IDS.settings}
        title={t('player.subtitleStyle.title')}
        onClose={onClose}
        onBack={back}
      >
        <SubtitleStyleOptions value={subtitleStyle.value} onChange={subtitleStyle.onChange} />
      </PlayerPanel>
    );
  }

  if (page === 'sound' && nightMode) {
    return (
      <PlayerPanel
        id={PANEL_IDS.settings}
        title={t('player.settings.sound')}
        onClose={onClose}
        onBack={back}
      >
        <div className={styles.sound}>
          <Switch isSelected={nightMode.on} onChange={nightMode.onChange}>
            {t('player.settings.nightMode')}
          </Switch>
          <p className={styles.hint}>{t('player.settings.nightModeHint')}</p>
        </div>
      </PlayerPanel>
    );
  }

  const rows = [
    ...(quality ? (['quality'] as const) : []),
    ...(version ? (['version'] as const) : []),
    ...(chapters ? (['chapters'] as const) : []),
    'speed' as const,
    'subtitleStyle' as const,
    ...(nightMode ? (['sound'] as const) : []),
  ];
  const values = {
    quality: labelOf(quality),
    version: labelOf(version),
    chapters: labelOf(chapters),
    speed: rateText(rate),
    subtitleStyle: undefined,
    sound: nightMode?.on ? t('player.settings.nightModeOn') : undefined,
  };
  return (
    <PlayerPanel id={PANEL_IDS.settings} title={t('player.settings.title')} onClose={onClose}>
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
