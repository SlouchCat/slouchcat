import type { AdminItem } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Chip, FactList, Notice, Panel } from '@/ui';
import { AriaButton, DisclosureGroup, RadioGroup } from '@/ui/aria';
import { ratingOptions } from '../../shared/choices';
import { itemFacts, sourceFacts, sourceSummary } from './facts';
import styles from './AdminItem.module.css';

/** The choice of «Eigene Freigabe setzen» that hands the rating back to the metadata. */
export const FROM_METADATA = 'metadata';

/** The stored own rating as the choice holds it: the age, or «Aus Metadaten». */
export const ratingChoiceOf = (item: AdminItem): string =>
  item.ownRating !== undefined ? String(item.ownRating) : FROM_METADATA;

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
    </div>
  );
}
