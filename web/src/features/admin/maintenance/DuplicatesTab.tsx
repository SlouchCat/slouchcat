import type { DuplicateGroup } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { formatBytes } from '@/lib/format';
import { Notice, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import { AriaButton, DisclosureGroup, RadioGroup } from '@/ui/aria';
import { duplicatesQuery } from '../queries';
import { groupKey, titleWithYear, type Outcome } from './shared';
import styles from './Maintenance.module.css';

interface DuplicatesTabProps {
  outcome: Outcome | undefined;
  /** The title to keep per group (a draft of the page); the first one when none was picked. */
  targets: Readonly<Record<string, string>>;
  /** Open state of the groups; only the first one starts open (mockup Pfl-C). */
  expanded: Readonly<Record<string, boolean>>;
  hold: boolean;
  /** The key of the group whose merge runs. */
  merging: string | undefined;
  onTarget: (key: string, itemId: string) => void;
  onExpandedChange: (key: string, expanded: boolean) => void;
  onMerge: (group: DuplicateGroup, targetId: string) => void;
}

/** The duplicates (PFL-4): one section per group, not by library, since a group may span some. */
export function DuplicatesTab({
  outcome,
  targets,
  expanded,
  hold,
  merging,
  onTarget,
  onExpandedChange,
  onMerge,
}: DuplicatesTabProps) {
  const { t } = useTranslation();
  const duplicates = useQuery({ ...duplicatesQuery, refetchOnWindowFocus: !hold });

  return (
    <div className={styles.tab}>
      <VisuallyHidden as="h2">{t('admin.maintenance.tab.duplicates')}</VisuallyHidden>
      <Text tone="secondary">{t('admin.maintenance.hint.duplicates')}</Text>
      {outcome && <Notice tone={outcome.tone}>{outcome.text}</Notice>}
      {duplicates.isPending && <Skeleton shape="block" />}
      {duplicates.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, duplicates.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void duplicates.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {duplicates.data?.length === 0 && (
        <Text tone="secondary">{t('admin.maintenance.duplicates.empty')}</Text>
      )}
      {duplicates.data?.map((group, index) => {
        const key = groupKey(group);
        const picked = targets[key];
        return (
          <DuplicateSection
            key={key}
            group={group}
            index={index}
            target={
              group.items.some((item) => item.itemId === picked)
                ? (picked ?? '')
                : (group.items[0]?.itemId ?? '')
            }
            isExpanded={expanded[key] ?? index === 0}
            merging={merging === key}
            onTarget={(itemId) => onTarget(key, itemId)}
            onExpandedChange={(open) => onExpandedChange(key, open)}
            onMerge={onMerge}
          />
        );
      })}
    </div>
  );
}

function DuplicateSection({
  group,
  index,
  target,
  isExpanded,
  merging,
  onTarget,
  onExpandedChange,
  onMerge,
}: {
  group: DuplicateGroup;
  index: number;
  target: string;
  isExpanded: boolean;
  merging: boolean;
  onTarget: (itemId: string) => void;
  onExpandedChange: (expanded: boolean) => void;
  onMerge: (group: DuplicateGroup, targetId: string) => void;
}) {
  const { t, i18n } = useTranslation();
  const name = t('admin.maintenance.duplicates.group', { index: index + 1 });
  const first = group.items[0];
  return (
    <DisclosureGroup
      title={name}
      meta={
        <span className={styles.muted}>
          {t('admin.maintenance.duplicates.meta', {
            reason: t(`admin.maintenance.duplicates.reason.${group.reason}`),
            title: first ? titleWithYear(t, first.title, first.year) : '',
            count: group.items.length,
          })}
        </span>
      }
      isExpanded={isExpanded}
      onExpandedChange={onExpandedChange}
    >
      <ul className={styles.duplicates} aria-label={name}>
        {group.items.map((item) => (
          <li key={item.itemId} className={styles.duplicate}>
            <span className={styles.duplicateHead}>
              <TextLink href={paths.adminItem(item.itemId)}>
                {titleWithYear(t, item.title, item.year)}
              </TextLink>
              <span className={styles.muted}>{t(`kind.${item.kind}`)}</span>
              {item.itemId === target && (
                <span className={styles.keep}>{t('admin.maintenance.duplicates.keep')}</span>
              )}
            </span>
            {item.files.length === 0 ? (
              <span className={styles.muted}>{t('admin.maintenance.duplicates.noFiles')}</span>
            ) : (
              item.files.map((file) => (
                <span key={file.id} className={styles.fileLine}>
                  <code className={styles.path}>{file.path}</code>
                  <span className={styles.muted}>{formatBytes(file.sizeBytes, i18n.language)}</span>
                </span>
              ))
            )}
          </li>
        ))}
      </ul>
      <div className={styles.mergeRow}>
        <RadioGroup
          label={t('admin.maintenance.duplicates.target')}
          value={target}
          onChange={onTarget}
          options={group.items.map((item, position) => ({
            value: item.itemId,
            label: t('admin.maintenance.duplicates.option', {
              index: position + 1,
              title: item.title,
            }),
          }))}
        />
        <AriaButton
          variant="primary"
          isPending={merging}
          isDisabled={target === ''}
          onPress={() => onMerge(group, target)}
        >
          {t('admin.maintenance.duplicates.merge')}
        </AriaButton>
      </div>
    </DisclosureGroup>
  );
}
