import { useId, useState, type KeyboardEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { AriaButton, OptionList, OptionRow } from '@/ui/aria';
import { SUBTITLE_OFFSET_STEP_MS, type SubtitleStyle } from '../useSubtitleStyle';
import styles from './SubtitleStyleOptions.module.css';

interface SubtitleStyleOptionsProps {
  value: SubtitleStyle;
  onChange: (patch: Partial<SubtitleStyle>) => void;
}

/** The rows of each list and their texts, in the order shown. */
const GROUPS = [
  {
    group: 'size',
    title: 'player.subtitleStyle.size.title',
    rows: [
      ['s', 'player.subtitleStyle.size.s'],
      ['m', 'player.subtitleStyle.size.m'],
      ['l', 'player.subtitleStyle.size.l'],
      ['xl', 'player.subtitleStyle.size.xl'],
    ],
  },
  {
    group: 'background',
    title: 'player.subtitleStyle.background.title',
    rows: [
      ['none', 'player.subtitleStyle.background.none'],
      ['shadow', 'player.subtitleStyle.background.shadow'],
      ['translucent', 'player.subtitleStyle.background.translucent'],
      ['box', 'player.subtitleStyle.background.box'],
    ],
  },
  {
    group: 'color',
    title: 'player.subtitleStyle.color.title',
    rows: [
      ['white', 'player.subtitleStyle.color.white'],
      ['yellow', 'player.subtitleStyle.color.yellow'],
    ],
  },
  {
    group: 'position',
    title: 'player.subtitleStyle.position.title',
    rows: [
      ['bottom', 'player.subtitleStyle.position.bottom'],
      ['raised', 'player.subtitleStyle.position.raised'],
    ],
  },
] as const satisfies readonly {
  group: keyof Omit<SubtitleStyle, 'offsetMs'>;
  title: string;
  rows: readonly (readonly [string, string])[];
}[];

/** The buttons of the offset group in their order. */
const OFFSET_KEYS = ['earlier', 'later', 'reset'] as const;
type OffsetKey = (typeof OFFSET_KEYS)[number];

/**
 * The subtitle style (V-6) as the content of a panel page: four short lists (size, background,
 * colour, position), each one Tab stop with ↑/↓ and Enter like every other list of the player, and
 * the offset in steps of 0.1 s. Used by the subtitle panel and the gear.
 */
export function SubtitleStyleOptions({ value, onChange }: SubtitleStyleOptionsProps) {
  const { t, i18n } = useTranslation();
  const offsetId = useId();
  const seconds = new Intl.NumberFormat(i18n.language, {
    minimumFractionDigits: 1,
    maximumFractionDigits: 1,
    signDisplay: 'exceptZero',
  }).format(value.offsetMs / 1000);
  const step = (direction: -1 | 1) =>
    onChange({ offsetMs: value.offsetMs + direction * SUBTITLE_OFFSET_STEP_MS });
  const [offsetStop, setOffsetStop] = useState<OffsetKey>('earlier');
  const onOffsetKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
    const from = OFFSET_KEYS.indexOf(offsetStop);
    const to =
      event.key === 'ArrowLeft'
        ? from - 1
        : event.key === 'ArrowRight'
          ? from + 1
          : event.key === 'Home'
            ? 0
            : event.key === 'End'
              ? OFFSET_KEYS.length - 1
              : null;
    if (to === null) return;
    event.preventDefault();
    const key = OFFSET_KEYS[Math.min(OFFSET_KEYS.length - 1, Math.max(0, to))]!;
    event.currentTarget.querySelector<HTMLElement>(`[data-offset-key="${key}"] button`)?.focus();
  };

  return (
    <div className={styles.options}>
      {GROUPS.map(({ group, title, rows }) => (
        <section key={group} className={styles.group}>
          <h3 className={styles.heading}>{t(title)}</h3>
          <OptionList
            label={t(title)}
            selectedId={value[group]}
            onSelect={(id) => onChange({ [group]: id })}
          >
            {rows.map(([id, label]) => (
              <OptionRow key={id} id={id} label={t(label)} />
            ))}
          </OptionList>
        </section>
      ))}
      <section className={styles.group}>
        <h3 className={styles.heading} id={offsetId}>
          {t('player.subtitleStyle.offset.title')}
        </h3>
        {/* The offset is read with the buttons: earlier, the value, later, back to 0. The group
            is one stop with ←/→ between its buttons (a remote has no Tab, 11.2); «Zurücksetzen»
            stays pressable at 0, so the focus never falls off a button that went off. */}
        {/* eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions */}
        <div
          className={styles.offset}
          role="group"
          aria-labelledby={offsetId}
          onKeyDown={onOffsetKeyDown}
          onFocus={(event) => {
            const key = (event.target as HTMLElement)
              .closest('[data-offset-key]')
              ?.getAttribute('data-offset-key');
            if (key) setOffsetStop(key as OffsetKey);
          }}
        >
          <span className={styles.offsetItem} data-offset-key="earlier">
            <AriaButton
              variant="secondary"
              onPress={() => step(-1)}
              excludeFromTabOrder={offsetStop !== 'earlier'}
            >
              {t('player.subtitleStyle.offset.earlier')}
            </AriaButton>
          </span>
          <output className={styles.value} aria-live="polite">
            {t('player.subtitleStyle.offset.value', { seconds })}
          </output>
          <span className={styles.offsetItem} data-offset-key="later">
            <AriaButton
              variant="secondary"
              onPress={() => step(1)}
              excludeFromTabOrder={offsetStop !== 'later'}
            >
              {t('player.subtitleStyle.offset.later')}
            </AriaButton>
          </span>
          <span className={styles.offsetItem} data-offset-key="reset">
            <AriaButton
              variant="ghost"
              onPress={() => {
                if (value.offsetMs !== 0) onChange({ offsetMs: 0 });
              }}
              excludeFromTabOrder={offsetStop !== 'reset'}
            >
              {t('player.subtitleStyle.offset.reset')}
            </AriaButton>
          </span>
        </div>
      </section>
    </div>
  );
}
