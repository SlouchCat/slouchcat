import type { AdminItem } from '@slouchcat/api-client';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { LANGUAGE_CODES, NO_LANGUAGE } from '@/features/player/labels';
import { Chip, FactList, Notice, Panel, SelectField, Text } from '@/ui';
import { AriaButton, DisclosureGroup, RadioGroup } from '@/ui/aria';
import { ratingOptions } from '../../shared/choices';
import { itemFacts, providerName, sourceFacts, sourceSummary } from './facts';
import styles from './AdminItem.module.css';

/** The choice of «Eigene Freigabe setzen» that hands the rating back to the metadata. */
export const FROM_METADATA = 'metadata';

/** The stored own rating as the choice holds it: the age, or «Aus Metadaten». */
export const ratingChoiceOf = (item: AdminItem): string =>
  item.ownRating !== undefined ? String(item.ownRating) : FROM_METADATA;

/** The lock name of the original language (concept 13 11.7). */
const LANGUAGE_LOCK = 'original_language';

/**
 * The stored original language as the choice holds it: the locked code (or "" for «Unbekannt»),
 * else «Aus Metadaten».
 */
export const languageChoiceOf = (item: AdminItem): string =>
  item.lockedFields.includes(LANGUAGE_LOCK) ? (item.originalLanguage ?? '') : FROM_METADATA;

/** The original language can be set for films and series only. */
export const hasOriginalLanguage = (item: AdminItem): boolean =>
  item.kind === 'movie' || item.kind === 'series';

/** Withdrawn ISO 639-1 codes the server refuses (trackpick.Valid); iw is in LANGUAGE_CODES. */
const WITHDRAWN = new Set(['in', 'ji', 'jw', 'mo', 'sh']);

/**
 * A code the server takes as an original language: two letters, a language (not TMDB's xx), one
 * the shared language table keeps as it is (not cn, nb, nn) and not withdrawn.
 */
const isLanguageChoice = (code: string) =>
  /^[a-z]{2}$/.test(code) &&
  !NO_LANGUAGE.has(code) &&
  (LANGUAGE_CODES[code] ?? code) === code &&
  !WITHDRAWN.has(code);

/** The original language choice of the tab, held by the page like the rating. */
export interface LanguageChoice {
  value: string;
  dirty: boolean;
  /** Every language code the server knows (metadata settings), for the select. */
  known: readonly string[];
  onChange: (value: string) => void;
  onSave: () => void;
  saving: boolean;
  error: unknown;
}

interface MetadataTabProps {
  item: AdminItem;
  /** The open disclosures («sources», «rating»), held by the page so a tab switch keeps them. */
  isExpanded: (key: string) => boolean;
  onExpandedChange: (key: string, expanded: boolean) => void;
  /** The rating choice: the page's draft (S-17), else the stored value. */
  rating: string;
  ratingDirty: boolean;
  onRatingChange: (value: string) => void;
  onRatingSave: () => void;
  ratingSaving: boolean;
  ratingError: unknown;
  /** «Originalsprache» for films and series; undefined hides it. */
  language?: LanguageChoice;
}

/**
 * The tab «Metadaten» (TIT-2 to TIT-4): the facts of the title, then «Herkunft je Feld» and
 * «Freigabe» as disclosures. The rating choice is a draft of the page, so folding «Freigabe» or
 * switching the tab keeps it, and leaving the page with it asks first (S-13).
 */
export function MetadataTab({
  item,
  isExpanded,
  onExpandedChange,
  rating,
  ratingDirty,
  onRatingChange,
  onRatingSave,
  ratingSaving,
  ratingError,
  language,
}: MetadataTabProps) {
  const { t, i18n } = useTranslation();
  const none = t('admin.item.facts.none');
  const fsk = (age: number | undefined) => (age !== undefined ? t('rating.fsk', { age }) : none);
  const number = new Intl.NumberFormat(i18n.language);
  const sources = item.fieldSources.length;
  return (
    <div className={styles.tabBody}>
      <Panel as="section" aria-labelledby="admin-item-facts">
        <h2 id="admin-item-facts" className={styles.sectionTitle}>
          {t('admin.item.facts.metadata')}
        </h2>
        <FactList facts={itemFacts(t, item, i18n.language)} />
      </Panel>
      {sources > 0 && (
        <DisclosureGroup
          level={2}
          title={t('admin.item.sources.title')}
          count={number.format(sources)}
          countLabel={t('admin.item.sources.count', { count: sources })}
          meta={sourceSummary(t, item.fieldSources)}
          isExpanded={isExpanded('sources')}
          onExpandedChange={(open) => onExpandedChange('sources', open)}
        >
          <FactList facts={sourceFacts(t, item.fieldSources)} />
        </DisclosureGroup>
      )}
      <DisclosureGroup
        level={2}
        title={t('admin.item.rating.title')}
        meta={
          <>
            <span>
              {t('admin.item.rating.summary', {
                effective: fsk(item.effectiveRating),
                own: fsk(item.ownRating),
                source: item.ratingSource || none,
              })}
            </span>
            {ratingDirty && <Chip tone="warning">{t('admin.item.rating.unsaved')}</Chip>}
          </>
        }
        isExpanded={isExpanded('rating')}
        onExpandedChange={(open) => onExpandedChange('rating', open)}
      >
        <div className={styles.stack}>
          <FactList
            facts={[
              [t('admin.item.rating.own'), fsk(item.ownRating)],
              [t('admin.item.rating.effective'), fsk(item.effectiveRating)],
              [t('admin.item.rating.source'), item.ratingSource || none],
            ]}
          />
          <div className={styles.inline}>
            <RadioGroup
              label={t('admin.item.rating.label')}
              value={rating}
              onChange={onRatingChange}
              options={[
                { value: FROM_METADATA, label: t('admin.item.rating.fromMetadata') },
                ...ratingOptions(t),
              ]}
            />
            <AriaButton variant="primary" isPending={ratingSaving} onPress={onRatingSave}>
              {ratingSaving ? t('common.saving') : t('common.save')}
            </AriaButton>
          </div>
          {ratingError !== null && <Notice tone="danger">{problemMessage(t, ratingError)}</Notice>}
        </div>
      </DisclosureGroup>
      {language && (
        <OriginalLanguage
          item={item}
          choice={language}
          isExpanded={isExpanded('language')}
          onExpandedChange={(open) => onExpandedChange('language', open)}
        />
      )}
    </div>
  );
}

/**
 * «Originalsprache» (concept 13 11.7): the stored language with its source, then the choice
 * «Aus Metadaten», «Unbekannt» or a language, saved like the rating. A chosen value is locked;
 * «Aus Metadaten» lifts the lock and lets the next match look the language up again.
 */
function OriginalLanguage({
  item,
  choice,
  isExpanded,
  onExpandedChange,
}: {
  item: AdminItem;
  choice: LanguageChoice;
  isExpanded: boolean;
  onExpandedChange: (open: boolean) => void;
}) {
  const { t, i18n } = useTranslation();
  const names = useMemo(
    () => new Intl.DisplayNames([i18n.language], { type: 'language' }),
    [i18n.language],
  );
  const nameOf = (code: string) => names.of(code) ?? code;
  const current =
    item.originalLanguage === undefined
      ? t('admin.item.language.notLooked')
      : item.originalLanguage === ''
        ? t('admin.item.language.unknown')
        : nameOf(item.originalLanguage);
  const locked = item.lockedFields.includes(LANGUAGE_LOCK);
  const won = item.fieldSources.find((source) => source.field === LANGUAGE_LOCK)?.provider;
  const source = locked
    ? t('admin.item.language.manual')
    : won
      ? providerName(t, won)
      : t('admin.item.facts.none');
  const options = useMemo(() => {
    const codes = new Set(choice.known.filter(isLanguageChoice));
    if (choice.value !== FROM_METADATA && choice.value !== '') codes.add(choice.value);
    const languages = [...codes]
      .map((code) => ({ value: code, label: names.of(code) ?? code }))
      .sort((a, b) => a.label.localeCompare(b.label, i18n.language));
    return [
      { value: FROM_METADATA, label: t('admin.item.language.fromMetadata') },
      { value: '', label: t('admin.item.language.unknown') },
      ...languages,
    ];
  }, [choice.known, choice.value, names, i18n.language, t]);
  return (
    <DisclosureGroup
      level={2}
      title={t('admin.item.language.title')}
      meta={
        <>
          <span>{t('admin.item.language.summary', { language: current, source })}</span>
          {choice.dirty && <Chip tone="warning">{t('admin.item.language.unsaved')}</Chip>}
        </>
      }
      isExpanded={isExpanded}
      onExpandedChange={onExpandedChange}
    >
      <div className={styles.stack}>
        <FactList
          facts={[
            [t('admin.item.language.current'), current],
            [t('admin.item.language.source'), source],
          ]}
        />
        <div className={styles.inline}>
          <SelectField
            label={t('admin.item.language.label')}
            value={choice.value}
            options={options}
            onChange={choice.onChange}
          />
          <AriaButton variant="primary" isPending={choice.saving} onPress={choice.onSave}>
            {choice.saving ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
        <Text tone="secondary" variant="caption">
          {t('admin.item.language.hint')}
        </Text>
        {choice.error !== null && <Notice tone="danger">{problemMessage(t, choice.error)}</Notice>}
      </div>
    </DisclosureGroup>
  );
}
