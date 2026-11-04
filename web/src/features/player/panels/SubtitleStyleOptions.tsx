import { useId } from 'react';
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
        {/* The offset is read with the buttons: earlier, the value, later, back to 0. */}
        <div className={styles.offset} role="group" aria-labelledby={offsetId}>
          <AriaButton variant="secondary" onPress={() => step(-1)}>
            {t('player.subtitleStyle.offset.earlier')}
          </AriaButton>
          <output className={styles.value} aria-live="polite">
            {t('player.subtitleStyle.offset.value', { seconds })}
          </output>
          <AriaButton variant="secondary" onPress={() => step(1)}>
            {t('player.subtitleStyle.offset.later')}
          </AriaButton>
          <AriaButton
            variant="ghost"
            onPress={() => onChange({ offsetMs: 0 })}
            isDisabled={value.offsetMs === 0}
          >
            {t('player.subtitleStyle.offset.reset')}
          </AriaButton>
        </div>
      </section>
    </div>
  );
}
